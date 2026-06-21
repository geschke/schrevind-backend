package grrt

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/users"
)

func TestCanDoAnyWithContextAndScopeForActionWithContext(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "grrt.sqlite")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	adminID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:     "admin@example.com",
		Password:  "Secret123!",
		FirstName: "Ada",
		LastName:  "Lovelace",
	})
	if err != nil {
		t.Fatalf("create admin user: %v", err)
	}

	userID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:     "viewer@example.com",
		Password:  "Secret123!",
		FirstName: "Grace",
		LastName:  "Hopper",
	})
	if err != nil {
		t.Fatalf("create regular user: %v", err)
	}

	depot := db.Depot{
		Name:         "Main Depot",
		BaseCurrency: "EUR",
		Status:       "active",
	}
	if err := database.CreateDepot(&depot); err != nil {
		t.Fatalf("create depot: %v", err)
	}

	if err := database.GrantMembership(&db.Membership{
		EntityType: db.EntityTypeDepot,
		EntityID:   depot.ID,
		UserID:     userID,
		Role:       db.RoleDepotViewer,
	}); err != nil {
		t.Fatalf("grant depot membership: %v", err)
	}

	group := db.Group{Name: "Viewer Context Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("create context group: %v", err)
	}
	if _, err := database.AddGroupMember(group.ID, adminID, db.RoleGroupAdmin); err != nil {
		t.Fatalf("add admin to context group: %v", err)
	}
	if _, err := database.AddGroupMember(group.ID, userID, db.RoleGroupMember); err != nil {
		t.Fatalf("add user to context group: %v", err)
	}

	g := New(database)

	allowed, err := g.CanDoAnyWithContext(userID, group.ID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("CanDoAnyWithContext entries:list: %v", err)
	}
	if !allowed {
		t.Fatalf("expected viewer to have entries:list on at least one depot")
	}

	allowed, err = g.CanDoAnyWithContext(userID, group.ID, db.EntityTypeDepot, "depot:access:add")
	if err != nil {
		t.Fatalf("CanDoAnyWithContext depot:access:add: %v", err)
	}
	if allowed {
		t.Fatalf("expected viewer to not have depot:access:add on any depot")
	}

	scope, err := g.ScopeForActionWithContext(adminID, db.SystemGroupID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("ScopeForActionWithContext system admin: %v", err)
	}
	if !scope.All {
		t.Fatalf("expected system admin scope to allow all depot entities")
	}

	scope, err = g.ScopeForActionWithContext(userID, group.ID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("ScopeForActionWithContext regular user: %v", err)
	}
	if scope.All {
		t.Fatalf("expected regular user scope to be role-based")
	}
	if len(scope.Roles) != 3 {
		t.Fatalf("expected three allowed roles for entries:list, got %d", len(scope.Roles))
	}
}

func TestContextSudoOnlyAppliesInSystemContext(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "grrt-context.sqlite")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	systemAdminID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:     "system-admin@example.com",
		Password:  "Secret123!",
		FirstName: "System",
		LastName:  "Admin",
	})
	if err != nil {
		t.Fatalf("create system admin: %v", err)
	}

	groupAdminID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:     "group-admin@example.com",
		Password:  "Secret123!",
		FirstName: "Group",
		LastName:  "Admin",
	})
	if err != nil {
		t.Fatalf("create group admin: %v", err)
	}

	group := db.Group{Name: "Context Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := database.AddGroupMember(group.ID, groupAdminID, db.RoleGroupAdmin); err != nil {
		t.Fatalf("add group admin: %v", err)
	}
	if _, err := database.AddGroupMember(group.ID, systemAdminID, db.RoleGroupMember); err != nil {
		t.Fatalf("add system admin as group member: %v", err)
	}

	g := New(database)

	allowed, err := g.CanDoWithContext(systemAdminID, db.SystemGroupID, db.EntityTypeGroup, "currency:delete", group.ID)
	if err != nil {
		t.Fatalf("CanDoWithContext sudo system context: %v", err)
	}
	if !allowed {
		t.Fatalf("system admin in system context should be allowed")
	}

	allowed, err = g.CanDoWithContext(systemAdminID, group.ID, db.EntityTypeGroup, "currency:delete", group.ID)
	if err != nil {
		t.Fatalf("CanDoWithContext normal context: %v", err)
	}
	if allowed {
		t.Fatalf("system admin in normal group context must not get sudo rights")
	}

	allowed, err = g.CanDoWithContext(groupAdminID, group.ID, db.EntityTypeGroup, "currency:delete", group.ID)
	if err != nil {
		t.Fatalf("CanDoWithContext group admin: %v", err)
	}
	if !allowed {
		t.Fatalf("group admin in group context should be allowed")
	}
}

func TestContextRequiredForCanDoAnyAndScope(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "grrt-context-scope.sqlite")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	systemAdminID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:     "scope-system-admin@example.com",
		Password:  "Secret123!",
		FirstName: "Scope",
		LastName:  "Admin",
	})
	if err != nil {
		t.Fatalf("create system admin: %v", err)
	}

	userID, err := users.Create(context.Background(), database, users.CreateParams{
		Email:     "scope-user@example.com",
		Password:  "Secret123!",
		FirstName: "Scope",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	group := db.Group{Name: "Scope Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("create group: %v", err)
	}
	if _, err := database.AddGroupMember(group.ID, systemAdminID, db.RoleGroupAdmin); err != nil {
		t.Fatalf("add group admin: %v", err)
	}
	if _, err := database.AddGroupMember(group.ID, userID, db.RoleGroupMember); err != nil {
		t.Fatalf("add group member: %v", err)
	}

	depot := db.Depot{Name: "Scope Depot", BaseCurrency: "EUR", Status: "active"}
	if err := database.CreateDepot(&depot); err != nil {
		t.Fatalf("create depot: %v", err)
	}
	if err := database.GrantMembership(&db.Membership{
		EntityType: db.EntityTypeDepot,
		EntityID:   depot.ID,
		UserID:     userID,
		Role:       db.RoleDepotViewer,
	}); err != nil {
		t.Fatalf("grant depot viewer: %v", err)
	}

	g := New(database)

	allowed, err := g.CanDoAnyWithContext(userID, group.ID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("CanDoAnyWithContext group context: %v", err)
	}
	if !allowed {
		t.Fatalf("group member with depot viewer role should have entries:list")
	}

	allowed, err = g.CanDoAnyWithContext(userID, db.SystemGroupID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("CanDoAnyWithContext system context without sudo: %v", err)
	}
	if allowed {
		t.Fatalf("non-system-admin must not act in system context")
	}

	scope, err := g.ScopeForActionWithContext(systemAdminID, db.SystemGroupID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("ScopeForActionWithContext sudo: %v", err)
	}
	if !scope.All {
		t.Fatalf("system admin in system context should get all scope")
	}

	scope, err = g.ScopeForActionWithContext(userID, group.ID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("ScopeForActionWithContext group member: %v", err)
	}
	if scope.All {
		t.Fatalf("regular group context must not get all scope")
	}
	if !slices.Contains(scope.Roles, db.RoleDepotViewer) {
		t.Fatalf("scope roles = %v, want depot viewer role", scope.Roles)
	}
}

func TestGroupMemberPermissionsAreConfigured(t *testing.T) {
	if !db.IsValidGroupRole(db.RoleGroupMember) {
		t.Fatalf("RoleGroupMember = %q is not a valid group role", db.RoleGroupMember)
	}

	for _, action := range []string{
		"currency:list",
		"currency:add",
		"currency:edit",
		"security:list",
		"security:add",
		"security:edit",
		"withholding-tax-default:list",
		"withholding-tax-default:add",
		"withholding-tax-default:edit",
		"group:list:accessible",
		"member:list",
	} {
		roles, err := AllowedRoles(db.EntityTypeGroup, action)
		if err != nil {
			t.Fatalf("AllowedRoles(group, %q) error = %v", action, err)
		}
		if !slices.Contains(roles, db.RoleGroupAdmin) {
			t.Fatalf("AllowedRoles(group, %q) = %v, want admin", action, roles)
		}
		if !slices.Contains(roles, db.RoleGroupMember) {
			t.Fatalf("AllowedRoles(group, %q) = %v, want member", action, roles)
		}
	}

	for _, action := range []string{
		"currency:delete",
		"security:delete",
		"withholding-tax-default:delete",
	} {
		roles, err := AllowedRoles(db.EntityTypeGroup, action)
		if err != nil {
			t.Fatalf("AllowedRoles(group, %q) error = %v", action, err)
		}
		if slices.Contains(roles, db.RoleGroupMember) {
			t.Fatalf("AllowedRoles(group, %q) = %v, member must not be allowed", action, roles)
		}
		if !slices.Contains(roles, db.RoleGroupAdmin) {
			t.Fatalf("AllowedRoles(group, %q) = %v, want admin", action, roles)
		}
	}

	if _, err := AllowedRoles(db.EntityTypeGroup, "member:add"); err == nil {
		t.Fatalf("AllowedRoles(group, member:add) error = nil, want unknown action")
	}
	if _, err := AllowedRoles(db.EntityTypeGroup, "user:list"); err == nil {
		t.Fatalf("AllowedRoles(group, user:list) error = nil, want unknown action")
	}

	roles, err := AllowedRoles(db.EntityTypeSystem, "member:add")
	if err != nil {
		t.Fatalf("AllowedRoles(system, member:add) error = %v", err)
	}
	if !slices.Contains(roles, db.RoleSystemAdmin) {
		t.Fatalf("AllowedRoles(system, member:add) = %v, want system admin", roles)
	}
}
