package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
	threadservice "github.com/jieliapp/jieli/backend/service/thread"
)

func TestNewRouterShouldExposeHealthRoute(t *testing.T) {
	server := NewServer(common.DefaultConfig())

	routes := server.Routes()
	for _, route := range routes {
		if route.Method == "GET" && route.Path == "/web/api/health" {
			return
		}
	}
	t.Fatalf("expected GET /web/api/health to be registered")
}

func TestNewServerShouldUseConfiguredTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := common.DefaultConfig()
	cfg.Server.TrustedProxies = []string{"127.0.0.1/32"}
	server := NewServerWithFrontend(cfg, testFrontendAssets())
	server.GET("/test-client-ip", func(c *gin.Context) {
		c.String(http.StatusOK, c.ClientIP())
	})

	request := httptest.NewRequest(http.MethodGet, "/test-client-ip", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("X-Forwarded-For", "203.0.113.42, 127.0.0.1")
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != "203.0.113.42" {
		t.Fatalf("expected forwarded client IP, got %q", body)
	}
}

func TestNewServerShouldServeFrontendShellForBrowserRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	// Root serves the prerendered English home; dynamic routes fall back to the
	// SPA shell. Both carry the injected window.__APP_CONFIG__.
	cases := map[string]string{
		"/":            "EN HOME",
		"/threads/123": "SPA SHELL",
	}
	for path, want := range cases {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Accept", "text/html")
		server.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d body %s", path, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), want) {
			t.Fatalf("expected %s body to contain %q, got %q", path, want, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "window.__APP_CONFIG__") {
			t.Fatalf("expected %s to carry injected app config, got %q", path, recorder.Body.String())
		}
	}
}

func TestNewServerShouldGuideAgentFetchesForPlainThreadRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-519effc2-9212-4aa1-830b-36df206f03d2", nil)
	request.Header.Set("User-Agent", "curl/8.7.1")
	request.Header.Set("Accept", "*/*")
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected agent hint 401, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "SPA SHELL") {
		t.Fatalf("expected agent hint, got frontend shell")
	}
	if !strings.Contains(recorder.Body.String(), "use the jieli skill") || !strings.Contains(recorder.Body.String(), "/threads/T-519effc2-9212-4aa1-830b-36df206f03d2.md") {
		t.Fatalf("expected jieli skill hint, got %s", recorder.Body.String())
	}
}

func TestNewServerShouldServeFrontendShellForBrowserThreadRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-519effc2-9212-4aa1-830b-36df206f03d2", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0")
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected frontend shell for browser thread route, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "SPA SHELL") {
		t.Fatalf("expected SPA shell, got %q", recorder.Body.String())
	}
}

func TestNewServerShouldNotFallbackAPI404ToFrontendShell(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/missing", nil)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected API 404, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "SPA SHELL") {
		t.Fatalf("expected API 404 not to serve frontend shell")
	}
}

func TestNewServerShouldServeFrontendStaticAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/_app/immutable/entry/app.js", nil)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected asset response, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != "console.log('jieli')" {
		t.Fatalf("expected asset body, got %q", recorder.Body.String())
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable asset cache control, got %q", cacheControl)
	}
}

func TestNewServerShouldNotCacheFrontendShell(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-browser-route", nil)
	request.Header.Set("Accept", "text/html")
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected frontend shell, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if cacheControl := recorder.Header().Get("Cache-Control"); cacheControl != "" {
		t.Fatalf("expected frontend shell without cache control, got %q", cacheControl)
	}
}

func TestNewServerShouldRenderThreadJSONExportForAPIKeyUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	credential, err := authservice.ProvisionUser(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(user, threadservice.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-export-one","title":"Export me","messages":[{"role":"user","content":"please export this thread"}]}`),
		Repo:     "owner/repo",
		Labels:   []string{"export"},
	}); err != nil {
		t.Fatalf("upload thread: %v", err)
	}

	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-export-one.json", nil)
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("expected application/json content type, got %q", contentType)
	}
	if disposition := recorder.Header().Get("Content-Disposition"); disposition != "" {
		t.Fatalf("expected browser-rendered response without content disposition, got %q", disposition)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode export response: %v", err)
	}
	if body["provider_thread_id"] != "T-export-one" || body["title"] != "Export me" {
		t.Fatalf("expected exported thread detail, got %#v", body)
	}
}

func TestNewServerShouldRenderThreadMarkdownExportForAPIKeyUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	credential, err := authservice.ProvisionUser(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(user, threadservice.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-export-md","title":"Markdown export","messages":[{"role":"user","content":"please export markdown"}]}`),
		Repo:     "owner/repo",
	}); err != nil {
		t.Fatalf("upload thread: %v", err)
	}

	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-export-md.md", nil)
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/markdown") {
		t.Fatalf("expected text/markdown content type, got %q", contentType)
	}
	if disposition := recorder.Header().Get("Content-Disposition"); disposition != "" {
		t.Fatalf("expected browser-rendered response without content disposition, got %q", disposition)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "# Markdown export") || !strings.Contains(body, "## user") || !strings.Contains(body, "please export markdown") {
		t.Fatalf("expected markdown export body, got %q", body)
	}
}

func TestNewServerShouldRenderThreadMarkdownExportWithTruncatedToolResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	credential, err := authservice.ProvisionUser(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	longToolResult := strings.Repeat("tool-result-body-", 1000)
	threadPayload, err := json.Marshal(map[string]any{
		"id":    "T-api-export-md",
		"title": "API markdown export",
		"messages": []map[string]any{
			{"role": "assistant", "content": []map[string]any{{"type": "tool_result", "content": longToolResult}}},
		},
	})
	if err != nil {
		t.Fatalf("marshal thread: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(user, threadservice.UploadPayload{Provider: model.ProviderCodex, Thread: threadPayload}); err != nil {
		t.Fatalf("upload thread: %v", err)
	}

	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-api-export-md.md?truncate_tool_results=1", nil)
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/markdown") {
		t.Fatalf("expected text/markdown content type, got %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "# API markdown export") || !strings.Contains(body, "## assistant") {
		t.Fatalf("expected markdown thread export, got %q", body)
	}
	if strings.Contains(body, longToolResult) || !strings.Contains(body, "[tool result truncated]") {
		t.Fatalf("expected truncated tool result, got body length %d", len(body))
	}
}

func TestNewServerShouldRenderThreadExportForSessionUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(user, threadservice.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-session-export","title":"Session export","messages":[{"role":"user","content":"browser export"}]}`),
	}); err != nil {
		t.Fatalf("upload thread: %v", err)
	}
	cfg := common.DefaultConfig()
	cfg.Security.SessionSecret = "server-test-secret"
	cookie, err := middleware.NewSessionCookie(user.ID, cfg.Security.SessionSecret)
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	server := NewServerWithFrontend(cfg, testFrontendAssets())
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-session-export.md", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "# Session export") {
		t.Fatalf("expected session export markdown, got %q", recorder.Body.String())
	}
}

func TestNewServerShouldRejectAnonymousThreadExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-missing.md", nil)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected anonymous export status 401, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestNewServerShouldHideOtherUsersThreadExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-test-key"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	bobCredential, err := authservice.ProvisionUser(bob)
	if err != nil {
		t.Fatalf("rotate bob api key: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(alice, threadservice.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-alice-private","title":"Alice private","messages":[{"role":"user","content":"secret"}]}`),
	}); err != nil {
		t.Fatalf("upload alice thread: %v", err)
	}
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/threads/T-alice-private.json", nil)
	request.Header.Set("Authorization", "Bearer "+bobCredential.Plaintext)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected other user's export status 404, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestNewServerShouldReturn404ForMissingFrontendAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/_app/immutable/missing.js", nil)
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing frontend asset to return 404, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() == "<!doctype html><title>Jieli</title>" {
		t.Fatalf("expected missing frontend asset not to serve frontend shell")
	}
}

func TestNewServerShouldUseConfiguredSessionSecretForAuthRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	cfg := common.DefaultConfig()
	cfg.Security.SessionSecret = "server-test-secret"
	cookie, err := middleware.NewSessionCookie(user.ID, cfg.Security.SessionSecret)
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	server := NewServer(cfg)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected configured session secret to authenticate, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestNewServerShouldUseConfiguredSecureCookieFlagForGitHubOAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := common.DefaultConfig()
	cfg.Security.CookieSecure = true
	cfg.GitHub.ClientID = "github-client-id"
	cfg.GitHub.CallbackURL = "https://jieli.example.com/web/api/auth/callback/github"

	server := NewServer(cfg)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/github", nil)
	server.ServeHTTP(recorder, request)

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "jieli_oauth_state" && cookie.Secure {
			return
		}
	}
	t.Fatalf("expected GitHub OAuth state cookie to use configured Secure flag")
}

func TestNewServerShouldUseConfiguredSecureCookieFlagForGoogleOAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := common.DefaultConfig()
	cfg.Security.CookieSecure = true
	cfg.Google.ClientID = "google-client-id"
	cfg.Google.CallbackURL = "https://jieli.example.com/web/api/auth/callback/google"

	server := NewServer(cfg)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/google", nil)
	server.ServeHTTP(recorder, request)

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "jieli_oauth_state_google" && cookie.Secure {
			return
		}
	}
	t.Fatalf("expected Google OAuth state cookie to use configured Secure flag")
}

func closeModelDB(t *testing.T) func() {
	t.Helper()
	return func() {
		if err := model.CloseDB(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}

func TestNewServerShouldServeChineseHomeForChineseAcceptLanguage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept", "text/html")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "ZH HOME") {
		t.Fatalf("expected Chinese home, got %q", recorder.Body.String())
	}
	if vary := recorder.Header().Get("Vary"); vary != "Accept-Language" {
		t.Fatalf("expected Vary: Accept-Language, got %q", vary)
	}
}

func TestNewServerShouldServeEnglishHomeByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept", "text/html")
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "EN HOME") {
		t.Fatalf("expected English home, got %q", recorder.Body.String())
	}
	if vary := recorder.Header().Get("Vary"); vary != "Accept-Language" {
		t.Fatalf("expected Vary: Accept-Language, got %q", vary)
	}
}

func TestNewServerShouldServePrerenderedMarketingPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	// Clean route /pricing maps to the prerendered pricing.html, not the SPA shell.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/pricing", nil)
	request.Header.Set("Accept", "text/html")
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "PRICING EN") {
		t.Fatalf("expected prerendered pricing page, got %q", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "SPA SHELL") {
		t.Fatalf("expected prerendered page, got SPA shell")
	}
}

func TestNewServerShouldServePrerenderedChineseSubPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	// /zh/pricing maps to the nested prerendered zh/pricing.html.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/zh/pricing", nil)
	request.Header.Set("Accept", "text/html")
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "PRICING ZH") {
		t.Fatalf("expected prerendered Chinese pricing page, got %q", recorder.Body.String())
	}
}

func TestNewServerShouldRedirectSignedInHomeToDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	cfg := common.DefaultConfig()
	cfg.Security.SessionSecret = "server-test-secret"
	cookie, err := middleware.NewSessionCookie(user.ID, cfg.Security.SessionSecret)
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	server := NewServerWithFrontend(cfg, testFrontendAssets())

	for _, path := range []string{"/", "/zh"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
		request.Header.Set("Accept", "text/html")
		server.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusFound {
			t.Fatalf("expected signed-in %s to redirect, got %d body %s", path, recorder.Code, recorder.Body.String())
		}
		if location := recorder.Header().Get("Location"); location != "/web/dashboard" {
			t.Fatalf("expected signed-in %s to redirect to /web/dashboard, got %q", path, location)
		}
	}
}

func TestNewServerShouldFallBackToSPAForUnknownCleanRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := NewServerWithFrontend(common.DefaultConfig(), testFrontendAssets())

	// A clean route with no prerendered HTML (e.g. an authed SPA page) falls
	// back to the SPA shell so client-side routing can take over.
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/settings", nil)
	request.Header.Set("Accept", "text/html")
	server.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "SPA SHELL") {
		t.Fatalf("expected SPA shell, got %q", recorder.Body.String())
	}
}

func testFrontendAssets() fs.FS {
	return fstest.MapFS{
		"index.html":                  {Data: []byte("<!doctype html><head></head><body>EN HOME</body>")},
		"zh.html":                     {Data: []byte("<!doctype html><head></head><body>ZH HOME</body>")},
		"spa.html":                    {Data: []byte("<!doctype html><head></head><body>SPA SHELL</body>")},
		"pricing.html":                {Data: []byte("<!doctype html><head></head><body>PRICING EN</body>")},
		"zh/pricing.html":             {Data: []byte("<!doctype html><head></head><body>PRICING ZH</body>")},
		"_app/immutable/entry/app.js": {Data: []byte("console.log('jieli')")},
	}
}
