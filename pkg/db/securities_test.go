package db

import (
	"path/filepath"
	"testing"
)

func newSecuritiesTestDB(t *testing.T) *DB {
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

func createSecurityTestGroup(t *testing.T, database *DB, name string) Group {
	t.Helper()

	group := Group{Name: name}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("CreateGroup(%q) error = %v", name, err)
	}
	return group
}

func TestCreateSecurityAllowsSameISINInDifferentGroups(t *testing.T) {
	database := newSecuritiesTestDB(t)
	groupA := createSecurityTestGroup(t, database, "Securities Group A")
	groupB := createSecurityTestGroup(t, database, "Securities Group B")

	first := Security{
		GroupID: groupA.ID,
		Name:    "Example Security A",
		ISIN:    "US0000000001",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&first); err != nil {
		t.Fatalf("CreateSecurity(group A) error = %v", err)
	}

	second := Security{
		GroupID: groupB.ID,
		Name:    "Example Security B",
		ISIN:    "US0000000001",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&second); err != nil {
		t.Fatalf("CreateSecurity(group B same ISIN) error = %v", err)
	}
}

func TestCreateSecurityRejectsDuplicateISINWithinGroup(t *testing.T) {
	database := newSecuritiesTestDB(t)
	group := createSecurityTestGroup(t, database, "Duplicate ISIN Group")

	first := Security{
		GroupID: group.ID,
		Name:    "First Security",
		ISIN:    "US0000000002",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&first); err != nil {
		t.Fatalf("CreateSecurity(first) error = %v", err)
	}

	second := Security{
		GroupID: group.ID,
		Name:    "Second Security",
		ISIN:    "US0000000002",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&second); err == nil {
		t.Fatalf("CreateSecurity(duplicate ISIN) error = nil, want error")
	}
}

func TestListSecuritiesByGroupIDScopesResults(t *testing.T) {
	database := newSecuritiesTestDB(t)
	groupA := createSecurityTestGroup(t, database, "Scoped Securities Group A")
	groupB := createSecurityTestGroup(t, database, "Scoped Securities Group B")

	first := Security{
		GroupID: groupA.ID,
		Name:    "Group A Security",
		ISIN:    "US0000000005",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&first); err != nil {
		t.Fatalf("CreateSecurity(group A) error = %v", err)
	}

	second := Security{
		GroupID: groupB.ID,
		Name:    "Group B Security",
		ISIN:    "US0000000006",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&second); err != nil {
		t.Fatalf("CreateSecurity(group B) error = %v", err)
	}

	items, err := database.ListSecuritiesByGroupID(groupA.ID, 10, 0, "Name", "ASC", "", "")
	if err != nil {
		t.Fatalf("ListSecuritiesByGroupID(group A) error = %v", err)
	}
	if len(items) != 1 || items[0].ID != first.ID || items[0].GroupID != groupA.ID {
		t.Fatalf("group A items = %+v, want only security %+v", items, first)
	}

	allItems, err := database.ListAllSecuritiesByGroupID(groupB.ID)
	if err != nil {
		t.Fatalf("ListAllSecuritiesByGroupID(group B) error = %v", err)
	}
	if len(allItems) != 1 || allItems[0].ID != second.ID || allItems[0].GroupID != groupB.ID {
		t.Fatalf("group B all items = %+v, want only security %+v", allItems, second)
	}
}

func TestListSecuritiesByGroupIDSearchesFields(t *testing.T) {
	database := newSecuritiesTestDB(t)
	group := createSecurityTestGroup(t, database, "Search Securities Group")

	post := Security{
		GroupID: group.ID,
		Name:    "Österreichische Post AG",
		ISIN:    "AT000POST001",
		WKN:     "POST01",
		Symbol:  "POST",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&post); err != nil {
		t.Fatalf("CreateSecurity(post) error = %v", err)
	}

	cola := Security{
		GroupID: group.ID,
		Name:    "Cola Drinks AG",
		ISIN:    "DE000COLA001",
		WKN:     "COLA01",
		Symbol:  "COL",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&cola); err != nil {
		t.Fatalf("CreateSecurity(cola) error = %v", err)
	}

	for _, tc := range []struct {
		name   string
		search string
		wantID int64
	}{
		{name: "lowercase umlaut prefix", search: "öst", wantID: post.ID},
		{name: "uppercase umlaut prefix", search: "Öst", wantID: post.ID},
		{name: "wkn", search: "POST01", wantID: post.ID},
		{name: "symbol", search: "COL", wantID: cola.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, err := database.ListSecuritiesByGroupID(group.ID, 10, 0, "Name", "ASC", "", tc.search)
			if err != nil {
				t.Fatalf("ListSecuritiesByGroupID(search %q) error = %v", tc.search, err)
			}
			if len(items) != 1 || items[0].ID != tc.wantID {
				t.Fatalf("ListSecuritiesByGroupID(search %q) = %+v, want only ID %d", tc.search, items, tc.wantID)
			}

			count, err := database.CountSecuritiesByGroupID(group.ID, "", tc.search)
			if err != nil {
				t.Fatalf("CountSecuritiesByGroupID(search %q) error = %v", tc.search, err)
			}
			if count != 1 {
				t.Fatalf("CountSecuritiesByGroupID(search %q) = %d, want 1", tc.search, count)
			}
		})
	}
}

func TestGetSecurityByIDAndGroupIDDoesNotLeakAcrossGroups(t *testing.T) {
	database := newSecuritiesTestDB(t)
	groupA := createSecurityTestGroup(t, database, "Get Security Group A")
	groupB := createSecurityTestGroup(t, database, "Get Security Group B")

	item := Security{
		GroupID: groupA.ID,
		Name:    "Private Group Security",
		ISIN:    "US0000000007",
		Status:  SecurityStatusActive,
	}
	if err := database.CreateSecurity(&item); err != nil {
		t.Fatalf("CreateSecurity() error = %v", err)
	}

	got, err := database.GetSecurityByIDAndGroupID(item.ID, groupB.ID)
	if err != nil {
		t.Fatalf("GetSecurityByIDAndGroupID(wrong group) error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetSecurityByIDAndGroupID(wrong group) = %+v, want nil", got)
	}
}
