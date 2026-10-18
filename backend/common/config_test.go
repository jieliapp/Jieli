package common_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/common"
)

func TestDefaultConfigShouldRunJieliOnSQLiteWithEnglishLocale(t *testing.T) {
	cfg := common.DefaultConfig()

	if cfg.Server.Host != "0.0.0.0" {
		t.Fatalf("expected default host 0.0.0.0, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Fatalf("expected default database driver sqlite, got %q", cfg.Database.Driver)
	}
	if cfg.Database.SQLitePath != "data/jieli.db" {
		t.Fatalf("expected default sqlite path data/jieli.db, got %q", cfg.Database.SQLitePath)
	}
	if cfg.Attachments.Driver != "local" {
		t.Fatalf("expected default attachment storage driver local, got %q", cfg.Attachments.Driver)
	}
	if cfg.Attachments.LocalDir != "data/attachments" {
		t.Fatalf("expected default attachment storage dir data/attachments, got %q", cfg.Attachments.LocalDir)
	}
	if cfg.I18n.DefaultLocale != "en" {
		t.Fatalf("expected default locale en, got %q", cfg.I18n.DefaultLocale)
	}
}

func TestPrepareRuntimeShouldCreateSQLiteParentAndAttachmentDirectories(t *testing.T) {
	runtimeDir := t.TempDir()
	dbPath := filepath.Join(runtimeDir, "nested", "jieli.db")
	attachmentDir := filepath.Join(runtimeDir, "attachments")
	cfg := common.DefaultConfig()
	cfg.Database.SQLitePath = dbPath
	cfg.Attachments.LocalDir = attachmentDir

	if err := common.PrepareRuntime(cfg); err != nil {
		t.Fatalf("prepare runtime: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
		t.Fatalf("expected sqlite parent directory to exist: %v", err)
	}
	if _, err := os.Stat(attachmentDir); err != nil {
		t.Fatalf("expected attachment directory to exist: %v", err)
	}
}

func TestDefaultConfigShouldIncludeGitHubOAuthSettings(t *testing.T) {
	cfg := common.DefaultConfig()

	if cfg.GitHub.ClientID != "" {
		t.Fatalf("expected default GitHub client ID to be empty, got %q", cfg.GitHub.ClientID)
	}
	if cfg.GitHub.ClientSecret != "" {
		t.Fatalf("expected default GitHub client secret to be empty")
	}
	if cfg.GitHub.CallbackURL != "" {
		t.Fatalf("expected default GitHub callback URL to be empty, got %q", cfg.GitHub.CallbackURL)
	}
	if cfg.GitHub.PostLoginRedirect != "/web/dashboard" {
		t.Fatalf("expected default post-login redirect /web/dashboard, got %q", cfg.GitHub.PostLoginRedirect)
	}
}

func TestDefaultConfigShouldIncludeGoogleOAuthSettings(t *testing.T) {
	cfg := common.DefaultConfig()

	if cfg.Google.ClientID != "" {
		t.Fatalf("expected default Google client ID to be empty, got %q", cfg.Google.ClientID)
	}
	if cfg.Google.ClientSecret != "" {
		t.Fatalf("expected default Google client secret to be empty")
	}
	if cfg.Google.CallbackURL != "" {
		t.Fatalf("expected default Google callback URL to be empty, got %q", cfg.Google.CallbackURL)
	}
	if cfg.Google.PostLoginRedirect != "/web/dashboard" {
		t.Fatalf("expected default Google post-login redirect /web/dashboard, got %q", cfg.Google.PostLoginRedirect)
	}
}

func TestDefaultConfigShouldUseNonSecureCookies(t *testing.T) {
	cfg := common.DefaultConfig()

	if cfg.Security.CookieSecure {
		t.Fatalf("expected default cookie secure flag to be false")
	}
}

func TestConfigFromEnvShouldLoadGitHubOAuthAndSessionSettings(t *testing.T) {
	t.Setenv("GITHUB_CLIENT_ID", "github-client-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "github-client-secret")
	t.Setenv("GITHUB_CALLBACK_URL", "http://api.pika12.com:8080/api/auth/callback/github")
	t.Setenv("SESSION_SECRET", "session-secret")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("DEV", "1")

	cfg := common.ConfigFromEnv()

	if cfg.GitHub.ClientID != "github-client-id" {
		t.Fatalf("expected env GitHub client ID, got %q", cfg.GitHub.ClientID)
	}
	if cfg.GitHub.ClientSecret != "github-client-secret" {
		t.Fatalf("expected env GitHub client secret")
	}
	if cfg.GitHub.CallbackURL != "http://api.pika12.com:8080/api/auth/callback/github" {
		t.Fatalf("expected env callback URL, got %q", cfg.GitHub.CallbackURL)
	}
	if cfg.Security.SessionSecret != "session-secret" {
		t.Fatalf("expected env session secret")
	}
	if !cfg.Security.CookieSecure {
		t.Fatalf("expected env cookie secure flag")
	}
	if !cfg.DevMode {
		t.Fatalf("expected env dev mode")
	}
}

func TestConfigFromEnvShouldLoadGoogleOAuthSettings(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "google-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "google-client-secret")
	t.Setenv("GOOGLE_CALLBACK_URL", "https://jieli.example.com/web/api/auth/callback/google")
	t.Setenv("GOOGLE_POST_LOGIN_REDIRECT", "/threads")

	cfg := common.ConfigFromEnv()

	if cfg.Google.ClientID != "google-client-id" {
		t.Fatalf("expected env Google client ID, got %q", cfg.Google.ClientID)
	}
	if cfg.Google.ClientSecret != "google-client-secret" {
		t.Fatalf("expected env Google client secret")
	}
	if cfg.Google.CallbackURL != "https://jieli.example.com/web/api/auth/callback/google" {
		t.Fatalf("expected env Google callback URL, got %q", cfg.Google.CallbackURL)
	}
	if cfg.Google.PostLoginRedirect != "/threads" {
		t.Fatalf("expected env Google post-login redirect /threads, got %q", cfg.Google.PostLoginRedirect)
	}
}

func TestConfigFromEnvShouldLoadServerAndDatabaseSettings(t *testing.T) {
	t.Setenv("HOST", "127.0.0.1")
	t.Setenv("PORT", "9090")
	t.Setenv("SQLITE_PATH", "/data/jieli.db")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32, 10.66.0.2/32,,")
	t.Setenv("ATTACHMENT_STORAGE_DIR", "/data/attachments")

	cfg := common.ConfigFromEnv()

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("expected env host, got %q", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Fatalf("expected env port, got %d", cfg.Server.Port)
	}
	if cfg.Database.SQLitePath != "/data/jieli.db" {
		t.Fatalf("expected env sqlite path, got %q", cfg.Database.SQLitePath)
	}
	if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[0] != "127.0.0.1/32" || cfg.Server.TrustedProxies[1] != "10.66.0.2/32" {
		t.Fatalf("expected trusted proxies from env, got %#v", cfg.Server.TrustedProxies)
	}
	if cfg.Attachments.LocalDir != "/data/attachments" {
		t.Fatalf("expected env attachment storage dir, got %q", cfg.Attachments.LocalDir)
	}
}

func TestConfigFromEnvShouldEnableR2AttachmentStorageWhenCompleteSettingsExist(t *testing.T) {
	t.Setenv("R2_ENDPOINT", "https://account.r2.cloudflarestorage.com")
	t.Setenv("R2_BUCKET", "jieli-attachments")
	t.Setenv("R2_ACCESS_KEY_ID", "r2-access-key")
	t.Setenv("R2_SECRET_ACCESS_KEY", "r2-secret-key")
	t.Setenv("R2_PUBLIC_BASE_URL", "https://cdn.jieli.app")

	cfg := common.ConfigFromEnv()

	if cfg.Attachments.Driver != "r2" {
		t.Fatalf("expected complete R2 settings to select r2 driver, got %q", cfg.Attachments.Driver)
	}
	if cfg.Attachments.R2Endpoint != "https://account.r2.cloudflarestorage.com" {
		t.Fatalf("expected R2 endpoint from env, got %q", cfg.Attachments.R2Endpoint)
	}
	if cfg.Attachments.R2Bucket != "jieli-attachments" {
		t.Fatalf("expected R2 bucket from env, got %q", cfg.Attachments.R2Bucket)
	}
	if cfg.Attachments.R2AccessKeyID != "r2-access-key" {
		t.Fatalf("expected R2 access key ID from env, got %q", cfg.Attachments.R2AccessKeyID)
	}
	if cfg.Attachments.R2SecretAccessKey != "r2-secret-key" {
		t.Fatalf("expected R2 secret access key from env")
	}
	if cfg.Attachments.R2Region != "auto" {
		t.Fatalf("expected default R2 region auto, got %q", cfg.Attachments.R2Region)
	}
	if cfg.Attachments.R2PublicBaseURL != "https://cdn.jieli.app" {
		t.Fatalf("expected R2 public base URL from env, got %q", cfg.Attachments.R2PublicBaseURL)
	}
}

func TestPrepareRuntimeShouldRejectPartialR2AttachmentStorageSettings(t *testing.T) {
	cfg := common.DefaultConfig()
	cfg.Attachments.R2Endpoint = "https://account.r2.cloudflarestorage.com"

	if err := common.PrepareRuntime(cfg); err == nil {
		t.Fatalf("expected partial R2 settings to fail runtime preparation")
	}
}
