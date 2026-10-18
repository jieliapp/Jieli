package model_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/burugo/thing"
	"github.com/jieliapp/jieli/backend/model"
	_ "github.com/mattn/go-sqlite3"
)

func TestInitSQLiteCreatesEmptySiteSettings(t *testing.T) {
	t.Setenv("PUBLIC_BASE_URL", "https://ignored-legacy.example")
	dbPath := filepath.Join(t.TempDir(), "site-settings.db")
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatal(err)
	}
	values, err := model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if values.InstanceID == "" || values.PublicBaseURL != "" || values.Host != "" || values.Port != 0 || values.Username != "" || values.PasswordEncrypted != "" || values.From != "" || values.TLSMode != "" || values.Tested || values.RegistrationEnabled {
		t.Fatalf("expected empty configuration with a stable instance ID: %+v", values)
	}
}

func TestInitSQLitePreservesSavedSiteSettingsAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "site-settings-restart.db")
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatal(err)
	}
	values, err := model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	instanceID := values.InstanceID
	values.PublicBaseURL = "http://saved.example"
	values.Host = "saved.smtp"
	values.Port = 587
	values.Username = "saved-user"
	values.PasswordEncrypted = "saved-secret"
	values.From = "saved@example.com"
	values.TLSMode = "starttls"
	values.Tested = true
	values.RegistrationEnabled = true
	if err = model.SaveSiteSettings(context.Background(), values); err != nil {
		t.Fatal(err)
	}
	if err = model.InitSQLite(dbPath); err != nil {
		t.Fatal(err)
	}
	got, err := model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.InstanceID != instanceID || got.PublicBaseURL != "http://saved.example" || got.Host != "saved.smtp" || got.Port != 587 || got.Username != "saved-user" || got.PasswordEncrypted != "saved-secret" || got.From != "saved@example.com" || got.TLSMode != "starttls" || !got.Tested || !got.RegistrationEnabled {
		t.Fatalf("restart overwrote saved site settings: %+v", got)
	}
}

func TestSaveSiteSettingsUsesTypedRowsAndKeepsSecretsOutOfPlainValue(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "site-settings-rows.db")); err != nil {
		t.Fatal(err)
	}
	values, err := model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	values.PublicBaseURL = "https://example.test"
	values.Port = 465
	values.PasswordEncrypted = "encrypted-password"
	if err = model.SaveSiteSettings(context.Background(), values); err != nil {
		t.Fatal(err)
	}
	got, err := model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicBaseURL != "https://example.test" || got.Port != 465 || got.PasswordEncrypted != "encrypted-password" {
		t.Fatalf("cached settings were not refreshed after save: %+v", got)
	}
	store, err := thing.Use[*model.SiteSettings]()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := store.Where("name = ?", model.SiteSettingSMTPPassword).Fetch(0, 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("password setting row: %v count=%d", err, len(rows))
	}
	if rows[0].Value != "" || rows[0].EncryptedValue != "encrypted-password" || rows[0].ValueType != "secret" {
		t.Fatalf("secret was not stored separately: %+v", rows[0])
	}
	allRows, err := store.Where("id > ?", 0).Fetch(0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(allRows) != 10 {
		t.Fatalf("expected one row per known setting, got %d", len(allRows))
	}
}

func TestLoadSiteSettingsRejectsInvalidTypedValues(t *testing.T) {
	for _, tc := range []struct {
		name, key, value, valueType string
	}{
		{"wrong_type", model.SiteSettingSMTPPort, "587", "string"},
		{"invalid_integer", model.SiteSettingSMTPPort, "invalid", "int"},
		{"invalid_boolean", model.SiteSettingRegistrationEnabled, "invalid", "bool"},
		{"plaintext_secret", model.SiteSettingSMTPPassword, "plaintext", "secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := model.InitSQLite(filepath.Join(t.TempDir(), "invalid-settings.db")); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(closeModelDB(t))
			store, err := thing.Use[*model.SiteSettings]()
			if err != nil {
				t.Fatal(err)
			}
			if err = store.Save(&model.SiteSettings{Name: tc.key, Value: tc.value, ValueType: tc.valueType}); err != nil {
				t.Fatal(err)
			}
			if _, err = model.LoadSiteSettings(); err == nil {
				t.Fatal("invalid persisted setting was accepted")
			}
		})
	}
}

func TestSaveSiteSettingsRollsBackAllKeysWhenOneWriteFails(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "site-settings-atomic.db")
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatal(err)
	}
	values, err := model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	values.PublicBaseURL = "https://before.example"
	values.Host = "before.smtp"
	if err = model.SaveSiteSettings(context.Background(), values); err != nil {
		t.Fatal(err)
	}
	values.PublicBaseURL = "https://after.example"
	values.Host = "after.smtp"
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if _, err = sqlDB.Exec(`CREATE TRIGGER reject_smtp_host_update BEFORE UPDATE OF value ON site_settings WHEN OLD.name = 'smtp.host' BEGIN SELECT RAISE(ABORT, 'smtp host locked'); END`); err != nil {
		t.Fatal(err)
	}
	if err = model.SaveSiteSettings(context.Background(), values); err == nil {
		t.Fatal("partial site settings save unexpectedly succeeded")
	}
	got, err := model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicBaseURL != "https://before.example" || got.Host != "before.smtp" {
		t.Fatalf("failed transaction changed cached settings: %+v", got)
	}
	if err = model.CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err = model.InitSQLite(dbPath); err != nil {
		t.Fatal(err)
	}
	got, err = model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicBaseURL != "https://before.example" || got.Host != "before.smtp" {
		t.Fatalf("failed transaction changed persisted settings: %+v", got)
	}
}
