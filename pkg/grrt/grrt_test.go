package grrt

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/users"
)

func TestCanDoAnyAndScopeForAction(t *testing.T) {
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

	g := New(database)

	allowed, err := g.CanDoAny(userID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("CanDoAny entries:list: %v", err)
	}
	if !allowed {
		t.Fatalf("expected viewer to have entries:list on at least one depot")
	}

	allowed, err = g.CanDoAny(userID, db.EntityTypeDepot, "depot:access:add")
	if err != nil {
		t.Fatalf("CanDoAny depot:access:add: %v", err)
	}
	if allowed {
		t.Fatalf("expected viewer to not have depot:access:add on any depot")
	}

	scope, err := g.ScopeForAction(adminID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("ScopeForAction system admin: %v", err)
	}
	if !scope.All {
		t.Fatalf("expected system admin scope to allow all depot entities")
	}

	scope, err = g.ScopeForAction(userID, db.EntityTypeDepot, "entries:list")
	if err != nil {
		t.Fatalf("ScopeForAction regular user: %v", err)
	}
	if scope.All {
		t.Fatalf("expected regular user scope to be role-based")
	}
	if len(scope.Roles) != 3 {
		t.Fatalf("expected three allowed roles for entries:list, got %d", len(scope.Roles))
	}
}
