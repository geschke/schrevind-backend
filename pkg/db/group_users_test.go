package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func newGroupUsersTestDB(t *testing.T) *DB {
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
	database := newGroupUsersTestDB(t)
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

	if _, err := database.AddUserToGroup(group.ID, adminID); err != nil {
		t.Fatalf("AddUserToGroup(admin) error = %v", err)
	}
	if _, err := database.AddUserToGroup(group.ID, memberID); err != nil {
		t.Fatalf("AddUserToGroup(member) error = %v", err)
	}
	if err := database.GrantMembership(&Membership{
		EntityType: EntityTypeGroup,
		EntityID:   group.ID,
		UserID:     adminID,
		Role:       RoleGroupAdmin,
	}); err != nil {
		t.Fatalf("GrantMembership(admin) error = %v", err)
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
	if roleByUserID[memberID] != "" {
		t.Fatalf("member role = %q, want empty string", roleByUserID[memberID])
	}
}

func TestAddGroupMemberSetsAndClearsGroupRole(t *testing.T) {
	database := newGroupUsersTestDB(t)
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

	added, err = database.AddGroupMember(group.ID, memberID, "")
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

	_, found, err = database.GetMembership(EntityTypeGroup, group.ID, memberID)
	if err != nil {
		t.Fatalf("GetMembership(member) error = %v", err)
	}
	if found {
		t.Fatalf("member membership found = true, want false")
	}

	added, err = database.AddGroupMember(group.ID, memberID, RoleGroupAdmin)
	if err != nil {
		t.Fatalf("AddGroupMember(promote member) error = %v", err)
	}
	if added {
		t.Fatalf("AddGroupMember(promote member) added = true, want false for existing group member")
	}

	added, err = database.AddGroupMember(group.ID, memberID, "")
	if err != nil {
		t.Fatalf("AddGroupMember(clear member role) error = %v", err)
	}
	if added {
		t.Fatalf("AddGroupMember(clear member role) added = true, want false for existing group member")
	}

	_, found, err = database.GetMembership(EntityTypeGroup, group.ID, memberID)
	if err != nil {
		t.Fatalf("GetMembership(member after clear) error = %v", err)
	}
	if found {
		t.Fatalf("member membership after clear found = true, want false")
	}
}

func TestRemoveGroupMemberClearsRoleAndBlocksLastAdmin(t *testing.T) {
	database := newGroupUsersTestDB(t)
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
	database := newGroupUsersTestDB(t)
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

	added, err := database.AddGroupMember(group.ID, adminID, "")
	if !errors.Is(err, ErrLastGroupAdmin) {
		t.Fatalf("AddGroupMember(clear last admin) error = %v, want ErrLastGroupAdmin", err)
	}
	if added {
		t.Fatalf("AddGroupMember(clear last admin) added = true, want false")
	}

	membership, found, err := database.GetMembership(EntityTypeGroup, group.ID, adminID)
	if err != nil {
		t.Fatalf("GetMembership(admin) error = %v", err)
	}
	if !found || membership.Role != RoleGroupAdmin {
		t.Fatalf("admin membership = %+v found=%v, want role %q", membership, found, RoleGroupAdmin)
	}
}
