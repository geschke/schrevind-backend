package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func newGroupMembersTestDB(t *testing.T) *DB {
	t.Helper()

	database, err := Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	return database
}

func TestListGroupMembersByGroupIDIncludesExplicitRole(t *testing.T) {
	database := newGroupMembersTestDB(t)
	ctx := context.Background()

	adminID, err := database.CreateUser(ctx, User{
		Email:     "admin@example.com",
		Password:  "secret",
		FirstName: "Group",
		LastName:  "Admin",
		Locale:    "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(admin) error = %v", err)
	}

	memberID, err := database.CreateUser(ctx, User{
		Email:     "member@example.com",
		Password:  "secret",
		FirstName: "Group",
		LastName:  "Member",
		Locale:    "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(member) error = %v", err)
	}

	group := Group{Name: "Example Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}

	if _, err := database.AddGroupMember(group.ID, adminID, RoleGroupAdmin); err != nil {
		t.Fatalf("AddGroupMember(admin) error = %v", err)
	}
	if _, err := database.AddGroupMember(group.ID, memberID, RoleGroupMember); err != nil {
		t.Fatalf("AddGroupMember(member) error = %v", err)
	}

	members, err := database.ListGroupMembersByGroupID(group.ID)
	if err != nil {
		t.Fatalf("ListGroupMembersByGroupID() error = %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("len(members) = %d, want 2", len(members))
	}

	roleByUserID := make(map[int64]string, len(members))
	for _, member := range members {
		roleByUserID[member.ID] = member.Role
	}

	if roleByUserID[adminID] != RoleGroupAdmin {
		t.Fatalf("admin role = %q, want %q", roleByUserID[adminID], RoleGroupAdmin)
	}
	if roleByUserID[memberID] != RoleGroupMember {
		t.Fatalf("member role = %q, want %q", roleByUserID[memberID], RoleGroupMember)
	}
}

func TestAddGroupMemberSetsAndChangesGroupRole(t *testing.T) {
	database := newGroupMembersTestDB(t)
	ctx := context.Background()

	adminID, err := database.CreateUser(ctx, User{
		Email:    "admin-role@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(admin) error = %v", err)
	}
	memberID, err := database.CreateUser(ctx, User{
		Email:    "member-role@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(member) error = %v", err)
	}

	group := Group{Name: "Role Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}

	added, err := database.AddGroupMember(group.ID, adminID, RoleGroupAdmin)
	if err != nil {
		t.Fatalf("AddGroupMember(admin) error = %v", err)
	}
	if !added {
		t.Fatalf("AddGroupMember(admin) added = false, want true")
	}

	added, err = database.AddGroupMember(group.ID, memberID, RoleGroupMember)
	if err != nil {
		t.Fatalf("AddGroupMember(member) error = %v", err)
	}
	if !added {
		t.Fatalf("AddGroupMember(member) added = false, want true")
	}

	membership, found, err := database.GetMembership(EntityTypeGroup, group.ID, adminID)
	if err != nil {
		t.Fatalf("GetMembership(admin) error = %v", err)
	}
	if !found || membership.Role != RoleGroupAdmin {
		t.Fatalf("admin membership = %+v found=%v, want role %q", membership, found, RoleGroupAdmin)
	}

	membership, found, err = database.GetMembership(EntityTypeGroup, group.ID, memberID)
	if err != nil {
		t.Fatalf("GetMembership(member) error = %v", err)
	}
	if !found || membership.Role != RoleGroupMember {
		t.Fatalf("member membership = %+v found=%v, want role %q", membership, found, RoleGroupMember)
	}

	added, err = database.AddGroupMember(group.ID, memberID, RoleGroupAdmin)
	if err != nil {
		t.Fatalf("AddGroupMember(promote member) error = %v", err)
	}
	if added {
		t.Fatalf("AddGroupMember(promote member) added = true, want false for existing group member")
	}

	added, err = database.AddGroupMember(group.ID, memberID, RoleGroupMember)
	if err != nil {
		t.Fatalf("AddGroupMember(demote member) error = %v", err)
	}
	if added {
		t.Fatalf("AddGroupMember(demote member) added = true, want false for existing group member")
	}

	membership, found, err = database.GetMembership(EntityTypeGroup, group.ID, memberID)
	if err != nil {
		t.Fatalf("GetMembership(member after demote) error = %v", err)
	}
	if !found || membership.Role != RoleGroupMember {
		t.Fatalf("member membership after demote = %+v found=%v, want role %q", membership, found, RoleGroupMember)
	}
}

func TestRemoveGroupMemberClearsRoleAndBlocksLastAdmin(t *testing.T) {
	database := newGroupMembersTestDB(t)
	ctx := context.Background()

	adminID, err := database.CreateUser(ctx, User{
		Email:    "last-admin@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(admin) error = %v", err)
	}
	otherAdminID, err := database.CreateUser(ctx, User{
		Email:    "other-admin@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(other admin) error = %v", err)
	}

	group := Group{Name: "Admin Guard Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}

	if _, err := database.AddGroupMember(group.ID, adminID, RoleGroupAdmin); err != nil {
		t.Fatalf("AddGroupMember(admin) error = %v", err)
	}
	if _, err := database.AddGroupMember(group.ID, otherAdminID, RoleGroupAdmin); err != nil {
		t.Fatalf("AddGroupMember(other admin) error = %v", err)
	}

	removed, err := database.RemoveGroupMember(group.ID, otherAdminID)
	if err != nil {
		t.Fatalf("RemoveGroupMember(other admin) error = %v", err)
	}
	if !removed {
		t.Fatalf("RemoveGroupMember(other admin) removed = false, want true")
	}

	_, found, err := database.GetMembership(EntityTypeGroup, group.ID, otherAdminID)
	if err != nil {
		t.Fatalf("GetMembership(other admin) error = %v", err)
	}
	if found {
		t.Fatalf("other admin membership found = true, want false")
	}

	removed, err = database.RemoveGroupMember(group.ID, adminID)
	if !errors.Is(err, ErrLastGroupAdmin) {
		t.Fatalf("RemoveGroupMember(last admin) error = %v, want ErrLastGroupAdmin", err)
	}
	if removed {
		t.Fatalf("RemoveGroupMember(last admin) removed = true, want false")
	}
}

func TestAddGroupMemberBlocksClearingLastAdminRole(t *testing.T) {
	database := newGroupMembersTestDB(t)
	ctx := context.Background()

	adminID, err := database.CreateUser(ctx, User{
		Email:    "clear-last-admin@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(admin) error = %v", err)
	}

	group := Group{Name: "Clear Last Admin Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}

	if _, err := database.AddGroupMember(group.ID, adminID, RoleGroupAdmin); err != nil {
		t.Fatalf("AddGroupMember(admin) error = %v", err)
	}

	added, err := database.AddGroupMember(group.ID, adminID, RoleGroupMember)
	if !errors.Is(err, ErrLastGroupAdmin) {
		t.Fatalf("AddGroupMember(demote last admin) error = %v, want ErrLastGroupAdmin", err)
	}
	if added {
		t.Fatalf("AddGroupMember(demote last admin) added = true, want false")
	}

	membership, found, err := database.GetMembership(EntityTypeGroup, group.ID, adminID)
	if err != nil {
		t.Fatalf("GetMembership(admin) error = %v", err)
	}
	if !found || membership.Role != RoleGroupAdmin {
		t.Fatalf("admin membership = %+v found=%v, want role %q", membership, found, RoleGroupAdmin)
	}
}

func TestAddGroupMemberRequiresExplicitRole(t *testing.T) {
	database := newGroupMembersTestDB(t)
	ctx := context.Background()

	userID, err := database.CreateUser(ctx, User{
		Email:    "missing-role@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	group := Group{Name: "Missing Role Group"}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}

	added, err := database.AddGroupMember(group.ID, userID, "")
	if err == nil {
		t.Fatalf("AddGroupMember(empty role) error = nil, want error")
	}
	if added {
		t.Fatalf("AddGroupMember(empty role) added = true, want false")
	}
}

func TestCountGroupMembershipsByUserID(t *testing.T) {
	database := newGroupMembersTestDB(t)
	ctx := context.Background()

	userID, err := database.CreateUser(ctx, User{
		Email:    "multi-group@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	adminID, err := database.CreateUser(ctx, User{
		Email:    "multi-group-admin@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(admin) error = %v", err)
	}

	groupA := Group{Name: "Group A"}
	if err := database.CreateGroup(&groupA); err != nil {
		t.Fatalf("CreateGroup(A) error = %v", err)
	}
	groupB := Group{Name: "Group B"}
	if err := database.CreateGroup(&groupB); err != nil {
		t.Fatalf("CreateGroup(B) error = %v", err)
	}

	count, err := database.CountGroupMembershipsByUserID(userID)
	if err != nil {
		t.Fatalf("CountGroupMembershipsByUserID(empty) error = %v", err)
	}
	if count != 0 {
		t.Fatalf("group membership count = %d, want 0", count)
	}

	if _, err := database.AddGroupMember(groupA.ID, adminID, RoleGroupAdmin); err != nil {
		t.Fatalf("AddGroupMember(admin A) error = %v", err)
	}
	if _, err := database.AddGroupMember(groupB.ID, adminID, RoleGroupAdmin); err != nil {
		t.Fatalf("AddGroupMember(admin B) error = %v", err)
	}
	if _, err := database.AddGroupMember(groupA.ID, userID, RoleGroupMember); err != nil {
		t.Fatalf("AddGroupMember(A) error = %v", err)
	}
	if _, err := database.AddGroupMember(groupB.ID, userID, RoleGroupMember); err != nil {
		t.Fatalf("AddGroupMember(B) error = %v", err)
	}

	count, err = database.CountGroupMembershipsByUserID(userID)
	if err != nil {
		t.Fatalf("CountGroupMembershipsByUserID(two groups) error = %v", err)
	}
	if count != 2 {
		t.Fatalf("group membership count = %d, want 2", count)
	}
}

func TestCountActiveDepotsWhereUserIsSoleOwner(t *testing.T) {
	database := newGroupMembersTestDB(t)
	ctx := context.Background()

	ownerID, err := database.CreateUser(ctx, User{
		Email:    "sole-owner@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(owner) error = %v", err)
	}
	otherOwnerID, err := database.CreateUser(ctx, User{
		Email:    "other-owner@example.com",
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser(other owner) error = %v", err)
	}

	activeSole := Depot{Name: "Active Sole Owner", BaseCurrency: "EUR", Status: "active"}
	if err := database.CreateDepot(&activeSole); err != nil {
		t.Fatalf("CreateDepot(active sole) error = %v", err)
	}
	activeShared := Depot{Name: "Active Shared Owner", BaseCurrency: "EUR", Status: "active"}
	if err := database.CreateDepot(&activeShared); err != nil {
		t.Fatalf("CreateDepot(active shared) error = %v", err)
	}
	inactiveSole := Depot{Name: "Inactive Sole Owner", BaseCurrency: "EUR", Status: "inactive"}
	if err := database.CreateDepot(&inactiveSole); err != nil {
		t.Fatalf("CreateDepot(inactive sole) error = %v", err)
	}

	for _, depotID := range []int64{activeSole.ID, activeShared.ID, inactiveSole.ID} {
		if err := database.GrantMembership(&Membership{
			EntityType: EntityTypeDepot,
			EntityID:   depotID,
			UserID:     ownerID,
			Role:       RoleDepotOwner,
		}); err != nil {
			t.Fatalf("GrantMembership(owner, depot %d) error = %v", depotID, err)
		}
	}
	if err := database.GrantMembership(&Membership{
		EntityType: EntityTypeDepot,
		EntityID:   activeShared.ID,
		UserID:     otherOwnerID,
		Role:       RoleDepotOwner,
	}); err != nil {
		t.Fatalf("GrantMembership(other owner) error = %v", err)
	}

	count, err := database.CountActiveDepotsWhereUserIsSoleOwner(ownerID)
	if err != nil {
		t.Fatalf("CountActiveDepotsWhereUserIsSoleOwner(owner) error = %v", err)
	}
	if count != 1 {
		t.Fatalf("active sole-owner depot count = %d, want 1", count)
	}

	count, err = database.CountActiveDepotsWhereUserIsSoleOwner(otherOwnerID)
	if err != nil {
		t.Fatalf("CountActiveDepotsWhereUserIsSoleOwner(other owner) error = %v", err)
	}
	if count != 0 {
		t.Fatalf("other owner active sole-owner depot count = %d, want 0", count)
	}
}
