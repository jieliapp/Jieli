package model

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"

	"github.com/burugo/thing"
	"github.com/burugo/thing/drivers/db/sqlite"
	"github.com/burugo/thing/thingtest"
)

var dbAdapter thing.DBAdapter

func InitSQLite(path string) error {
	if path == "" {
		return errors.New("sqlite path is required")
	}

	if dbAdapter != nil {
		if err := CloseDB(); err != nil {
			return err
		}
	}

	adapter, err := sqlite.NewSQLiteAdapter(sqliteDSN(path))
	if err != nil {
		return err
	}
	if path == ":memory:" {
		adapter.DB().SetMaxOpenConns(1)
	}

	// Older inline transcripts need an explicit, audited data migration. Never
	// silently discard their column to make new inserts succeed.
	columns, err := adapter.DB().Query("PRAGMA table_info(threads)")
	if err != nil {
		_ = adapter.Close()
		return err
	}
	legacyRequired := false
	for columns.Next() {
		var cid, required, primary int
		var name, kind string
		var defaultValue sql.NullString
		if err = columns.Scan(&cid, &name, &kind, &required, &defaultValue, &primary); err != nil {
			break
		}
		if name == "data_json" && required != 0 && !defaultValue.Valid {
			legacyRequired = true
		}
	}
	if err == nil {
		err = columns.Err()
	}
	_ = columns.Close()
	if err != nil {
		_ = adapter.Close()
		return err
	}
	if legacyRequired {
		_ = adapter.Close()
		return errors.New("legacy threads.data_json requires explicit migration before upgrade; back up and migrate transcripts first")
	}

	thing.AllowDropColumn = false
	if os.Getenv("DEBUG") == "1" {
		thing.SetLogLevel(thing.LogDebug)
	}
	if err := thing.Configure(adapter, cacheForRuntime()); err != nil {
		_ = adapter.Close()
		return err
	}
	if err := thing.AutoMigrate(&SiteSettings{}, &EmailToken{}, &User{}, &Subscription{}, &Thread{}, &ThreadMessage{}, &ThreadLabel{}, &DailyActivity{}, &Attachment{}, &ThreadAttachment{}); err != nil {
		_ = adapter.Close()
		return err
	}
	if err := ensureSQLiteIndexes(adapter); err != nil {
		_ = adapter.Close()
		return err
	}

	dbAdapter = adapter
	if err := initModelHandles(); err != nil {
		_ = CloseDB()
		return err
	}
	if err := EnsureSiteSettings(); err != nil {
		_ = CloseDB()
		return err
	}
	return nil
}

// sqliteDSN appends write-tuning pragmas to a file-backed SQLite DSN so the
// mattn driver opens the database in WAL mode with relaxed sync and a busy
// timeout. WAL collapses the per-commit fsync cost that dominates the
// write-heavy upload path. The in-memory DSN is returned unchanged because WAL
// does not apply to ":memory:".
func sqliteDSN(path string) string {
	if path == ":memory:" {
		return path
	}
	const params = "_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_txlock=immediate"
	if strings.Contains(path, "?") {
		return path + "&" + params
	}
	return path + "?" + params
}

func cacheForRuntime() thing.CacheClient {
	if strings.HasSuffix(os.Args[0], ".test") {
		return thingtest.NewMockCache()
	}
	return nil
}

func CloseDB() error {
	resetModelHandles()
	if dbAdapter == nil {
		return nil
	}
	err := dbAdapter.Close()
	dbAdapter = nil
	return err
}

func ensureSQLiteIndexes(adapter thing.DBAdapter) error {
	statements := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_login_email_present ON users (login_email) WHERE login_email <> ''",
		"DROP INDEX IF EXISTS uniq_users_github_id",
		"DROP INDEX IF EXISTS uniq_users_google_id",
		"CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_github_id_present ON users (github_id) WHERE github_id <> ''",
		"CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_google_id_present ON users (google_id) WHERE google_id <> ''",
		"CREATE UNIQUE INDEX IF NOT EXISTS uniq_threads_share_slug_present ON threads (share_slug) WHERE share_slug <> ''",
		"CREATE UNIQUE INDEX IF NOT EXISTS uniq_attachments_public_id ON attachments (public_id)",
	}
	for _, statement := range statements {
		if _, err := adapter.Exec(context.Background(), statement); err != nil {
			return err
		}
	}
	return nil
}

func initModelHandles() error {
	var err error
	UserDB, err = thing.Use[*User]()
	if err != nil {
		return err
	}
	SubscriptionDB, err = thing.Use[*Subscription]()
	if err != nil {
		return err
	}
	ThreadDB, err = thing.Use[*Thread]()
	if err != nil {
		return err
	}
	ThreadMessageDB, err = thing.Use[*ThreadMessage]()
	if err != nil {
		return err
	}
	ThreadLabelDB, err = thing.Use[*ThreadLabel]()
	if err != nil {
		return err
	}
	DailyActivityDB, err = thing.Use[*DailyActivity]()
	if err != nil {
		return err
	}
	AttachmentDB, err = thing.Use[*Attachment]()
	if err != nil {
		return err
	}
	ThreadAttachmentDB, err = thing.Use[*ThreadAttachment]()
	if err != nil {
		return err
	}
	return nil
}

func resetModelHandles() {
	UserDB = nil
	SubscriptionDB = nil
	ThreadDB = nil
	ThreadMessageDB = nil
	ThreadLabelDB = nil
	DailyActivityDB = nil
	AttachmentDB = nil
	ThreadAttachmentDB = nil
}
