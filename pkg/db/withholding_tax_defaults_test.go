package db

import (
	"path/filepath"
	"testing"
)

func newWithholdingTaxDefaultsTestDB(t *testing.T) *DB {
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

func createWithholdingTaxDefaultTestGroup(t *testing.T, database *DB, name string) Group {
	t.Helper()

	group := Group{Name: name}
	if err := database.CreateGroup(&group); err != nil {
		t.Fatalf("CreateGroup(%q) error = %v", name, err)
	}
	return group
}

func createWithholdingTaxDefaultTestDepot(t *testing.T, database *DB, name string) Depot {
	t.Helper()

	depot := Depot{Name: name, BaseCurrency: "EUR"}
	if err := database.CreateDepot(&depot); err != nil {
		t.Fatalf("CreateDepot(%q) error = %v", name, err)
	}
	return depot
}

func TestCreateWithholdingTaxDefaultAllowsSameCountryInDifferentGroups(t *testing.T) {
	database := newWithholdingTaxDefaultsTestDB(t)
	groupA := createWithholdingTaxDefaultTestGroup(t, database, "Tax Group A")
	groupB := createWithholdingTaxDefaultTestGroup(t, database, "Tax Group B")

	first := WithholdingTaxDefault{
		GroupID:                            groupA.ID,
		DepotID:                            0,
		CountryCode:                        "US",
		CountryName:                        "United States",
		WithholdingTaxPercentDefault:       "15",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&first); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault(group A) error = %v", err)
	}

	second := WithholdingTaxDefault{
		GroupID:                            groupB.ID,
		DepotID:                            0,
		CountryCode:                        "US",
		CountryName:                        "United States",
		WithholdingTaxPercentDefault:       "30",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&second); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault(group B same country) error = %v", err)
	}
}

func TestCreateWithholdingTaxDefaultRejectsDuplicateGroupDepotCountry(t *testing.T) {
	database := newWithholdingTaxDefaultsTestDB(t)
	group := createWithholdingTaxDefaultTestGroup(t, database, "Duplicate Tax Group")

	first := WithholdingTaxDefault{
		GroupID:                            group.ID,
		DepotID:                            0,
		CountryCode:                        "CH",
		CountryName:                        "Switzerland",
		WithholdingTaxPercentDefault:       "35",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&first); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault(first) error = %v", err)
	}

	second := WithholdingTaxDefault{
		GroupID:                            group.ID,
		DepotID:                            0,
		CountryCode:                        "CH",
		CountryName:                        "Switzerland",
		WithholdingTaxPercentDefault:       "30",
		WithholdingTaxPercentCreditDefault: "10",
	}
	if err := database.CreateWithholdingTaxDefault(&second); err == nil {
		t.Fatalf("CreateWithholdingTaxDefault(duplicate) error = nil, want error")
	}
}

func TestGetEffectiveWithholdingTaxDefaultPrefersDepotSpecific(t *testing.T) {
	database := newWithholdingTaxDefaultsTestDB(t)
	group := createWithholdingTaxDefaultTestGroup(t, database, "Effective Tax Group")
	depot := createWithholdingTaxDefaultTestDepot(t, database, "Effective Depot")

	groupFallback := WithholdingTaxDefault{
		GroupID:                            group.ID,
		DepotID:                            0,
		CountryCode:                        "US",
		CountryName:                        "United States",
		WithholdingTaxPercentDefault:       "30",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&groupFallback); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault(group fallback) error = %v", err)
	}

	depotSpecific := WithholdingTaxDefault{
		GroupID:                            group.ID,
		DepotID:                            depot.ID,
		CountryCode:                        "US",
		CountryName:                        "United States",
		WithholdingTaxPercentDefault:       "15",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&depotSpecific); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault(depot specific) error = %v", err)
	}

	got, err := database.GetEffectiveWithholdingTaxDefault(group.ID, depot.ID, "US")
	if err != nil {
		t.Fatalf("GetEffectiveWithholdingTaxDefault() error = %v", err)
	}
	if got == nil || got.ID != depotSpecific.ID {
		t.Fatalf("effective default = %+v, want depot-specific %+v", got, depotSpecific)
	}
}

func TestGetEffectiveWithholdingTaxDefaultFallsBackToGroup(t *testing.T) {
	database := newWithholdingTaxDefaultsTestDB(t)
	group := createWithholdingTaxDefaultTestGroup(t, database, "Fallback Tax Group")
	depot := createWithholdingTaxDefaultTestDepot(t, database, "Fallback Depot")

	groupFallback := WithholdingTaxDefault{
		GroupID:                            group.ID,
		DepotID:                            0,
		CountryCode:                        "FR",
		CountryName:                        "France",
		WithholdingTaxPercentDefault:       "25",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&groupFallback); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault(group fallback) error = %v", err)
	}

	got, err := database.GetEffectiveWithholdingTaxDefault(group.ID, depot.ID, "FR")
	if err != nil {
		t.Fatalf("GetEffectiveWithholdingTaxDefault() error = %v", err)
	}
	if got == nil || got.ID != groupFallback.ID {
		t.Fatalf("effective default = %+v, want group fallback %+v", got, groupFallback)
	}
}

func TestListWithholdingTaxDefaultsByGroupIDScopesResults(t *testing.T) {
	database := newWithholdingTaxDefaultsTestDB(t)
	groupA := createWithholdingTaxDefaultTestGroup(t, database, "Scoped Tax Group A")
	groupB := createWithholdingTaxDefaultTestGroup(t, database, "Scoped Tax Group B")

	first := WithholdingTaxDefault{
		GroupID:                            groupA.ID,
		DepotID:                            0,
		CountryCode:                        "IT",
		CountryName:                        "Italy",
		WithholdingTaxPercentDefault:       "26",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&first); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault(group A) error = %v", err)
	}

	second := WithholdingTaxDefault{
		GroupID:                            groupB.ID,
		DepotID:                            0,
		CountryCode:                        "ES",
		CountryName:                        "Spain",
		WithholdingTaxPercentDefault:       "19",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&second); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault(group B) error = %v", err)
	}

	items, err := database.ListWithholdingTaxDefaultsByGroupID(groupA.ID)
	if err != nil {
		t.Fatalf("ListWithholdingTaxDefaultsByGroupID(group A) error = %v", err)
	}
	if len(items) != 1 || items[0].ID != first.ID || items[0].GroupID != groupA.ID {
		t.Fatalf("group A items = %+v, want only default %+v", items, first)
	}
}

func TestDeleteDepotDeletesWithholdingTaxDefaults(t *testing.T) {
	database := newWithholdingTaxDefaultsTestDB(t)
	group := createWithholdingTaxDefaultTestGroup(t, database, "Depot Cascade Tax Group")
	depot := createWithholdingTaxDefaultTestDepot(t, database, "Depot Cascade")

	item := WithholdingTaxDefault{
		GroupID:                            group.ID,
		DepotID:                            depot.ID,
		CountryCode:                        "NL",
		CountryName:                        "Netherlands",
		WithholdingTaxPercentDefault:       "15",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&item); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault() error = %v", err)
	}
	if err := database.DeleteDepot(depot.ID); err != nil {
		t.Fatalf("DeleteDepot() error = %v", err)
	}

	items, err := database.ListWithholdingTaxDefaultsByGroupID(group.ID)
	if err != nil {
		t.Fatalf("ListWithholdingTaxDefaultsByGroupID() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("withholding defaults after depot delete = %+v, want none", items)
	}
}

func TestDeleteGroupDeletesWithholdingTaxDefaults(t *testing.T) {
	database := newWithholdingTaxDefaultsTestDB(t)
	group := createWithholdingTaxDefaultTestGroup(t, database, "Group Cascade Tax Group")

	item := WithholdingTaxDefault{
		GroupID:                            group.ID,
		DepotID:                            0,
		CountryCode:                        "BE",
		CountryName:                        "Belgium",
		WithholdingTaxPercentDefault:       "30",
		WithholdingTaxPercentCreditDefault: "15",
	}
	if err := database.CreateWithholdingTaxDefault(&item); err != nil {
		t.Fatalf("CreateWithholdingTaxDefault() error = %v", err)
	}
	if err := database.DeleteGroup(group.ID); err != nil {
		t.Fatalf("DeleteGroup() error = %v", err)
	}

	got, err := database.GetWithholdingTaxDefaultByIDAndGroupID(item.ID, group.ID)
	if err != nil {
		t.Fatalf("GetWithholdingTaxDefaultByIDAndGroupID() error = %v", err)
	}
	if got != nil {
		t.Fatalf("withholding default after group delete = %+v, want nil", got)
	}
}
