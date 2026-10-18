package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/api/route"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	attachmentservice "github.com/jieliapp/jieli/backend/service/attachment"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
	subscriptionservice "github.com/jieliapp/jieli/backend/service/subscription"
	frontendassets "github.com/jieliapp/jieli/frontend"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func versionLine() string {
	fields := []string{"jieli", "version=" + version}
	buildCommit := commit
	buildDate := date
	modified := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			fields[1] = "version=" + info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if buildCommit == "" {
					buildCommit = setting.Value
				}
			case "vcs.time":
				if buildDate == "" {
					buildDate = setting.Value
				}
			case "vcs.modified":
				modified = setting.Value
			}
		}
	}
	if buildCommit != "" {
		fields = append(fields, "commit="+buildCommit)
	}
	if buildDate != "" {
		fields = append(fields, "date="+buildDate)
	}
	if modified != "" {
		fields = append(fields, "modified="+modified)
	}
	return strings.Join(fields, " ")
}

func NewServer(cfg common.Config) *gin.Engine {
	return NewServerWithFrontend(cfg, frontendassets.Dist())
}

func NewServerWithFrontend(cfg common.Config, frontend fs.FS) *gin.Engine {
	if err := attachmentservice.ConfigureStorageFromConfig(cfg.Attachments); err != nil {
		panic(err)
	}
	apiKeyEncryptionSecret := common.ResolveAPIKeyEncryptionSecret(cfg.Security.SessionSecret)
	authservice.SetAPIKeyEncryptionSecret(apiKeyEncryptionSecret)
	router := gin.Default()
	if err := router.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Printf("failed to set trusted proxies: %v", err)
	}
	routeCfg := route.Config{
		SetupToken:             cfg.SetupToken,
		SelfHosted:             cfg.DeploymentMode == "self-hosted",
		DevMode:                cfg.DevMode,
		SessionSecret:          cfg.Security.SessionSecret,
		CookieSecure:           cfg.Security.CookieSecure,
		R2PublicBaseURL:        cfg.Attachments.R2PublicBaseURL,
		APIKeyEncryptionSecret: apiKeyEncryptionSecret,
		GitHub:                 cfg.GitHub,
		Google:                 cfg.Google,
	}
	route.RegisterWithConfig(router, routeCfg)
	serveFrontend(router, frontend, routeCfg)
	return router
}

func serveFrontend(router *gin.Engine, frontend fs.FS, routeCfg route.Config) {
	files := http.FileServer(http.FS(frontend))
	// Shells that hydrate into the SPA and may read window.__APP_CONFIG__:
	// the SPA fallback and both prerendered home variants. Built once at startup.
	spaShell := buildShell(frontend, "spa.html", routeCfg)
	homeEN := buildShell(frontend, "index.html", routeCfg)
	homeZH := buildShell(frontend, "zh.html", routeCfg)
	router.NoRoute(func(c *gin.Context) {
		if route.ServeThreadExport(c, routeCfg) {
			return
		}
		if serveAgentThreadHint(c) {
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/web/api/") {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "not found"})
			return
		}
		cleanPath := path.Clean(c.Request.URL.Path)
		if (cleanPath == "/" || cleanPath == "/zh") && hasSession(c, routeCfg) {
			c.Redirect(http.StatusFound, "/web/dashboard")
			return
		}

		// Home: serve the prerendered language variant chosen by Accept-Language.
		// /zh and other explicit URLs carry their own language and fall through
		// to the prerendered <path>.html lookup below.
		if cleanPath == "/" {
			shell := homeEN
			if homeZH != nil && prefersChinese(c.GetHeader("Accept-Language")) {
				shell = homeZH
			}
			if shell == nil {
				shell = spaShell
			}
			if shell == nil {
				c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "frontend shell is missing"})
				return
			}
			c.Header("Vary", "Accept-Language")
			c.Data(http.StatusOK, "text/html; charset=utf-8", shell)
			return
		}

		assetPath := strings.TrimPrefix(cleanPath, "/")
		if assetPath != "." && assetPath != "" {
			if path.Ext(assetPath) == "" {
				// Clean route with no extension -> prerendered <path>.html
				// (e.g. /pricing -> pricing.html, /zh/manual -> zh/manual.html).
				if html, err := fs.ReadFile(frontend, assetPath+".html"); err == nil {
					c.Data(http.StatusOK, "text/html; charset=utf-8", html)
					return
				}
			} else if file, err := frontend.Open(assetPath); err == nil {
				_ = file.Close()
				if cacheControl := frontendAssetCacheControl(c.Request.URL.Path); cacheControl != "" {
					c.Header("Cache-Control", cacheControl)
				}
				files.ServeHTTP(c.Writer, c.Request)
				return
			} else if strings.HasPrefix(c.Request.URL.Path, "/_app/") {
				c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "frontend asset not found"})
				return
			}
		}
		if spaShell == nil {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "frontend shell is missing"})
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", spaShell)
	})
}

func hasSession(c *gin.Context, routeCfg route.Config) bool {
	if routeCfg.SessionSecret == "" {
		return false
	}
	_, ok := middleware.ResolveSessionUser(c, routeCfg.SessionSecret)
	return ok
}

// prefersChinese reports whether the Accept-Language header's primary language
// is Chinese, used to pick the zh home variant for the unprefixed "/" URL.
func prefersChinese(acceptLanguage string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(acceptLanguage)), "zh")
}

// buildShell reads an HTML shell from the embedded frontend and injects the
// public runtime config as window.__APP_CONFIG__ so the SPA can render the
// sign-in buttons without an extra /web/api/config round-trip. Returns nil when
// the file is missing.
func buildShell(frontend fs.FS, name string, routeCfg route.Config) []byte {
	html, err := fs.ReadFile(frontend, name)
	if err != nil {
		return nil
	}
	payload, err := json.Marshal(route.PublicRuntimeConfig(routeCfg))
	if err != nil {
		return html
	}
	snippet := "<script>window.__APP_CONFIG__=" + string(payload) + "</script>"
	if idx := strings.Index(string(html), "</head>"); idx != -1 {
		return []byte(string(html)[:idx] + snippet + string(html)[idx:])
	}
	return append([]byte(snippet), html...)
}

func frontendAssetCacheControl(rawPath string) string {
	if strings.HasPrefix(rawPath, "/_app/immutable/") {
		return "public, max-age=31536000, immutable"
	}
	if strings.HasPrefix(rawPath, "/brand/") {
		return "public, max-age=86400"
	}
	return ""
}

func serveAgentThreadHint(c *gin.Context) bool {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		return false
	}
	providerThreadID, ok := plainThreadPath(c.Request.URL.Path)
	if !ok || wantsBrowserHTML(c.Request) {
		return false
	}
	message := "This is a Jieli thread page served by a SPA. Agents should not use WebFetch or curl on /threads/" + providerThreadID + ". If the Jieli plugin is installed, use the jieli skill. The skill reads /threads/" + providerThreadID + ".md with Authorization: Bearer $JIELI_API_KEY."
	c.Data(http.StatusUnauthorized, "text/plain; charset=utf-8", []byte(message+"\n"))
	return true
}

func plainThreadPath(rawPath string) (string, bool) {
	value := strings.TrimPrefix(rawPath, "/threads/")
	if value == rawPath || value == "" || strings.Contains(value, "/") || strings.Contains(value, ".") {
		return "", false
	}
	if !strings.HasPrefix(value, "T-") {
		return "", false
	}
	return value, true
}

func wantsBrowserHTML(r *http.Request) bool {
	userAgent := strings.ToLower(r.UserAgent())
	if strings.Contains(userAgent, "curl/") ||
		strings.Contains(userAgent, "wget/") ||
		strings.Contains(userAgent, "python-requests") ||
		strings.Contains(userAgent, "go-http-client") ||
		strings.Contains(userAgent, "node-fetch") ||
		strings.Contains(userAgent, "undici") {
		return false
	}
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "text/html")
}

func startSubscriptionSyncScheduler(ctx context.Context, cfg common.Config, apiKeyEncryptionSecret string) func() {
	return subscriptionservice.StartPeriodicSync(ctx, 15*time.Minute, subscriptionservice.Service{}.ExpireDue, log.Printf)
}

func main() {
	devFlag := flag.Bool("dev", false, "enable dev mode (auto-login dev user, dev session secret)")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(versionLine())
		return
	}

	cfg := common.ConfigFromEnv()
	cfg.DevMode = cfg.DevMode || *devFlag
	if cfg.DevMode && cfg.Security.SessionSecret == "" {
		cfg.Security.SessionSecret = common.DevSessionSecret
	}

	if err := common.PrepareRuntime(cfg); err != nil {
		panic(err)
	}
	if err := model.InitSQLite(cfg.Database.SQLitePath); err != nil {
		panic(err)
	}
	defer func() {
		if err := model.CloseDB(); err != nil {
			panic(err)
		}
	}()

	if err := route.PrepareCommercial(cfg.DeploymentMode); err != nil {
		panic(err)
	}

	if cfg.DevMode {
		fmt.Println("[jieli] dev mode enabled — visit /api/auth/dev-login to sign in as the local dev user")
	}

	schedulerCtx, stopSchedulerCtx := context.WithCancel(context.Background())
	defer stopSchedulerCtx()
	stopSubscriptionSync := startSubscriptionSyncScheduler(schedulerCtx, cfg, common.ResolveAPIKeyEncryptionSecret(cfg.Security.SessionSecret))
	defer stopSubscriptionSync()

	server := NewServer(cfg)
	if err := server.Run(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)); err != nil {
		panic(err)
	}
}
