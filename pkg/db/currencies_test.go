package db

import (
	"context"
	"path/filepath"
	"testing"
)

func newCurrencyTestDB(t *testing.T) *DB {
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

func TestCreateGroupWithDefaultCurrenciesCopiesTemplateCurrencies(t *testing.T) {
	database := newCurrencyTestDB(t)

	group := Group{Name: "Example Group"}
	if err := database.CreateGroupWithDefaultCurrencies(&group); err != nil {
		t.Fatalf("CreateGroupWithDefaultCurrencies() error = %v", err)
	}

	eur, err := database.GetCurrencyByCurrencyAndGroupID("EUR", group.ID)
	if err != nil {
		t.Fatalf("GetCurrencyByCurrencyAndGroupID(EUR) error = %v", err)
	}
	if eur == nil || eur.GroupID != group.ID || eur.Currency != "EUR" || eur.DecimalPlaces != 2 {
		t.Fatalf("EUR copy = %+v, want group-scoped EUR with decimal_places 2", eur)
	}

	usd, err := database.GetCurrencyByCurrencyAndGroupID("USD", group.ID)
	if err != nil {
		t.Fatalf("GetCurrencyByCurrencyAndGroupID(USD) error = %v", err)
	}
	if usd == nil || usd.GroupID != group.ID || usd.Currency != "USD" || usd.DecimalPlaces != 2 {
		t.Fatalf("USD copy = %+v, want group-scoped USD with decimal_places 2", usd)
	}
}

func TestCreateGroupWithDefaultCurrenciesAndAdminAssignsCreator(t *testing.T) {
	database := newCurrencyTestDB(t)

	user := User{
		Email:     "creator@example.com",
		Password:  "secret",
		FirstName: "Example",
		LastName:  "Creator",
		Status:    "active",
		Locale:    "en-US",
	}
	if _, err := database.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	creator, found, err := database.GetUserByEmail(context.Background(), user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail() error = %v", err)
	}
	if !found {
		t.Fatalf("GetUserByEmail() found = false, want true")
	}

	group := Group{Name: "Example Group With Admin"}
	if err := database.CreateGroupWithDefaultCurrenciesAndAdmin(&group, creator.ID); err != nil {
		t.Fatalf("CreateGroupWithDefaultCurrenciesAndAdmin() error = %v", err)
	}

	inGroup, err := database.IsUserInGroup(group.ID, creator.ID)
	if err != nil {
		t.Fatalf("IsUserInGroup() error = %v", err)
	}
	if !inGroup {
		t.Fatalf("IsUserInGroup() = false, want true")
	}

	membership, found, err := database.GetMembership(EntityTypeGroup, group.ID, creator.ID)
	if err != nil {
		t.Fatalf("GetMembership() error = %v", err)
	}
	if !found {
		t.Fatalf("GetMembership() found = false, want true")
	}
	if membership.Role != RoleGroupAdmin {
		t.Fatalf("membership role = %q, want %q", membership.Role, RoleGroupAdmin)
	}
}

func TestGetCurrencyByCurrencyAndGroupIDScopesByGroup(t *testing.T) {
	database := newCurrencyTestDB(t)

	currencyA := Currency{
		GroupID:       1,
		Currency:      "CHF",
		Name:          "Swiss Franc A",
		DecimalPlaces: 2,
		Status:        CurrencyStatusActive,
	}
	if err := database.CreateCurrency(&currencyA); err != nil {
		t.Fatalf("CreateCurrency(group 1) error = %v", err)
	}

	currencyB := Currency{
		GroupID:       2,
		Currency:      "CHF",
		Name:          "Swiss Franc B",
		DecimalPlaces: 3,
		Status:        CurrencyStatusActive,
	}
	if err := database.CreateCurrency(&currencyB); err != nil {
		t.Fatalf("CreateCurrency(group 2) error = %v", err)
	}

	gotA, err := database.GetCurrencyByCurrencyAndGroupID("CHF", 1)
	if err != nil {
		t.Fatalf("GetCurrencyByCurrencyAndGroupID(group 1) error = %v", err)
	}
	if gotA == nil || gotA.Name != "Swiss Franc A" || gotA.DecimalPlaces != 2 {
		t.Fatalf("group 1 currency = %+v, want Swiss Franc A / 2", gotA)
	}

	gotB, err := database.GetCurrencyByCurrencyAndGroupID("CHF", 2)
	if err != nil {
		t.Fatalf("GetCurrencyByCurrencyAndGroupID(group 2) error = %v", err)
	}
	if gotB == nil || gotB.Name != "Swiss Franc B" || gotB.DecimalPlaces != 3 {
		t.Fatalf("group 2 currency = %+v, want Swiss Franc B / 3", gotB)
	}
}
