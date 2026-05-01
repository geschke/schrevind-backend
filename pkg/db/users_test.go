package db

import (
	"context"
	"path/filepath"
	"testing"
)

func newUsersTestDB(t *testing.T) *DB {
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

func createSettingsTestUser(t *testing.T, database *DB, email string) int64 {
	t.Helper()

	id, err := database.CreateUser(context.Background(), User{
		Email:    email,
		Password: "secret",
		Locale:   "en-US",
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	return id
}

func TestUserSettingsDefaultToZero(t *testing.T) {
	database := newUsersTestDB(t)
	userID := createSettingsTestUser(t, database, "settings-default@example.com")

	user, found, err := database.GetUserByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !found {
		t.Fatalf("GetUserByID() found = false, want true")
	}
	if user.Settings == nil {
		t.Fatalf("Settings = nil, want default settings")
	}
	if user.Settings.LastActiveGroupID != 0 {
		t.Fatalf("LastActiveGroupID = %d, want 0", user.Settings.LastActiveGroupID)
	}
	if user.Settings.InlandTaxTemplate != "" {
		t.Fatalf("InlandTaxTemplate = %q, want empty", user.Settings.InlandTaxTemplate)
	}
}

func TestUpdateUserSettingsStoresSettings(t *testing.T) {
	database := newUsersTestDB(t)
	userID := createSettingsTestUser(t, database, "settings-update@example.com")

	updated, err := database.UpdateUserSettings(context.Background(), userID, UserSettings{
		LastActiveGroupID: 42,
		Theme:             "dark",
		InlandTaxTemplate: "DE",
	})
	if err != nil {
		t.Fatalf("UpdateUserSettings() error = %v", err)
	}
	if !updated {
		t.Fatalf("UpdateUserSettings() updated = false, want true")
	}

	user, found, err := database.GetUserByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !found {
		t.Fatalf("GetUserByID() found = false, want true")
	}
	if user.Settings == nil || user.Settings.LastActiveGroupID != 42 || user.Settings.Theme != "dark" || user.Settings.InlandTaxTemplate != "DE" {
		t.Fatalf("Settings = %+v, want LastActiveGroupID 42, Theme dark and InlandTaxTemplate DE", user.Settings)
	}
}

func TestListUsersDoesNotLoadSettings(t *testing.T) {
	database := newUsersTestDB(t)
	userID := createSettingsTestUser(t, database, "settings-list@example.com")

	if _, err := database.UpdateUserSettings(context.Background(), userID, UserSettings{LastActiveGroupID: 7}); err != nil {
		t.Fatalf("UpdateUserSettings() error = %v", err)
	}

	items, err := database.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].Settings != nil {
		t.Fatalf("ListUsers Settings = %+v, want nil", items[0].Settings)
	}
}

func TestUserSettingsInvalidJSONDefaultsToZero(t *testing.T) {
	database := newUsersTestDB(t)
	userID := createSettingsTestUser(t, database, "settings-invalid@example.com")

	if _, err := database.SQL.Exec(`UPDATE users SET settings = ? WHERE id = ?;`, `{not-json`, userID); err != nil {
		t.Fatalf("corrupt settings: %v", err)
	}

	user, found, err := database.GetUserByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if !found {
		t.Fatalf("GetUserByID() found = false, want true")
	}
	if user.Settings == nil || user.Settings.LastActiveGroupID != 0 {
		t.Fatalf("Settings = %+v, want default settings", user.Settings)
	}
}
