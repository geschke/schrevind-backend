package users

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geschke/schrevind/pkg/db"
)

func TestCreateFirstUserGrantsSystemAdminWithoutGroupMembership(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if err := database.Migrate(); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	userID, err := Create(context.Background(), database, CreateParams{
		Email:     "first@example.com",
		Password:  "Secret123!",
		FirstName: "First",
		LastName:  "User",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	systemMembership, found, err := database.GetMembership(db.EntityTypeSystem, db.SystemGroupID, userID)
	if err != nil {
		t.Fatalf("GetMembership(system) error = %v", err)
	}
	if !found || systemMembership.Role != db.RoleSystemAdmin {
		t.Fatalf("system membership = %+v found=%v, want system admin", systemMembership, found)
	}

	groupMembership, found, err := database.GetMembership(db.EntityTypeGroup, db.SystemGroupID, userID)
	if err != nil {
		t.Fatalf("GetMembership(group system) error = %v", err)
	}
	if found {
		t.Fatalf("group system membership = %+v found=true, want no duplicate group membership", groupMembership)
	}

	groups, err := database.ListGroupsWithRoleByUserID(userID)
	if err != nil {
		t.Fatalf("ListGroupsWithRoleByUserID() error = %v", err)
	}
	if len(groups) != 1 || groups[0].ID != db.SystemGroupID || groups[0].Role != db.RoleSystemAdmin {
		t.Fatalf("groups = %+v, want virtual system group with admin role", groups)
	}
}
