package model_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	_ "github.com/mattn/go-sqlite3"
)

func TestInitSQLiteShouldRejectDestructiveLegacyMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	db := openSQLite(t, dbPath)
	if _, err := db.Exec(`CREATE TABLE threads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted BOOLEAN,
		user_id INTEGER NOT NULL,
		provider TEXT NOT NULL,
		provider_thread_id TEXT NOT NULL,
		title TEXT NOT NULL,
		visibility TEXT NOT NULL DEFAULT 'private',
		share_slug TEXT NOT NULL,
		repo TEXT NOT NULL,
		branch TEXT NOT NULL,
		source_url TEXT NOT NULL,
		data_json TEXT NOT NULL,
		search_text TEXT NOT NULL DEFAULT '',
		message_count INTEGER NOT NULL DEFAULT 0,
		lines_added INTEGER NOT NULL DEFAULT 0,
		lines_removed INTEGER NOT NULL DEFAULT 0,
		files_changed INTEGER NOT NULL DEFAULT 0,
		archived BOOLEAN NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatalf("create legacy threads table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close setup db: %v", err)
	}
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err == nil || !strings.Contains(err.Error(), "explicit migration") {
		t.Fatalf("legacy schema should require explicit migration, got %v", err)
	}
	db = openSQLite(t, dbPath)
	defer db.Close()
	if !tableColumns(t, db, "threads")["data_json"] {
		t.Fatal("startup deleted legacy data column")
	}
}

func TestInitSQLiteShouldAddUploadFingerprintToExistingThreads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("initial init sqlite: %v", err)
	}
	if err := model.ThreadDB.Save(&model.Thread{UserID: 1, Provider: model.ProviderAmp, ProviderThreadID: "T-existing", Title: "Existing", Visibility: model.ThreadVisibilityPrivate}); err != nil {
		t.Fatalf("save existing thread: %v", err)
	}
	db := openSQLite(t, dbPath)
	if _, err := db.Exec("ALTER TABLE threads DROP COLUMN upload_fingerprint"); err != nil {
		t.Fatalf("drop upload_fingerprint column: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db before remigration: %v", err)
	}

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("reinit sqlite: %v", err)
	}
	db = openSQLite(t, dbPath)
	if !tableColumns(t, db, "threads")["upload_fingerprint"] {
		t.Fatalf("expected threads upload_fingerprint column after remigration")
	}
}

func TestInitSQLiteShouldMigrateCoreTablesWithSpecColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}

	db := openSQLite(t, dbPath)
	expectedColumns := map[string][]string{
		"users": {"id", "username", "github_id", "google_id", "api_key_hash", "api_key_prefix", "api_key_encrypted", "preferred_locale", "plan_level", "usage_info"},
		"threads": {
			"id", "user_id", "provider", "provider_thread_id", "visibility", "share_slug", "search_text", "thread_v",
			"provider_created_ms", "user_last_interacted_at_ms", "provider_updated_at", "creator_user_id", "agent_mode",
			"uses_dtw", "uses_thread_actors", "env_json", "meta_json", "relationships_json", "summary_stats_json", "reasoning_effort", "upload_fingerprint", "preview", "repo_url",
		},
		"thread_messages": {"id", "thread_id", "position", "provider_message_id", "protocol_message_id", "role", "raw_zstd", "content_hash"},
		"thread_labels":   {"id", "user_id", "label", "thread_id"},
		"daily_activity":  {"id", "user_id", "date", "thread_count", "message_count", "lines_added", "lines_removed", "lines_changed"},
		"site_settings":   {"id", "name", "value", "encrypted_value", "value_type"},
		"attachments":     {"id", "deleted", "user_id", "public_id", "media_type", "extension", "storage_driver", "storage_key", "size_bytes"},
		"thread_attachments": {
			"id", "user_id", "thread_id", "attachment_id", "public_id", "storage_driver", "storage_key",
		},
		"subscriptions": {
			"id", "user_id", "plan_level", "status", "starts_at_ms", "ends_at_ms", "source",
			"external_subscription_id", "external_group_id", "last_synced_at_ms", "metadata_json",
		},
	}
	for table, columns := range expectedColumns {
		actual := tableColumns(t, db, table)
		for _, column := range columns {
			if !actual[column] {
				t.Fatalf("expected table %q to include column %q", table, column)
			}
		}
	}
	if tableColumns(t, db, "attachments")["data"] {
		t.Fatalf("expected attachments table not to include legacy data column")
	}
	if tableColumns(t, db, "thread_messages")["content_json"] {
		t.Fatalf("expected thread_messages table not to include legacy content_json column")
	}
	if !tableIndexes(t, db, "thread_labels")["idx_thread_labels_user_label_thread"] {
		t.Fatalf("expected thread_labels user/label/thread index")
	}
	if !tableIndexes(t, db, "users")["idx_users_api_key_prefix"] {
		t.Fatalf("expected users api_key_prefix index")
	}
}

func TestInitSQLiteShouldEnableWALJournalMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}

	// journal_mode is a persistent property of the database file, so a fresh
	// connection observes the WAL mode that InitSQLite enabled.
	db := openSQLite(t, dbPath)
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if strings.ToLower(journalMode) != "wal" {
		t.Fatalf("expected journal_mode wal, got %q", journalMode)
	}
}

func TestInitSQLiteShouldInitializeInMemoryDatabase(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(":memory:"); err != nil {
		t.Fatalf("init in-memory sqlite: %v", err)
	}
	if err := model.ThreadDB.Save(&model.Thread{UserID: 1, Provider: model.ProviderAmp, ProviderThreadID: "T-mem", Title: "mem", Visibility: model.ThreadVisibilityPrivate}); err != nil {
		t.Fatalf("save thread on in-memory db: %v", err)
	}
}

func TestInitSQLiteShouldAllowIndependentGitHubAndGoogleUsers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	if err := model.UserDB.Save(&model.User{Username: "github-one", GitHubID: "github-one", APIKeyHash: "hash-github-one"}); err != nil {
		t.Fatalf("save first GitHub user: %v", err)
	}
	if err := model.UserDB.Save(&model.User{Username: "github-two", GitHubID: "github-two", APIKeyHash: "hash-github-two"}); err != nil {
		t.Fatalf("save second GitHub user with empty google_id: %v", err)
	}
	if err := model.UserDB.Save(&model.User{Username: "google-one", GoogleID: "google-one", APIKeyHash: "hash-google-one"}); err != nil {
		t.Fatalf("save first Google user with empty github_id: %v", err)
	}
	if err := model.UserDB.Save(&model.User{Username: "google-two", GoogleID: "google-two", APIKeyHash: "hash-google-two"}); err != nil {
		t.Fatalf("save second Google user with empty github_id: %v", err)
	}
}

func TestInitSQLiteShouldPreserveExistingColumnsAndRowsOnRemigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("initial init sqlite: %v", err)
	}
	if err := model.UserDB.Save(&model.User{
		Username:     "alice",
		GitHubID:     "github-alice",
		APIKeyHash:   "api-key-hash-alice",
		APIKeyPrefix: "jieli_alice",
	}); err != nil {
		t.Fatalf("save user: %v", err)
	}

	db := openSQLite(t, dbPath)
	if _, err := db.Exec("ALTER TABLE users ADD COLUMN legacy_note TEXT"); err != nil {
		t.Fatalf("add legacy column: %v", err)
	}
	if _, err := db.Exec("UPDATE users SET legacy_note = 'keep-me' WHERE github_id = 'github-alice'"); err != nil {
		t.Fatalf("write legacy column: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close inspection db: %v", err)
	}

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("reinit sqlite: %v", err)
	}

	db = openSQLite(t, dbPath)
	if !tableColumns(t, db, "users")["legacy_note"] {
		t.Fatalf("expected remigration to preserve unknown legacy_note column")
	}
	var note string
	if err := db.QueryRow("SELECT legacy_note FROM users WHERE github_id = ?", "github-alice").Scan(&note); err != nil {
		t.Fatalf("read legacy column: %v", err)
	}
	if note != "keep-me" {
		t.Fatalf("expected legacy value to survive remigration, got %q", note)
	}
}

func TestThreadShareSlugShouldBeUniqueWhenPresent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	if err := model.ThreadDB.Save(&model.Thread{
		UserID:           1,
		Provider:         model.ProviderAmp,
		ProviderThreadID: "T-one",
		Visibility:       model.ThreadVisibilityUnlisted,
		ShareSlug:        "shared-slug",
	}); err != nil {
		t.Fatalf("save first shared thread: %v", err)
	}

	err := model.ThreadDB.Save(&model.Thread{
		UserID:           2,
		Provider:         model.ProviderAmp,
		ProviderThreadID: "T-two",
		Visibility:       model.ThreadVisibilityUnlisted,
		ShareSlug:        "shared-slug",
	})
	if err == nil {
		t.Fatalf("expected duplicate share slug to be rejected")
	}
}

func TestAttachmentPublicIDShouldBeUnique(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	if err := model.AttachmentDB.Save(&model.Attachment{
		UserID:        1,
		PublicID:      "attachment-id",
		MediaType:     "image/png",
		Extension:     "png",
		StorageDriver: "local",
		StorageKey:    "attachment-id.png",
		SizeBytes:     9,
	}); err != nil {
		t.Fatalf("save first attachment: %v", err)
	}

	err := model.AttachmentDB.Save(&model.Attachment{
		UserID:        2,
		PublicID:      "attachment-id",
		MediaType:     "image/png",
		Extension:     "png",
		StorageDriver: "local",
		StorageKey:    "other-attachment-id.png",
		SizeBytes:     15,
	})
	if err == nil {
		t.Fatalf("expected duplicate attachment public id to be rejected")
	}
}

func TestPrivateThreadsShouldNotRequireShareSlug(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))

	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	for _, providerThreadID := range []string{"T-one", "T-two"} {
		if err := model.ThreadDB.Save(&model.Thread{
			UserID:           1,
			Provider:         model.ProviderAmp,
			ProviderThreadID: providerThreadID,
			Visibility:       model.ThreadVisibilityPrivate,
		}); err != nil {
			t.Fatalf("save private thread %s without share slug: %v", providerThreadID, err)
		}
	}
}

func closeModelDB(t *testing.T) func() {
	t.Helper()
	return func() {
		if err := model.CloseDB(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}

func openSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func tableColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("read table info for %s: %v", table, err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table info for %s: %v", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table info for %s: %v", table, err)
	}
	if len(columns) == 0 {
		t.Fatalf("expected table %q to exist", table)
	}
	return columns
}

func tableIndexes(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.Query("PRAGMA index_list(" + table + ")")
	if err != nil {
		t.Fatalf("read index list for %s: %v", table, err)
	}
	defer rows.Close()

	indexes := make(map[string]bool)
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index list for %s: %v", table, err)
		}
		indexes[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate index list for %s: %v", table, err)
	}
	return indexes
}

func TestStandaloneUpgradePreservesAccountsHistoryAndGiftExpiry(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "upgrade.db")
	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeModelDB(t))
	user := &model.User{Username: "legacyuser", Email: "legacy@example.com", GitHubID: "legacy-github", APIKeyHash: "legacy-key", PlanLevel: "pro"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatal(err)
	}
	thread := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-legacy", Title: "Preserved legacy conversation", Visibility: "private", MessageCount: 1}
	if err := model.ThreadDB.Save(thread); err != nil {
		t.Fatal(err)
	}
	gift := &model.Subscription{UserID: user.ID, PlanLevel: "pro", Status: "active", Source: model.SubscriptionSourceNeo, StartsAtMS: 1700000000000, EndsAtMS: 1900000000000}
	if err := model.SubscriptionDB.Save(gift); err != nil {
		t.Fatal(err)
	}
	if err := model.CloseDB(); err != nil {
		t.Fatal(err)
	}
	// Strip only the newly introduced account schema in this disposable fixture.
	db := openSQLite(t, dbPath)
	for _, sql := range []string{"DROP INDEX uniq_users_login_email_present", "DROP INDEX idx_users_login_email", "ALTER TABLE users DROP COLUMN login_email", "ALTER TABLE users DROP COLUMN password_hash", "ALTER TABLE users DROP COLUMN is_admin", "ALTER TABLE users DROP COLUMN must_change_password", "DROP TABLE email_tokens"} {
		if _, err := db.Exec(sql); err != nil {
			t.Fatalf("prepare pre-upgrade schema %s: %v", sql, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatal(err)
	}
	updated, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != user.Username || updated.APIKeyHash != user.APIKeyHash || updated.GitHubID != user.GitHubID || updated.PlanLevel != "pro" || updated.IsAdmin || updated.LoginEmail != "" || updated.PasswordHash != "" {
		t.Fatal("upgrade changed existing identity or rights")
	}
	history, err := model.ThreadDB.ByID(thread.ID)
	if err != nil || history.Title != thread.Title || history.MessageCount != 1 {
		t.Fatal("upgrade changed history")
	}
	subscription, err := model.SubscriptionDB.ByID(gift.ID)
	if err != nil || subscription.EndsAtMS != gift.EndsAtMS || subscription.Source != gift.Source || subscription.Status != gift.Status {
		t.Fatal("upgrade changed gift")
	}
}
