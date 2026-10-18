package common

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultHost                    = "0.0.0.0"
	DefaultPort                    = 8080
	DefaultDBDriver                = "sqlite"
	DefaultSQLitePath              = "data/jieli.db"
	DefaultAttachmentStorageDriver = "local"
	R2AttachmentStorageDriver      = "r2"
	DefaultAttachmentStorageDir    = "data/attachments"
	DefaultR2Region                = "auto"
	DefaultLocale                  = "en"
	// DevSessionSecret is used only when dev mode is enabled and no explicit
	// session secret is configured. Never use this in production.
	DevSessionSecret = "jieli-dev-session-secret"
	// DefaultAPIKeyEncryptionSecret encrypts stored API keys at rest only when
	// SESSION_SECRET is not configured. It is a public constant, so production
	// deployments MUST set SESSION_SECRET to a private value.
	DefaultAPIKeyEncryptionSecret = "jieli-default-api-key-secret"
)

type Config struct {
	SetupToken     string
	DeploymentMode string
	DevMode        bool
	Server         ServerConfig
	Database       DatabaseConfig
	I18n           I18nConfig
	Security       SecurityConfig
	GitHub         GitHubConfig
	Google         GoogleConfig
	Attachments    AttachmentStorageConfig
}

type ServerConfig struct {
	Host           string
	Port           int
	TrustedProxies []string
}

type DatabaseConfig struct {
	Driver     string
	SQLitePath string
}

type I18nConfig struct {
	DefaultLocale string
}

type SecurityConfig struct {
	SessionSecret string
	CookieSecure  bool
}

type GitHubConfig struct {
	ClientID          string
	ClientSecret      string
	CallbackURL       string
	PostLoginRedirect string
}

type GoogleConfig struct {
	ClientID          string
	ClientSecret      string
	CallbackURL       string
	PostLoginRedirect string
}

type AttachmentStorageConfig struct {
	Driver            string
	LocalDir          string
	R2Endpoint        string
	R2Bucket          string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Region          string
	R2PublicBaseURL   string
}

func DefaultConfig() Config {
	return Config{
		DeploymentMode: "self-hosted",
		Server: ServerConfig{
			Host: DefaultHost,
			Port: DefaultPort,
		},
		Database: DatabaseConfig{
			Driver:     DefaultDBDriver,
			SQLitePath: DefaultSQLitePath,
		},
		I18n: I18nConfig{
			DefaultLocale: DefaultLocale,
		},
		Security: SecurityConfig{},
		GitHub: GitHubConfig{
			PostLoginRedirect: "/web/dashboard",
		},
		Google: GoogleConfig{
			PostLoginRedirect: "/web/dashboard",
		},
		Attachments: AttachmentStorageConfig{
			Driver:   DefaultAttachmentStorageDriver,
			LocalDir: DefaultAttachmentStorageDir,
			R2Region: DefaultR2Region,
		},
	}
}

func ConfigFromEnv() Config {
	cfg := DefaultConfig()
	cfg.SetupToken = os.Getenv("JIELI_SETUP_TOKEN")
	if mode := os.Getenv("JIELI_DEPLOYMENT_MODE"); mode != "" {
		cfg.DeploymentMode = mode
	}
	cfg.DevMode = os.Getenv("DEV") == "1"
	if host := os.Getenv("HOST"); host != "" {
		cfg.Server.Host = host
	}
	if port := os.Getenv("PORT"); port != "" {
		parsed, err := strconv.Atoi(port)
		if err == nil {
			cfg.Server.Port = parsed
		}
	}
	if path := os.Getenv("SQLITE_PATH"); path != "" {
		cfg.Database.SQLitePath = path
	}
	if trustedProxies := parseCSVEnv(os.Getenv("TRUSTED_PROXIES")); len(trustedProxies) > 0 {
		cfg.Server.TrustedProxies = trustedProxies
	}
	if attachmentStorageDir := os.Getenv("ATTACHMENT_STORAGE_DIR"); attachmentStorageDir != "" {
		cfg.Attachments.LocalDir = attachmentStorageDir
	}
	attachmentStorageDriver := strings.TrimSpace(os.Getenv("ATTACHMENT_STORAGE_DRIVER"))
	if attachmentStorageDriver != "" {
		cfg.Attachments.Driver = attachmentStorageDriver
	}
	cfg.Attachments.R2Endpoint = strings.TrimSpace(os.Getenv("R2_ENDPOINT"))
	cfg.Attachments.R2Bucket = strings.TrimSpace(os.Getenv("R2_BUCKET"))
	cfg.Attachments.R2AccessKeyID = strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
	cfg.Attachments.R2SecretAccessKey = strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
	if r2Region := strings.TrimSpace(os.Getenv("R2_REGION")); r2Region != "" {
		cfg.Attachments.R2Region = r2Region
	}
	cfg.Attachments.R2PublicBaseURL = strings.TrimSpace(os.Getenv("R2_PUBLIC_BASE_URL"))
	if attachmentStorageDriver == "" && cfg.Attachments.hasCompleteR2Config() {
		cfg.Attachments.Driver = R2AttachmentStorageDriver
	}
	cfg.GitHub.ClientID = os.Getenv("GITHUB_CLIENT_ID")
	cfg.GitHub.ClientSecret = os.Getenv("GITHUB_CLIENT_SECRET")
	cfg.GitHub.CallbackURL = os.Getenv("GITHUB_CALLBACK_URL")
	if redirect := os.Getenv("GITHUB_POST_LOGIN_REDIRECT"); redirect != "" {
		cfg.GitHub.PostLoginRedirect = redirect
	}
	cfg.Google.ClientID = os.Getenv("GOOGLE_CLIENT_ID")
	cfg.Google.ClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	cfg.Google.CallbackURL = os.Getenv("GOOGLE_CALLBACK_URL")
	if redirect := os.Getenv("GOOGLE_POST_LOGIN_REDIRECT"); redirect != "" {
		cfg.Google.PostLoginRedirect = redirect
	}
	cfg.Security.SessionSecret = os.Getenv("SESSION_SECRET")
	if secure := os.Getenv("COOKIE_SECURE"); secure != "" {
		parsed, err := strconv.ParseBool(secure)
		if err == nil {
			cfg.Security.CookieSecure = parsed
		}
	}
	return cfg
}

func parseCSVEnv(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

// ResolveAPIKeyEncryptionSecret derives the secret used to encrypt stored API
// keys from the session secret, falling back to a default so keys are never
// encrypted with an empty secret when SESSION_SECRET is unset.
func ResolveAPIKeyEncryptionSecret(sessionSecret string) string {
	if sessionSecret == "" {
		return DefaultAPIKeyEncryptionSecret
	}
	return sessionSecret
}

func PrepareRuntime(cfg Config) error {
	if cfg.DeploymentMode != "self-hosted" && cfg.DeploymentMode != "cloud" {
		return errors.New("JIELI_DEPLOYMENT_MODE must be self-hosted or cloud")
	}
	if err := ValidateAttachmentStorage(cfg.Attachments); err != nil {
		return err
	}
	if cfg.Database.Driver == DefaultDBDriver {
		if err := os.MkdirAll(filepath.Dir(cfg.Database.SQLitePath), 0o755); err != nil {
			return err
		}
	}
	if cfg.Attachments.Driver == DefaultAttachmentStorageDriver {
		return os.MkdirAll(cfg.Attachments.LocalDir, 0o755)
	}
	return nil
}

func ValidateAttachmentStorage(cfg AttachmentStorageConfig) error {
	driver := strings.TrimSpace(cfg.Driver)
	if driver == "" {
		driver = DefaultAttachmentStorageDriver
	}
	switch driver {
	case DefaultAttachmentStorageDriver, R2AttachmentStorageDriver:
	default:
		return errors.New("unsupported attachment storage driver")
	}
	if cfg.hasPartialR2Config() {
		return errors.New("partial R2 attachment storage config")
	}
	if driver == R2AttachmentStorageDriver && !cfg.hasCompleteR2Config() {
		return errors.New("R2 attachment storage requires endpoint, bucket, access key ID, and secret access key")
	}
	return nil
}

func (cfg AttachmentStorageConfig) hasAnyR2Config() bool {
	return strings.TrimSpace(cfg.R2Endpoint) != "" ||
		strings.TrimSpace(cfg.R2Bucket) != "" ||
		strings.TrimSpace(cfg.R2AccessKeyID) != "" ||
		strings.TrimSpace(cfg.R2SecretAccessKey) != ""
}

func (cfg AttachmentStorageConfig) hasCompleteR2Config() bool {
	return strings.TrimSpace(cfg.R2Endpoint) != "" &&
		strings.TrimSpace(cfg.R2Bucket) != "" &&
		strings.TrimSpace(cfg.R2AccessKeyID) != "" &&
		strings.TrimSpace(cfg.R2SecretAccessKey) != ""
}

func (cfg AttachmentStorageConfig) hasPartialR2Config() bool {
	return cfg.hasAnyR2Config() && !cfg.hasCompleteR2Config()
}
