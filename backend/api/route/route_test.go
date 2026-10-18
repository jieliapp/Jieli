package route_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/burugo/thing"
	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/api/route"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	attachmentservice "github.com/jieliapp/jieli/backend/service/attachment"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
	threadservice "github.com/jieliapp/jieli/backend/service/thread"
)

var tinyPNGBytes = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00}

func TestRegisterShouldExposeHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.Register(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/health", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["success"] != true {
		t.Fatalf("expected success true, got %#v", body["success"])
	}
	if body["message"] != "" {
		t.Fatalf("expected empty message, got %#v", body["message"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	if data["status"] != "ok" {
		t.Fatalf("expected health status ok, got %#v", data["status"])
	}
}

func TestAttachmentUploadShouldReturnAbsoluteURLAndServeImageBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	attachmentservice.ConfigureStorage(attachmentservice.NewLocalStorage(t.TempDir()))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	settings, err := thing.Use[*model.SiteSettings]()
	if err != nil {
		t.Fatalf("site settings store: %v", err)
	}
	if err = settings.Save(&model.SiteSettings{Name: model.SiteSettingPublicBaseURL, Value: "https://saved.example.com", ValueType: "string"}); err != nil {
		t.Fatalf("save site settings: %v", err)
	}
	credential, err := authservice.RotateAPIKey(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	recorder := httptest.NewRecorder()
	payload := `{"data":"` + base64.StdEncoding.EncodeToString(tinyPNGBytes) + `","mediaType":"image/png"}`
	request := httptest.NewRequest(http.MethodPost, "/plugin/attachments", bytes.NewBufferString(payload))
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected upload status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if _, ok := body["success"]; ok {
		t.Fatalf("expected bare upload response without common envelope, got %#v", body)
	}
	rawURL, ok := body["url"].(string)
	if !ok || !strings.HasPrefix(rawURL, "https://saved.example.com/attachments/") || !strings.HasSuffix(rawURL, ".png") {
		t.Fatalf("expected absolute attachment URL, got %#v", body["url"])
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse attachment URL: %v", err)
	}

	imageRecorder := httptest.NewRecorder()
	imageRequest := httptest.NewRequest(http.MethodGet, parsed.Path, nil)
	router.ServeHTTP(imageRecorder, imageRequest)

	if imageRecorder.Code != http.StatusOK {
		t.Fatalf("expected image status 200, got %d body %s", imageRecorder.Code, imageRecorder.Body.String())
	}
	if imageRecorder.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("expected image/png content type, got %q", imageRecorder.Header().Get("Content-Type"))
	}
	if !bytes.Equal(imageRecorder.Body.Bytes(), tinyPNGBytes) {
		t.Fatalf("expected original image bytes, got %#v", imageRecorder.Body.Bytes())
	}
}

func TestAttachmentUploadShouldRateLimitPerUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	attachmentservice.ConfigureStorage(attachmentservice.NewLocalStorage(t.TempDir()))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	credential, err := authservice.RotateAPIKey(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	payload := `{"data":"` + base64.StdEncoding.EncodeToString(tinyPNGBytes) + `","mediaType":"image/png"}`

	upload := func() int {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/plugin/attachments", bytes.NewBufferString(payload))
		request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
		router.ServeHTTP(recorder, request)
		return recorder.Code
	}

	// attachmentLimiter allows 60 uploads per user per minute.
	for i := 0; i < 60; i++ {
		if code := upload(); code != http.StatusOK {
			t.Fatalf("expected upload %d to succeed with 200, got %d", i+1, code)
		}
	}
	if code := upload(); code != http.StatusTooManyRequests {
		t.Fatalf("expected 61st upload to be rate limited with 429, got %d", code)
	}
}

func TestPluginAttachmentUploadShouldReturnAbsoluteURLAndServeImageBytes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	attachmentservice.ConfigureStorage(attachmentservice.NewLocalStorage(t.TempDir()))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	settings, err := thing.Use[*model.SiteSettings]()
	if err != nil {
		t.Fatalf("site settings store: %v", err)
	}
	if err = settings.Save(&model.SiteSettings{Name: model.SiteSettingPublicBaseURL, Value: "https://jieli.example.test", ValueType: "string"}); err != nil {
		t.Fatalf("save site URL: %v", err)
	}
	credential, err := authservice.RotateAPIKey(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	recorder := httptest.NewRecorder()
	payload := `{"data":"` + base64.StdEncoding.EncodeToString(tinyPNGBytes) + `","mediaType":"image/png"}`
	request := httptest.NewRequest(http.MethodPost, "/plugin/attachments", bytes.NewBufferString(payload))
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected upload status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	rawURL, ok := body["url"].(string)
	if !ok || !strings.HasPrefix(rawURL, "https://jieli.example.test/attachments/") || !strings.HasSuffix(rawURL, ".png") {
		t.Fatalf("expected absolute attachment URL, got %#v", body["url"])
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse attachment URL: %v", err)
	}

	imageRecorder := httptest.NewRecorder()
	imageRequest := httptest.NewRequest(http.MethodGet, parsed.Path, nil)
	router.ServeHTTP(imageRecorder, imageRequest)

	if imageRecorder.Code != http.StatusOK {
		t.Fatalf("expected image status 200, got %d body %s", imageRecorder.Code, imageRecorder.Body.String())
	}
	if !bytes.Equal(imageRecorder.Body.Bytes(), tinyPNGBytes) {
		t.Fatalf("expected original image bytes, got %#v", imageRecorder.Body.Bytes())
	}
}

func TestAttachmentUploadShouldReturnR2PublicURLWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	attachmentservice.ConfigureStorage(newMemoryStorage("r2"))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	credential, err := authservice.RotateAPIKey(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{R2PublicBaseURL: "https://cdn.jieli.app"})
	recorder := httptest.NewRecorder()
	payload := `{"data":"` + base64.StdEncoding.EncodeToString(tinyPNGBytes) + `","mediaType":"image/png"}`
	request := httptest.NewRequest(http.MethodPost, "/plugin/attachments", bytes.NewBufferString(payload))
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected upload status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	rawURL, ok := body["url"].(string)
	if !ok || !strings.HasPrefix(rawURL, "https://cdn.jieli.app/") || !strings.HasSuffix(rawURL, ".png") || strings.Contains(rawURL, "/attachments/") {
		t.Fatalf("expected R2 CDN attachment URL, got %#v", body["url"])
	}
}

func TestAuthMeShouldReturnCurrentSessionUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(user.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/me", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected user data, got %#v", body["data"])
	}
	if data["username"] != "alice" {
		t.Fatalf("expected username alice, got %#v", data["username"])
	}
}

func TestAuthLogoutShouldClearSessionCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/web/api/auth/logout", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	for _, cookie := range cookies {
		if cookie.Name == middleware.SessionCookieName && cookie.MaxAge < 0 {
			return
		}
	}
	t.Fatalf("expected logout to clear %s cookie", middleware.SessionCookieName)
}

func TestUserAPIKeyShouldReturnPrefixStatusForCurrentSessionUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	const encryptionSecret = "test-encryption-secret"
	encrypted, err := common.EncryptSecret("jieli_public_secret", encryptionSecret)
	if err != nil {
		t.Fatalf("encrypt api key: %v", err)
	}
	user := &model.User{
		Username:        "alice",
		GitHubID:        "github-alice",
		APIKeyHash:      "stored-hash",
		APIKeyPrefix:    "jieli_public",
		APIKeyEncrypted: encrypted,
	}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(user.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret", APIKeyEncryptionSecret: encryptionSecret})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/user/api-key", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode api key response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	if data["prefix"] != "jieli_public" {
		t.Fatalf("expected api key prefix, got %#v", data["prefix"])
	}
	if data["has_key"] != true {
		t.Fatalf("expected has_key true, got %#v", data["has_key"])
	}
	if data["api_key"] != "jieli_public_secret" {
		t.Fatalf("expected decrypted plaintext api key, got %#v", data["api_key"])
	}
	if _, ok := data["hash"]; ok {
		t.Fatalf("expected api key status not to expose hash")
	}
}

func TestUserAPIKeyShouldRejectAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/user/api-key", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode unauthorized response: %v", err)
	}
	if body["success"] != false {
		t.Fatalf("expected success false, got %#v", body["success"])
	}
}

func TestUserProfileShouldReturnCurrentSessionUserProfile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", Email: "alice@example.com", AvatarURL: "https://example.com/avatar.png", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix", PreferredLocale: "zh"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	aliceKey, err := authservice.RotateAPIKey(alice)
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	uploadThread(t, router, aliceKey.Plaintext, "T-alpha", "OAuth fix", "please fix oauth", "owner/repo", "auth")
	alice, err = model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("reload alice: %v", err)
	}
	if err := alice.SetUsageInfo(model.UserUsageInfo{LastUploadError: &model.UserUsageUploadError{
		Code:              "message_quota_exceeded",
		Provider:          model.ProviderCodex,
		ProviderThreadID:  "T-over-quota",
		CurrentMessages:   29999,
		IncomingMessages:  2,
		ProjectedMessages: 30001,
		Quota:             30000,
	}}); err != nil {
		t.Fatalf("set usage info: %v", err)
	}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save usage info: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/user/profile", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected profile data object, got %#v", body["data"])
	}
	userData, ok := data["user"].(map[string]any)
	if !ok || userData["username"] != "alice" || userData["display_name"] != "Alice Doe" || userData["preferred_locale"] != "zh" {
		t.Fatalf("expected current user profile data, got %#v", data["user"])
	}
	stats, ok := data["stats"].(map[string]any)
	if !ok || stats["total_threads"] != float64(1) || stats["total_messages"] != float64(1) {
		t.Fatalf("expected profile stats, got %#v", data["stats"])
	}
	if data["plan_level"] != model.PlanLevelFree {
		t.Fatalf("expected profile plan_level %q, got %#v", model.PlanLevelFree, data["plan_level"])
	}
	if _, ok := userData["api_key_hash"]; ok {
		t.Fatalf("expected profile not to expose api key hash")
	}
	usageInfo, ok := data["usage_info"].(map[string]any)
	if !ok {
		t.Fatalf("expected profile usage_info, got %#v", data["usage_info"])
	}
	lastUploadError, ok := usageInfo["last_upload_error"].(map[string]any)
	if !ok || lastUploadError["code"] != "message_quota_exceeded" || lastUploadError["provider_thread_id"] != "T-over-quota" || lastUploadError["projected_messages"] != float64(30001) {
		t.Fatalf("expected profile usage_info last upload error, got %#v", usageInfo["last_upload_error"])
	}
}

func TestPublicUserProfileShouldReturnPublicSafeProfileWithoutSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", Email: "alice@example.com", AvatarURL: "https://example.com/avatar.png", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(alice, threadservice.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix","messages":[{"role":"user","content":"one"}]}`)}); err != nil {
		t.Fatalf("upload alice thread: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/users/alice/profile", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode public profile response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected profile data object, got %#v", body["data"])
	}
	userData, ok := data["user"].(map[string]any)
	if !ok || userData["username"] != "alice" || userData["display_name"] != "Alice Doe" || userData["avatar_url"] != "https://example.com/avatar.png" {
		t.Fatalf("expected public user data, got %#v", data["user"])
	}
	if _, ok := userData["email"]; ok {
		t.Fatalf("expected public profile not to expose email")
	}
	if _, ok := userData["github_id"]; ok {
		t.Fatalf("expected public profile not to expose github_id")
	}
	if _, ok := userData["api_key_hash"]; ok {
		t.Fatalf("expected public profile not to expose api key hash")
	}
	stats, ok := data["stats"].(map[string]any)
	if !ok || stats["total_threads"] != float64(1) || stats["total_messages"] != float64(1) {
		t.Fatalf("expected public profile stats, got %#v", data["stats"])
	}
}

func TestPublicUserEndpointsShouldResolveStoredMixedCaseUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "sandboxQtJcmGds", APIKeyHash: "mixed-case-key"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: user.ID, Date: "2026-10-09", ThreadCount: 2, MessageCount: 5}); err != nil {
		t.Fatalf("save activity: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	for _, endpoint := range []string{"profile", "activity"} {
		t.Run(endpoint, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/web/api/users/sandboxQtJcmGds/"+endpoint, nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
			}
			var body struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if endpoint == "profile" {
				var profile struct {
					User struct {
						Username string `json:"username"`
					} `json:"user"`
				}
				if err := json.Unmarshal(body.Data, &profile); err != nil || profile.User.Username != user.Username {
					t.Fatalf("expected stored username, got %s: %v", body.Data, err)
				}
			} else {
				var activity []struct {
					MessageCount int `json:"message_count"`
				}
				if err := json.Unmarshal(body.Data, &activity); err != nil || len(activity) != 1 || activity[0].MessageCount != 5 {
					t.Fatalf("expected user's activity, got %s: %v", body.Data, err)
				}
			}
		})
	}
}

func TestPublicUserProfileShouldReturnNotFoundForMissingUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/users/missing/profile", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestPublicUserActivityShouldReturnNamedUserActivityWithoutSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: alice.ID, Date: "2026-05-29", ThreadCount: 2, MessageCount: 5}); err != nil {
		t.Fatalf("save alice activity: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: bob.ID, Date: "2026-05-30", ThreadCount: 9, MessageCount: 99}); err != nil {
		t.Fatalf("save bob activity: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/users/alice/activity", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data []struct {
			Date         string `json:"date"`
			MessageCount int    `json:"message_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode public activity response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Date != "2026-05-29" || body.Data[0].MessageCount != 5 {
		t.Fatalf("expected alice activity only, got %s", recorder.Body.String())
	}
}

func TestUserProfileUpdateShouldPersistCurrentSessionUserFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", Email: "alice@example.com", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/web/api/user/profile", bytes.NewBufferString(`{"username":"alice2","display_name":"Alice Dev"}`))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile update response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["username"] != "alice2" || data["display_name"] != "Alice Dev" || data["email"] != "alice@example.com" {
		t.Fatalf("expected updated safe user data, got %#v", body["data"])
	}
	if _, ok := data["api_key_hash"]; ok {
		t.Fatalf("expected profile update response not to expose api key hash")
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.Username != "alice2" || stored.DisplayName != "Alice Dev" {
		t.Fatalf("expected stored profile fields, got %#v", stored)
	}
}

func TestUserAutoCleanupShouldPersistPreference(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", Email: "alice@example.com", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/web/api/user/auto-cleanup", bytes.NewBufferString(`{"enabled":true}`))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode auto-cleanup response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["auto_cleanup_enabled"] != true {
		t.Fatalf("expected auto_cleanup_enabled true, got %#v", body["data"])
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if !stored.AutoCleanupEnabled {
		t.Fatalf("expected stored auto cleanup enabled")
	}
}

func TestUserAutoCleanupShouldRejectAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/web/api/user/auto-cleanup", bytes.NewBufferString(`{"enabled":true}`))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestUserRestoreThreadsShouldRequireUnlimitedPlanAndRestoreSoftDeletedThreads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix", PlanLevel: model.PlanLevelFree}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	stored, err := threadservice.UploadThreadSnapshot(alice, threadservice.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix","messages":[{"role":"user","content":"please fix oauth"}]}`)})
	if err != nil {
		t.Fatalf("upload alice thread: %v", err)
	}
	if err := threadservice.SoftDeleteThread(alice, stored.ID); err != nil {
		t.Fatalf("soft delete thread: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	forbidden := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(http.MethodPost, "/web/api/user/restore-threads", nil)
	forbiddenRequest.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(forbidden, forbiddenRequest)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("expected free user restore status 403, got %d body %s", forbidden.Code, forbidden.Body.String())
	}

	alice.PlanLevel = model.PlanLevelPro
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("upgrade alice: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/web/api/user/restore-threads", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected pro restore status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode restore response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["restored"] != float64(1) {
		t.Fatalf("expected restored count 1, got %#v", body["data"])
	}
	if _, err := model.ThreadDB.ByID(stored.ID); err != nil {
		t.Fatalf("expected restored thread active: %v", err)
	}
}

func TestUserProfileUpdateShouldRejectAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/web/api/user/profile", bytes.NewBufferString(`{"username":"alice.dev","display_name":"Alice Dev"}`))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestUserProfileUpdateShouldRejectTakenUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", DisplayName: "Bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(bob.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/web/api/user/profile", bytes.NewBufferString(`{"username":"alice","display_name":"Bob Updated"}`))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d body %s", recorder.Code, recorder.Body.String())
	}
	stored, err := model.UserDB.ByID(bob.ID)
	if err != nil {
		t.Fatalf("load stored bob: %v", err)
	}
	if stored.Username != "bob" || stored.DisplayName != "Bob" {
		t.Fatalf("expected stored profile fields unchanged, got %#v", stored)
	}
}

func TestUserProfileUpdateShouldTreatReservedUsernameAsTaken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", GitHubID: "github-alice"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/web/api/user/profile", bytes.NewBufferString(`{"username":"cli","display_name":"Alice Dev"}`))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode profile update response: %v", err)
	}
	if body["message"] != "username unavailable" {
		t.Fatalf("expected username unavailable message, got %#v", body["message"])
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.Username != "alice" || stored.DisplayName != "Alice Doe" {
		t.Fatalf("expected stored profile fields unchanged, got %#v", stored)
	}
}

func TestUserProfileUpdateShouldRejectInvalidUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", GitHubID: "github-alice"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/web/api/user/profile", bytes.NewBufferString(`{"username":"bad name","display_name":"Alice Dev"}`))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body %s", recorder.Code, recorder.Body.String())
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.Username != "alice" || stored.DisplayName != "Alice Doe" {
		t.Fatalf("expected stored profile fields unchanged, got %#v", stored)
	}
}

func TestUserAvatarUploadShouldPersistCurrentSessionUserAvatar(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	attachmentservice.ConfigureStorage(attachmentservice.NewLocalStorage(t.TempDir()))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	settings, err := thing.Use[*model.SiteSettings]()
	if err != nil {
		t.Fatalf("site settings store: %v", err)
	}
	if err = settings.Save(&model.SiteSettings{Name: model.SiteSettingPublicBaseURL, Value: "https://saved-avatar.example", ValueType: "string"}); err != nil {
		t.Fatalf("save site settings: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	payload := `{"data":"` + base64.StdEncoding.EncodeToString(tinyPNGBytes) + `","mediaType":"image/png"}`
	request := httptest.NewRequest(http.MethodPost, "/web/api/user/avatar", bytes.NewBufferString(payload))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode avatar upload response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected avatar user data, got %#v", body["data"])
	}
	avatarURL, ok := data["avatar_url"].(string)
	if !ok || !strings.HasPrefix(avatarURL, "https://saved-avatar.example/attachments/") || !strings.HasSuffix(avatarURL, ".png") {
		t.Fatalf("expected public avatar attachment URL, got %#v", data["avatar_url"])
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.AvatarURL != avatarURL {
		t.Fatalf("expected stored avatar URL %q, got %q", avatarURL, stored.AvatarURL)
	}
}

func TestUserAvatarUploadShouldRejectAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	attachmentservice.ConfigureStorage(attachmentservice.NewLocalStorage(t.TempDir()))
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	payload := `{"data":"` + base64.StdEncoding.EncodeToString(tinyPNGBytes) + `","mediaType":"image/png"}`
	request := httptest.NewRequest(http.MethodPost, "/web/api/user/avatar", bytes.NewBufferString(payload))
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestUserAvatarUploadShouldRejectUnsupportedMediaType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	attachmentservice.ConfigureStorage(attachmentservice.NewLocalStorage(t.TempDir()))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	payload := `{"data":"` + base64.StdEncoding.EncodeToString(tinyPNGBytes) + `","mediaType":"image/svg+xml"}`
	request := httptest.NewRequest(http.MethodPost, "/web/api/user/avatar", bytes.NewBufferString(payload))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected status 415, got %d body %s", recorder.Code, recorder.Body.String())
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.AvatarURL != "" {
		t.Fatalf("expected avatar URL unchanged, got %q", stored.AvatarURL)
	}
}

func TestUserActivityShouldReturnCurrentSessionUsersActivity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: alice.ID, Date: "2026-05-29", ThreadCount: 2, MessageCount: 5}); err != nil {
		t.Fatalf("save alice first activity: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: alice.ID, Date: "2026-05-30", ThreadCount: 1, MessageCount: 3}); err != nil {
		t.Fatalf("save alice second activity: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: bob.ID, Date: "2026-05-30", ThreadCount: 9, MessageCount: 99}); err != nil {
		t.Fatalf("save bob activity: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/user/activity", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode activity response: %v", err)
	}
	activity, ok := body["data"].([]any)
	if !ok || len(activity) != 2 {
		t.Fatalf("expected two activity rows, got %#v", body["data"])
	}
	first, ok := activity[0].(map[string]any)
	if !ok || first["date"] != "2026-05-29" || first["thread_count"] != float64(2) || first["message_count"] != float64(5) {
		t.Fatalf("expected first alice activity row, got %#v", activity[0])
	}
	second, ok := activity[1].(map[string]any)
	if !ok || second["date"] != "2026-05-30" || second["thread_count"] != float64(1) || second["message_count"] != float64(3) {
		t.Fatalf("expected second alice activity row, got %#v", activity[1])
	}
}

func TestUserLocaleShouldPersistSupportedLocaleOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix", PreferredLocale: "en"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/web/api/user/locale", bytes.NewBufferString(`{"locale":"zh"}`))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode locale response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["preferred_locale"] != "zh" {
		t.Fatalf("expected zh locale response, got %#v", body["data"])
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.PreferredLocale != "zh" {
		t.Fatalf("expected stored zh locale, got %q", stored.PreferredLocale)
	}

	badRecorder := httptest.NewRecorder()
	badRequest := httptest.NewRequest(http.MethodPatch, "/web/api/user/locale", bytes.NewBufferString(`{"locale":"fr"}`))
	badRequest.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(badRecorder, badRequest)
	if badRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported locale 400, got %d body %s", badRecorder.Code, badRecorder.Body.String())
	}
}

func TestUserAPIKeyRotateShouldReturnPlaintextOnceAndStoreOnlyHash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{
		Username:     "alice",
		GitHubID:     "github-alice",
		APIKeyHash:   "old-hash",
		APIKeyPrefix: "old-prefix",
	}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(user.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/web/api/user/api-key/rotate", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	plaintext, ok := data["api_key"].(string)
	if !ok || plaintext == "" {
		t.Fatalf("expected plaintext api_key to be returned once, got %#v", data["api_key"])
	}
	prefix, ok := data["prefix"].(string)
	if !ok || prefix == "" {
		t.Fatalf("expected prefix to be returned, got %#v", data["prefix"])
	}
	stored, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.APIKeyPrefix != prefix {
		t.Fatalf("expected stored prefix %q to match response %q", stored.APIKeyPrefix, prefix)
	}
	if stored.APIKeyHash == "" || stored.APIKeyHash == plaintext {
		t.Fatalf("expected stored API key hash only, got %q", stored.APIKeyHash)
	}
}

func TestUserAPIKeyRotateShouldRejectAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/web/api/user/api-key/rotate", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestThreadUploadShouldCreateCodexThreadForAPIKeyUser(t *testing.T) {
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

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", bytes.NewBufferString(`{
		"provider":"codex",
		"thread":{"id":"T-upload-one","title":"Fix auth flow","messages":[{"role":"user","content":"please fix oauth"}],"v":42},
		"repo":"owner/repo",
		"branch":"main",
		"source_url":"https://ampcode.com/threads/T-upload-one",
		"labels":["auth","bug"]
	}`))
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["provider_thread_id"] != "T-upload-one" {
		t.Fatalf("expected uploaded thread data, got %#v", body["data"])
	}
	threads, err := model.ThreadDB.Where("user_id = ? AND provider = ? AND provider_thread_id = ?", user.ID, model.ProviderCodex, "T-upload-one").Fetch(0, 1)
	if err != nil {
		t.Fatalf("fetch stored thread: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("expected one stored thread, got %d", len(threads))
	}
	thread := threads[0]
	if thread.Title != "Fix auth flow" || thread.Repo != "owner/repo" || thread.Branch != "main" || thread.SourceURL != "https://ampcode.com/threads/T-upload-one" {
		t.Fatalf("unexpected stored thread %#v", thread)
	}
	if !strings.Contains(thread.SearchText, "Fix auth flow") || !strings.Contains(thread.SearchText, "please fix oauth") || !strings.Contains(thread.SearchText, "owner/repo") || !strings.Contains(thread.SearchText, "auth") {
		t.Fatalf("expected search text to include upload metadata, got %q", thread.SearchText)
	}
}

func TestThreadUploadShouldReturnConflictWhenMessageQuotaExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", PlanLevel: model.PlanLevelFree, AutoCleanupEnabled: false}
	credential, err := authservice.ProvisionUser(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	existingUsage := &model.Thread{
		UserID:           user.ID,
		Provider:         model.ProviderCodex,
		ProviderThreadID: "T-existing-usage",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     model.FreeMessageQuota - 1,
	}
	if err := model.ThreadDB.Save(existingUsage); err != nil {
		t.Fatalf("seed existing usage: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", strings.NewReader(`{
		"provider":"codex",
		"thread":{"id":"T-over-quota","title":"Over quota","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}
	}`))
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode quota response: %v", err)
	}
	if response["message"] != "message quota exceeded" {
		t.Fatalf("expected quota message, got %#v", response["message"])
	}
}

func TestPluginThreadUploadShouldCreateClaudeCodeThreadForAPIKeyUser(t *testing.T) {
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

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", bytes.NewBufferString(`{
		"provider":"claude_code",
		"thread":{"id":"T-cc-route-one","title":"Sync Claude session","model":"claude-opus-4-1","cwd":"/Users/alice/work/jieli","created_ms":1730000000000,"updated_ms":1730000300000,"messages":[{"role":"user","message_id":"u-1","content":"sync this session"},{"role":"assistant","message_id":"a-1","content":"synced"}]},
		"repo_url":"git@git.example.test:burugo/jieli.git",
		"branch":"plugin/sync",
		"source_url":"https://jieli.test/threads/T-cc-route-one",
		"labels":["claude-code"]
	}`))
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["provider"] != model.ProviderClaudeCode || data["provider_thread_id"] != "T-cc-route-one" {
		t.Fatalf("expected uploaded Claude Code thread data, got %#v", body["data"])
	}
	threads, err := model.ThreadDB.Where("user_id = ? AND provider = ? AND provider_thread_id = ?", user.ID, model.ProviderClaudeCode, "T-cc-route-one").Fetch(0, 1)
	if err != nil {
		t.Fatalf("fetch stored thread: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("expected one stored thread, got %d", len(threads))
	}
	if threads[0].Title != "Sync Claude session" || threads[0].Repo != "burugo/jieli" || threads[0].RepoURL != "https://git.example.test/burugo/jieli" || threads[0].Branch != "plugin/sync" {
		t.Fatalf("unexpected stored thread %#v", threads[0])
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/plugin/threads?provider=claude_code&page_size=10", nil)
	listRequest.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d body %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listBody map[string]any
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	listData, ok := listBody["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected list data object, got %#v", listBody["data"])
	}
	listThreads, ok := listData["threads"].([]any)
	if !ok || len(listThreads) != 1 {
		t.Fatalf("expected one listed thread, got %#v", listData["threads"])
	}
	first, ok := listThreads[0].(map[string]any)
	if !ok || first["repo"] != "burugo/jieli" || first["repo_url"] != "https://git.example.test/burugo/jieli" {
		t.Fatalf("expected repo_url in Claude Code summary, got %#v", listThreads[0])
	}
}

func TestPluginThreadUploadShouldRateLimitPerNormalizedProviderThreadID(t *testing.T) {
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
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	upload := func(threadID string) *httptest.ResponseRecorder {
		body, err := json.Marshal(map[string]any{
			"provider": model.ProviderClaudeCode,
			"thread": map[string]any{
				"id":         threadID,
				"title":      "Rate limited Claude thread",
				"created_ms": 1730000000000,
				"updated_ms": 1730000300000,
				"messages":   []map[string]string{{"role": "user", "content": "same upload"}},
			},
		})
		if err != nil {
			t.Fatalf("marshal plugin upload: %v", err)
		}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
		router.ServeHTTP(recorder, request)
		return recorder
	}

	for index := 1; index <= 5; index++ {
		recorder := upload("claude-rate")
		if recorder.Code != http.StatusOK {
			t.Fatalf("plugin upload %d expected 200, got %d body %s", index, recorder.Code, recorder.Body.String())
		}
	}
	for index := 6; index <= 10; index++ {
		recorder := upload("T-claude-rate")
		if recorder.Code != http.StatusOK {
			t.Fatalf("normalized plugin upload %d expected 200, got %d body %s", index, recorder.Code, recorder.Body.String())
		}
	}
	if recorder := upload("claude-rate"); recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("11th normalized Claude thread upload expected 429, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if recorder := upload("claude-new-thread"); recorder.Code != http.StatusOK {
		t.Fatalf("new normalized Claude thread expected 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestPluginThreadUploadShouldCreateCodexThreadForAPIKeyUser(t *testing.T) {
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

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", bytes.NewBufferString(`{
		"provider":"codex",
		"thread":{"id":"T-codex-route-one","title":"Sync Codex session","model":"gpt-5.5","cwd":"/Users/alice/work/jieli","created_ms":1730000000000,"updated_ms":1730000300000,"messages":[{"role":"user","message_id":"u-1","content":"sync this codex session"},{"role":"assistant","message_id":"a-1","content":"synced"}]},
		"repo_url":"git@home.pika12.com:guoyb/jieli.git",
		"branch":"plugin/codex",
		"source_url":"https://jieli.test/threads/T-codex-route-one",
		"labels":["codex"]
	}`))
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["provider"] != model.ProviderCodex || data["provider_thread_id"] != "T-codex-route-one" {
		t.Fatalf("expected uploaded Codex thread data, got %#v", body["data"])
	}
	threads, err := model.ThreadDB.Where("user_id = ? AND provider = ? AND provider_thread_id = ?", user.ID, model.ProviderCodex, "T-codex-route-one").Fetch(0, 1)
	if err != nil {
		t.Fatalf("fetch stored thread: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("expected one stored thread, got %d", len(threads))
	}
	if threads[0].Title != "Sync Codex session" || threads[0].Repo != "guoyb/jieli" || threads[0].RepoURL != "https://home.pika12.com/guoyb/jieli" || threads[0].Branch != "plugin/codex" {
		t.Fatalf("unexpected stored thread %#v", threads[0])
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/plugin/threads?provider=codex&page_size=10", nil)
	listRequest.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d body %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listBody map[string]any
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	listData, ok := listBody["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected list data object, got %#v", listBody["data"])
	}
	listThreads, ok := listData["threads"].([]any)
	if !ok || len(listThreads) != 1 {
		t.Fatalf("expected one listed thread, got %#v", listData["threads"])
	}
	first, ok := listThreads[0].(map[string]any)
	if !ok || first["provider"] != model.ProviderCodex || first["repo_url"] != "https://home.pika12.com/guoyb/jieli" {
		t.Fatalf("expected Codex summary, got %#v", listThreads[0])
	}
}

func TestPluginThreadArchiveShouldUpdateOwnedProviderThread(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice"}
	credential, err := authservice.ProvisionUser(alice)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	codexThread := &model.Thread{UserID: alice.ID, Provider: model.ProviderCodex, ProviderThreadID: "T-shared", Title: "Codex thread", Visibility: model.ThreadVisibilityPrivate}
	claudeThread := &model.Thread{UserID: alice.ID, Provider: model.ProviderClaudeCode, ProviderThreadID: "T-shared", Title: "Claude thread", Visibility: model.ThreadVisibilityPrivate}
	if err := model.ThreadDB.Save(codexThread); err != nil {
		t.Fatalf("save Codex thread: %v", err)
	}
	if err := model.ThreadDB.Save(claudeThread); err != nil {
		t.Fatalf("save Claude thread: %v", err)
	}
	neoCalls := 0
	neo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		neoCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer neo.Close()

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugin/threads/archive", bytes.NewBufferString(`{
		"provider":"codex",
		"provider_thread_id":"T-shared",
		"archived":true
	}`))
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode archive response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["provider"] != model.ProviderCodex || data["provider_thread_id"] != "T-shared" || data["archived"] != true {
		t.Fatalf("expected archived Codex response, got %#v", body["data"])
	}
	storedCodex, err := model.ThreadDB.ByID(codexThread.ID)
	if err != nil {
		t.Fatalf("reload Codex thread: %v", err)
	}
	storedClaude, err := model.ThreadDB.ByID(claudeThread.ID)
	if err != nil {
		t.Fatalf("reload Claude thread: %v", err)
	}
	if !storedCodex.Archived || storedClaude.Archived {
		t.Fatalf("expected only Codex thread archived, codex=%v claude=%v", storedCodex.Archived, storedClaude.Archived)
	}
	if neoCalls != 0 {
		t.Fatalf("non-Amp archive triggered %d Neo sync calls", neoCalls)
	}
	claudeRecorder := httptest.NewRecorder()
	claudeRequest := httptest.NewRequest(http.MethodPost, "/plugin/threads/archive", bytes.NewBufferString(`{
		"provider":"claude_code",
		"provider_thread_id":"T-shared",
		"archived":true
	}`))
	claudeRequest.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	claudeRequest.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(claudeRecorder, claudeRequest)
	if claudeRecorder.Code != http.StatusOK {
		t.Fatalf("archive Claude thread: expected 200, got %d body %s", claudeRecorder.Code, claudeRecorder.Body.String())
	}
	if neoCalls != 0 {
		t.Fatalf("Claude archive triggered %d Neo sync calls", neoCalls)
	}
}

func TestPluginThreadUploadShouldCreateOpenCodeAndPiThreadsForAPIKeyUser(t *testing.T) {
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

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})

	cases := []struct {
		name     string
		provider string
		id       string
		title    string
	}{
		{name: "opencode", provider: model.ProviderOpenCode, id: "T-opencode-route-one", title: "Sync OpenCode session"},
		{name: "pi", provider: model.ProviderPi, id: "T-pi-route-one", title: "Sync Pi session"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{
				"provider":%q,
				"thread":{"id":%q,"title":%q,"model":"gpt-5.5","cwd":"/Users/alice/work/jieli","created_ms":1730000000000,"updated_ms":1730000300000,"messages":[{"role":"user","message_id":"u-1","content":"sync this session"},{"role":"assistant","message_id":"a-1","content":"synced"}]},
				"repo_url":"git@home.pika12.com:guoyb/jieli.git",
				"branch":"plugin/%s",
				"labels":[%q]
			}`, tc.provider, tc.id, tc.title, tc.name, tc.provider)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", bytes.NewBufferString(body))
			request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
			}
			var response map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode upload response: %v", err)
			}
			data, ok := response["data"].(map[string]any)
			if !ok || data["provider"] != tc.provider || data["provider_thread_id"] != tc.id {
				t.Fatalf("expected uploaded %s thread data, got %#v", tc.provider, response["data"])
			}
			threads, err := model.ThreadDB.Where("user_id = ? AND provider = ? AND provider_thread_id = ?", user.ID, tc.provider, tc.id).Fetch(0, 1)
			if err != nil {
				t.Fatalf("fetch stored thread: %v", err)
			}
			if len(threads) != 1 {
				t.Fatalf("expected one stored thread, got %d", len(threads))
			}
			if threads[0].Title != tc.title || threads[0].Repo != "guoyb/jieli" || threads[0].Branch != "plugin/"+tc.name {
				t.Fatalf("unexpected stored thread %#v", threads[0])
			}
		})
	}
}

func TestThreadUploadShouldLimitEachAPIKeyUserToTenRequestsPerMinute(t *testing.T) {
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
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})

	for index := 1; index <= 10; index++ {
		recorder := uploadThreadRequest(t, router, credential.Plaintext, "T-rate")
		if recorder.Code != http.StatusOK {
			t.Fatalf("upload %d expected 200, got %d body %s", index, recorder.Code, recorder.Body.String())
		}
	}
	recorder := uploadThreadRequest(t, router, credential.Plaintext, "T-rate")
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("11th upload for one thread expected 429, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestThreadUploadShouldAllowNewThreadWhenUpdateLimitIsExhausted(t *testing.T) {
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
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})

	for index := 1; index <= 10; index++ {
		recorder := uploadThreadRequest(t, router, credential.Plaintext, "T-existing-rate")
		if recorder.Code != http.StatusOK {
			t.Fatalf("existing upload %d expected 200, got %d body %s", index, recorder.Code, recorder.Body.String())
		}
	}
	limited := uploadThreadRequest(t, router, credential.Plaintext, "T-existing-rate")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("11th upload for one thread expected 429, got %d body %s", limited.Code, limited.Body.String())
	}

	recorder := uploadThreadRequest(t, router, credential.Plaintext, "T-new-thread")
	if recorder.Code != http.StatusOK {
		t.Fatalf("new thread creation expected 200 after update limit exhaustion, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestThreadListShouldReturnCurrentSessionUsersThreads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	aliceKey, err := authservice.RotateAPIKey(alice)
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	bobKey, err := authservice.RotateAPIKey(bob)
	if err != nil {
		t.Fatalf("rotate bob api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	uploadThread(t, router, aliceKey.Plaintext, "T-alpha", "OAuth fix", "please fix oauth", "owner/repo", "auth")
	uploadThread(t, router, aliceKey.Plaintext, "T-beta", "Billing cleanup", "billing", "owner/repo", "billing")
	uploadThread(t, router, bobKey.Plaintext, "T-alpha", "Bob oauth", "please fix oauth", "other/repo", "auth")
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/threads?search=oauth&repo=owner/repo&label=auth", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	threads, ok := data["threads"].([]any)
	if !ok || len(threads) != 1 {
		t.Fatalf("expected one listed thread, got %#v", data["threads"])
	}
	first, ok := threads[0].(map[string]any)
	if !ok || first["provider_thread_id"] != "T-alpha" {
		t.Fatalf("expected alice T-alpha, got %#v", threads[0])
	}
}

func TestPluginThreadListShouldSearchProviderThreads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	aliceKey, err := authservice.RotateAPIKey(alice)
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(alice, threadservice.UploadPayload{
		Provider: model.ProviderClaudeCode,
		Thread:   json.RawMessage(`{"id":"T-cc-oauth","title":"Claude OAuth fix","messages":[{"role":"user","message_id":"u-1","content":"please fix oauth callback"}]}`),
		Repo:     "owner/repo",
		Branch:   "plugin/search",
		Labels:   []string{"claude-code"},
	}); err != nil {
		t.Fatalf("upload alice claude thread: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(alice, threadservice.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-oauth","title":"Amp OAuth fix","messages":[{"role":"user","content":"please fix oauth"}]}`),
		Repo:     "owner/repo",
	}); err != nil {
		t.Fatalf("upload alice amp thread: %v", err)
	}
	if _, err := threadservice.UploadThreadSnapshot(bob, threadservice.UploadPayload{
		Provider: model.ProviderClaudeCode,
		Thread:   json.RawMessage(`{"id":"T-cc-bob","title":"Bob OAuth fix","messages":[{"role":"user","message_id":"u-1","content":"please fix oauth callback"}]}`),
		Repo:     "other/repo",
	}); err != nil {
		t.Fatalf("upload bob claude thread: %v", err)
	}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/plugin/threads?provider=claude_code&search=oauth&page_size=10&sort=updated", nil)
	request.Header.Set("Authorization", "Bearer "+aliceKey.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode api key list response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	threads, ok := data["threads"].([]any)
	if !ok || len(threads) != 1 {
		t.Fatalf("expected one listed thread, got %#v", data["threads"])
	}
	first, ok := threads[0].(map[string]any)
	if !ok || first["provider"] != model.ProviderClaudeCode || first["provider_thread_id"] != "T-cc-oauth" || first["repo"] != "owner/repo" || first["branch"] != "plugin/search" {
		t.Fatalf("expected alice Claude thread summary, got %#v", threads[0])
	}
	if first["preview"] == "" || first["message_count"] != float64(1) {
		t.Fatalf("expected preview and message count in summary, got %#v", first)
	}

	apiRecorder := httptest.NewRecorder()
	apiRequest := httptest.NewRequest(http.MethodGet, "/api/threads?provider=claude_code&search=oauth&page_size=10&sort=updated", nil)
	apiRequest.Header.Set("Authorization", "Bearer "+aliceKey.Plaintext)
	router.ServeHTTP(apiRecorder, apiRequest)
	if apiRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected plugin list to avoid /api prefix, got status %d body %s", apiRecorder.Code, apiRecorder.Body.String())
	}
}

func TestThreadDetailShouldReturnCurrentSessionUsersThread(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice A.", Email: "alice@example.test", AvatarURL: "https://example.test/alice.png", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	aliceKey, err := authservice.RotateAPIKey(alice)
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	bobKey, err := authservice.RotateAPIKey(bob)
	if err != nil {
		t.Fatalf("rotate bob api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	uploadThread(t, router, aliceKey.Plaintext, "T-alpha", "OAuth fix", "please fix oauth", "owner/repo", "auth")
	uploadThread(t, router, bobKey.Plaintext, "T-alpha", "Bob oauth", "please fix oauth", "other/repo", "auth")
	aliceThreads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", alice.ID, "T-alpha").Fetch(0, 1)
	if err != nil || len(aliceThreads) != 1 {
		t.Fatalf("load alice thread: rows=%d err=%v", len(aliceThreads), err)
	}
	bobThreads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", bob.ID, "T-alpha").Fetch(0, 1)
	if err != nil || len(bobThreads) != 1 {
		t.Fatalf("load bob thread: rows=%d err=%v", len(bobThreads), err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/threads/"+strconv.FormatInt(aliceThreads[0].ID, 10), nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["provider_thread_id"] != "T-alpha" || data["title"] != "OAuth fix" {
		t.Fatalf("expected alice detail data, got %#v", body["data"])
	}
	threadMessages, ok := data["messages"].([]any)
	if !ok || len(threadMessages) == 0 {
		t.Fatalf("expected embedded thread messages, got %#v", data["messages"])
	}
	labels, ok := data["labels"].([]any)
	if !ok || len(labels) != 1 || labels[0] != "auth" {
		t.Fatalf("expected auth labels, got %#v", data["labels"])
	}
	owner, ok := data["owner"].(map[string]any)
	if !ok || owner["username"] != "alice" || owner["display_name"] != "Alice A." || owner["avatar_url"] != "https://example.test/alice.png" {
		t.Fatalf("expected public owner fields, got %#v", data["owner"])
	}
	if _, ok := owner["email"]; ok {
		t.Fatalf("owner must not expose email, got %#v", owner)
	}
	if _, ok := owner["github_id"]; ok {
		t.Fatalf("owner must not expose github_id, got %#v", owner)
	}
	if _, ok := owner["api_key_prefix"]; ok {
		t.Fatalf("owner must not expose api key fields, got %#v", owner)
	}

	forbiddenRecorder := httptest.NewRecorder()
	forbiddenRequest := httptest.NewRequest(http.MethodGet, "/web/api/threads/"+strconv.FormatInt(bobThreads[0].ID, 10), nil)
	forbiddenRequest.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(forbiddenRecorder, forbiddenRequest)
	if forbiddenRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected cross-user thread read to return 404, got %d body %s", forbiddenRecorder.Code, forbiddenRecorder.Body.String())
	}
}

func TestThreadMetaShouldUpdateCurrentSessionUsersThread(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	aliceKey, err := authservice.RotateAPIKey(alice)
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	uploadThread(t, router, aliceKey.Plaintext, "T-alpha", "OAuth fix", "please fix oauth", "owner/repo", "auth")
	aliceThreads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", alice.ID, "T-alpha").Fetch(0, 1)
	if err != nil || len(aliceThreads) != 1 {
		t.Fatalf("load alice thread: rows=%d err=%v", len(aliceThreads), err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	body := bytes.NewBufferString(`{"title":"Public OAuth fix","visibility":"unlisted","labels":["oauth","shared"]}`)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/web/api/threads/"+strconv.FormatInt(aliceThreads[0].ID, 10)+"/meta", body)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode meta response: %v", err)
	}
	data, ok := response["data"].(map[string]any)
	if !ok || data["title"] != "Public OAuth fix" || data["visibility"] != "unlisted" || data["share_slug"] == "" {
		t.Fatalf("expected updated meta response, got %#v", response["data"])
	}
	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/web/api/threads/"+strconv.FormatInt(aliceThreads[0].ID, 10), nil)
	detailRequest.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d body %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detailResponse map[string]any
	if err := json.Unmarshal(detailRecorder.Body.Bytes(), &detailResponse); err != nil {
		t.Fatalf("decode detail response: %v", err)
	}
	detailData := detailResponse["data"].(map[string]any)
	labels := detailData["labels"].([]any)
	if len(labels) != 2 || labels[0] != "oauth" || labels[1] != "shared" {
		t.Fatalf("expected replacement labels, got %#v", detailData["labels"])
	}
}

func TestThreadArchiveShouldToggleCurrentSessionUsersThread(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	aliceKey, err := authservice.RotateAPIKey(alice)
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	uploadThread(t, router, aliceKey.Plaintext, "T-alpha", "OAuth fix", "please fix oauth", "owner/repo", "auth")
	aliceThreads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", alice.ID, "T-alpha").Fetch(0, 1)
	if err != nil || len(aliceThreads) != 1 {
		t.Fatalf("load alice thread: rows=%d err=%v", len(aliceThreads), err)
	}
	aliceThreads[0].Archived = true
	if err := model.ThreadDB.Save(aliceThreads[0]); err != nil {
		t.Fatalf("pre-archive alice thread: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	router = gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/web/api/threads/"+strconv.FormatInt(aliceThreads[0].ID, 10)+"/archive", bytes.NewBufferString(`{"archived":false}`))
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode archive response: %v", err)
	}
	data, ok := response["data"].(map[string]any)
	if !ok || data["archived"] != false {
		t.Fatalf("expected archived false response, got %#v", response["data"])
	}
}

func TestThreadDeleteShouldRemoveCurrentSessionUsersThread(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	aliceKey, err := authservice.RotateAPIKey(alice)
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	uploadThread(t, router, aliceKey.Plaintext, "T-alpha", "OAuth fix", "please fix oauth", "owner/repo", "auth")
	aliceThreads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", alice.ID, "T-alpha").Fetch(0, 1)
	if err != nil || len(aliceThreads) != 1 {
		t.Fatalf("load alice thread: rows=%d err=%v", len(aliceThreads), err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/web/api/threads/"+strconv.FormatInt(aliceThreads[0].ID, 10), nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(http.MethodGet, "/web/api/threads/"+strconv.FormatInt(aliceThreads[0].ID, 10), nil)
	detailRequest.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected deleted thread detail 404, got %d body %s", detailRecorder.Code, detailRecorder.Body.String())
	}
}

func TestSharedThreadShouldReturnOnlyPublicUnlistedThread(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice A.", Email: "alice@example.test", AvatarURL: "https://example.test/alice.png", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	aliceKey, err := authservice.RotateAPIKey(alice)
	if err != nil {
		t.Fatalf("rotate alice api key: %v", err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret"})
	uploadThread(t, router, aliceKey.Plaintext, "T-public", "Public OAuth fix", "please fix oauth", "owner/repo", "auth")
	uploadThread(t, router, aliceKey.Plaintext, "T-private", "Private incident", "secret", "owner/repo", "secret")
	threads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", alice.ID, "T-public").Fetch(0, 1)
	if err != nil || len(threads) != 1 {
		t.Fatalf("load public thread: rows=%d err=%v", len(threads), err)
	}
	privateThreads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", alice.ID, "T-private").Fetch(0, 1)
	if err != nil || len(privateThreads) != 1 {
		t.Fatalf("load private thread: rows=%d err=%v", len(privateThreads), err)
	}
	privateThreads[0].ShareSlug = "private-slug"
	if err := model.ThreadDB.Save(privateThreads[0]); err != nil {
		t.Fatalf("save private slug: %v", err)
	}
	cookie, err := middleware.NewSessionCookie(alice.ID, "test-secret")
	if err != nil {
		t.Fatalf("new session cookie: %v", err)
	}
	metaRecorder := httptest.NewRecorder()
	metaRequest := httptest.NewRequest(http.MethodPatch, "/web/api/threads/"+strconv.FormatInt(threads[0].ID, 10)+"/meta", bytes.NewBufferString(`{"visibility":"unlisted","labels":["oauth","shared"]}`))
	metaRequest.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(metaRecorder, metaRequest)
	if metaRecorder.Code != http.StatusOK {
		t.Fatalf("expected meta status 200, got %d body %s", metaRecorder.Code, metaRecorder.Body.String())
	}
	var metaResponse map[string]any
	if err := json.Unmarshal(metaRecorder.Body.Bytes(), &metaResponse); err != nil {
		t.Fatalf("decode meta response: %v", err)
	}
	shareSlug := metaResponse["data"].(map[string]any)["share_slug"].(string)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/share/"+shareSlug, nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("expected X-Robots-Tag noindex, got %q", recorder.Header().Get("X-Robots-Tag"))
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode shared response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["provider_thread_id"] != "T-public" || data["visibility"] != model.ThreadVisibilityUnlisted {
		t.Fatalf("expected shared thread detail, got %#v", body["data"])
	}
	labels, ok := data["labels"].([]any)
	if !ok || len(labels) != 2 || labels[0] != "oauth" || labels[1] != "shared" {
		t.Fatalf("expected shared labels, got %#v", data["labels"])
	}
	owner, ok := data["owner"].(map[string]any)
	if !ok || owner["username"] != "alice" || owner["display_name"] != "Alice A." || owner["avatar_url"] != "https://example.test/alice.png" {
		t.Fatalf("expected shared public owner fields, got %#v", data["owner"])
	}
	if _, ok := owner["email"]; ok {
		t.Fatalf("shared owner must not expose email, got %#v", owner)
	}
	if _, ok := owner["github_id"]; ok {
		t.Fatalf("shared owner must not expose github_id, got %#v", owner)
	}
	if _, ok := owner["api_key_prefix"]; ok {
		t.Fatalf("shared owner must not expose api key fields, got %#v", owner)
	}

	publicDetailRecorder := httptest.NewRecorder()
	publicDetailRequest := httptest.NewRequest(http.MethodGet, "/web/api/threads/T-public", nil)
	router.ServeHTTP(publicDetailRecorder, publicDetailRequest)
	if publicDetailRecorder.Code != http.StatusOK {
		t.Fatalf("expected public thread detail status 200, got %d body %s", publicDetailRecorder.Code, publicDetailRecorder.Body.String())
	}
	if publicDetailRecorder.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("expected public detail X-Robots-Tag noindex, got %q", publicDetailRecorder.Header().Get("X-Robots-Tag"))
	}

	privateDetailRecorder := httptest.NewRecorder()
	privateDetailRequest := httptest.NewRequest(http.MethodGet, "/web/api/threads/T-private", nil)
	router.ServeHTTP(privateDetailRecorder, privateDetailRequest)
	if privateDetailRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected private public detail 404, got %d body %s", privateDetailRecorder.Code, privateDetailRecorder.Body.String())
	}

	router.NoRoute(func(c *gin.Context) {
		if route.ServeThreadExport(c, route.Config{SessionSecret: "test-secret"}) {
			return
		}
		c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "not found"})
	})

	exportRecorder := httptest.NewRecorder()
	exportRequest := httptest.NewRequest(http.MethodGet, "/threads/T-public.md", nil)
	router.ServeHTTP(exportRecorder, exportRequest)
	if exportRecorder.Code != http.StatusOK {
		t.Fatalf("expected public markdown export status 200, got %d body %s", exportRecorder.Code, exportRecorder.Body.String())
	}
	if exportRecorder.Header().Get("X-Robots-Tag") != "noindex" {
		t.Fatalf("expected public export X-Robots-Tag noindex, got %q", exportRecorder.Header().Get("X-Robots-Tag"))
	}
	if !strings.Contains(exportRecorder.Body.String(), "Public OAuth fix") {
		t.Fatalf("expected markdown export to include thread title, got %s", exportRecorder.Body.String())
	}

	privateExportRecorder := httptest.NewRecorder()
	privateExportRequest := httptest.NewRequest(http.MethodGet, "/threads/T-private.md", nil)
	router.ServeHTTP(privateExportRecorder, privateExportRequest)
	if privateExportRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected private markdown export 401, got %d body %s", privateExportRecorder.Code, privateExportRecorder.Body.String())
	}
	if !strings.Contains(privateExportRecorder.Body.String(), "Use the jieli skill") || !strings.Contains(privateExportRecorder.Body.String(), "do not use WebFetch or curl") {
		t.Fatalf("expected private markdown export to guide agents to the jieli skill, got %s", privateExportRecorder.Body.String())
	}

	privateJSONExportRecorder := httptest.NewRecorder()
	privateJSONExportRequest := httptest.NewRequest(http.MethodGet, "/threads/T-private.json", nil)
	router.ServeHTTP(privateJSONExportRecorder, privateJSONExportRequest)
	if privateJSONExportRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected private json export 401, got %d body %s", privateJSONExportRecorder.Code, privateJSONExportRecorder.Body.String())
	}
	if !strings.Contains(privateJSONExportRecorder.Body.String(), "Use the installed Jieli jieli skill") {
		t.Fatalf("expected private json export to include agent hint, got %s", privateJSONExportRecorder.Body.String())
	}

	privateRecorder := httptest.NewRecorder()
	privateRequest := httptest.NewRequest(http.MethodGet, "/web/api/share/private-slug", nil)
	router.ServeHTTP(privateRecorder, privateRequest)
	if privateRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected private slug 404, got %d body %s", privateRecorder.Code, privateRecorder.Body.String())
	}
	missingRecorder := httptest.NewRecorder()
	missingRequest := httptest.NewRequest(http.MethodGet, "/web/api/share/missing-slug", nil)
	router.ServeHTTP(missingRecorder, missingRequest)
	if missingRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing slug 404, got %d body %s", missingRecorder.Code, missingRecorder.Body.String())
	}
}

func TestGitHubOAuthShouldRedirectToGitHubAuthorizeURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{GitHub: common.GitHubConfig{
		ClientID:    "github-client-id",
		CallbackURL: "https://jieli.example.com/api/auth/callback/github",
	}})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/github", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	location := recorder.Header().Get("Location")
	if location == "" {
		t.Fatalf("expected redirect location")
	}
	if !strings.HasPrefix(location, "https://github.com/login/oauth/authorize?") {
		t.Fatalf("expected GitHub authorize URL, got %q", location)
	}
	if !strings.Contains(location, "client_id=github-client-id") || !strings.Contains(location, "redirect_uri=https%3A%2F%2Fjieli.example.com%2Fapi%2Fauth%2Fcallback%2Fgithub") {
		t.Fatalf("expected client_id and redirect_uri in location, got %q", location)
	}
	parsedLocation, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	state := parsedLocation.Query().Get("state")
	if state == "" {
		t.Fatalf("expected oauth state in redirect location")
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "jieli_oauth_state" && cookie.Value == state && cookie.HttpOnly {
			return
		}
	}
	t.Fatalf("expected http-only oauth state cookie matching redirect state")
}

func TestWebConfigShouldReportConfiguredOAuthProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		DevMode: true,
		GitHub: common.GitHubConfig{
			ClientID:    "github-client-id",
			CallbackURL: "https://jieli.example.com/web/api/auth/callback/github",
		},
		Google: common.GoogleConfig{
			ClientID:    "google-client-id",
			CallbackURL: "https://jieli.example.com/web/api/auth/callback/google",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/config", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["dev_mode"] != true || data["github_oauth"] != true || data["google_oauth"] != true {
		t.Fatalf("expected configured oauth providers, got %#v", body["data"])
	}
}

func TestGitHubOAuthCallbackShouldRejectMismatchedState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	client := &countingGitHubClient{user: authservice.GitHubUser{ID: 12345, Login: "alice"}}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "test-secret",
		GitHubClient:  client,
		GitHub: common.GitHubConfig{
			ClientID:          "github-client-id",
			ClientSecret:      "github-client-secret",
			CallbackURL:       "https://jieli.example.com/api/auth/callback/github",
			PostLoginRedirect: "/threads",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/callback/github?code=oauth-code&state=query-state", nil)
	request.AddCookie(&http.Cookie{Name: "jieli_oauth_state", Value: "cookie-state"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected mismatched oauth state to return 400, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if client.calls != 0 {
		t.Fatalf("expected mismatched oauth state not to call GitHub, got %d calls", client.calls)
	}
}

func TestGitHubOAuthCallbackShouldRedirectCancelledAuthorizationToSignin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	client := &countingGitHubClient{user: authservice.GitHubUser{ID: 12345, Login: "alice"}}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "test-secret",
		GitHubClient:  client,
		GitHub: common.GitHubConfig{
			ClientID:          "github-client-id",
			ClientSecret:      "github-client-secret",
			CallbackURL:       "https://jieli.example.com/api/auth/callback/github",
			PostLoginRedirect: "/threads",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/callback/github?error=access_denied&state=oauth-state", nil)
	request.AddCookie(&http.Cookie{Name: "jieli_oauth_state", Value: "oauth-state"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != "/web/auth/signin?error=oauth_cancelled" {
		t.Fatalf("expected cancelled oauth signin redirect, got %q", location)
	}
	if client.calls != 0 {
		t.Fatalf("expected cancelled oauth not to call GitHub, got %d calls", client.calls)
	}
}

func TestGitHubOAuthCallbackShouldCreateSessionAndRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	client := &countingGitHubClient{user: authservice.GitHubUser{
		ID:        12345,
		Login:     "alice",
		Name:      "Alice Doe",
		Email:     "alice@example.com",
		AvatarURL: "https://example.com/avatar.png",
	}}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "test-secret",
		GitHubClient:  client,
		GitHub: common.GitHubConfig{
			ClientID:          "github-client-id",
			ClientSecret:      "github-client-secret",
			CallbackURL:       "https://jieli.example.com/api/auth/callback/github",
			PostLoginRedirect: "/threads",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/callback/github?code=oauth-code&state=oauth-state", nil)
	request.AddCookie(&http.Cookie{Name: "jieli_oauth_state", Value: "oauth-state"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != "/threads" {
		t.Fatalf("expected post-login redirect /threads, got %q", location)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == middleware.SessionCookieName {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" || !sessionCookie.HttpOnly {
		t.Fatalf("expected http-only session cookie, got %#v", sessionCookie)
	}
	var clearStateCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "jieli_oauth_state" && cookie.MaxAge < 0 {
			clearStateCookie = cookie
		}
	}
	if clearStateCookie == nil {
		t.Fatalf("expected callback to clear oauth state cookie")
	}

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/web/api/auth/me", nil)
	meRequest.AddCookie(sessionCookie)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("expected session cookie to authenticate, got %d body %s", meRecorder.Code, meRecorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["username"] != "alice" {
		t.Fatalf("expected authenticated GitHub user alice, got %#v", body["data"])
	}
	if client.calls != 1 {
		t.Fatalf("expected callback to call GitHub once, got %d calls", client.calls)
	}
}

func TestGitHubOAuthCallbackShouldRedirectDuplicateEmailToSignin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	existing := &model.User{
		Username:     "alice-google",
		Email:        "alice@example.com",
		GoogleID:     "google-sub-123",
		APIKeyHash:   "google-hash",
		APIKeyPrefix: "google-prefix",
	}
	if err := model.UserDB.Save(existing); err != nil {
		t.Fatalf("save existing user: %v", err)
	}
	client := &countingGitHubClient{user: authservice.GitHubUser{
		ID:    12345,
		Login: "alice",
		Email: "alice@example.com",
	}}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "test-secret",
		GitHubClient:  client,
		GitHub: common.GitHubConfig{
			ClientID:          "github-client-id",
			ClientSecret:      "github-client-secret",
			CallbackURL:       "https://jieli.example.com/api/auth/callback/github",
			PostLoginRedirect: "/threads",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/callback/github?code=oauth-code&state=oauth-state", nil)
	request.AddCookie(&http.Cookie{Name: "jieli_oauth_state", Value: "oauth-state"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != "/web/auth/signin?error=oauth_email_exists" {
		t.Fatalf("expected duplicate email signin redirect, got %q", location)
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == middleware.SessionCookieName && cookie.Value != "" {
			t.Fatalf("expected duplicate email not to create session cookie")
		}
	}
}

func TestGoogleOAuthShouldRedirectToGoogleAuthorizeURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{Google: common.GoogleConfig{
		ClientID:    "google-client-id",
		CallbackURL: "https://jieli.example.com/web/api/auth/callback/google",
	}})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/google", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	location := recorder.Header().Get("Location")
	if !strings.HasPrefix(location, "https://accounts.google.com/o/oauth2/v2/auth?") {
		t.Fatalf("expected Google authorize URL, got %q", location)
	}
	parsedLocation, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	query := parsedLocation.Query()
	if query.Get("client_id") != "google-client-id" || query.Get("redirect_uri") != "https://jieli.example.com/web/api/auth/callback/google" {
		t.Fatalf("expected client_id and redirect_uri in location, got %q", location)
	}
	if query.Get("scope") != "openid email profile" || query.Get("response_type") != "code" {
		t.Fatalf("expected Google scope and response_type, got %q", location)
	}
	state := query.Get("state")
	if state == "" {
		t.Fatalf("expected oauth state in redirect location")
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "jieli_oauth_state_google" && cookie.Value == state && cookie.HttpOnly {
			return
		}
	}
	t.Fatalf("expected http-only Google oauth state cookie matching redirect state")
}

func TestGoogleOAuthCallbackShouldRejectMismatchedState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	client := &countingGoogleClient{user: authservice.GoogleUser{ID: "google-sub-123", Email: "alice@example.com"}}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "test-secret",
		GoogleClient:  client,
		Google: common.GoogleConfig{
			ClientID:          "google-client-id",
			ClientSecret:      "google-client-secret",
			CallbackURL:       "https://jieli.example.com/web/api/auth/callback/google",
			PostLoginRedirect: "/threads",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/callback/google?code=oauth-code&state=query-state", nil)
	request.AddCookie(&http.Cookie{Name: "jieli_oauth_state_google", Value: "cookie-state"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected mismatched oauth state to return 400, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if client.calls != 0 {
		t.Fatalf("expected mismatched oauth state not to call Google, got %d calls", client.calls)
	}
}

func TestGoogleOAuthCallbackShouldRedirectCancelledAuthorizationToSignin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	client := &countingGoogleClient{user: authservice.GoogleUser{ID: "google-sub-123", Email: "alice@example.com"}}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "test-secret",
		GoogleClient:  client,
		Google: common.GoogleConfig{
			ClientID:          "google-client-id",
			ClientSecret:      "google-client-secret",
			CallbackURL:       "https://jieli.example.com/web/api/auth/callback/google",
			PostLoginRedirect: "/threads",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/callback/google?error=access_denied&state=oauth-state", nil)
	request.AddCookie(&http.Cookie{Name: "jieli_oauth_state_google", Value: "oauth-state"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != "/web/auth/signin?error=oauth_cancelled" {
		t.Fatalf("expected cancelled oauth signin redirect, got %q", location)
	}
	if client.calls != 0 {
		t.Fatalf("expected cancelled oauth not to call Google, got %d calls", client.calls)
	}
}

func TestGoogleOAuthCallbackShouldCreateSessionAndRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	client := &countingGoogleClient{user: authservice.GoogleUser{
		ID:         "google-sub-123",
		Email:      "alice@example.com",
		Name:       "Alice Doe",
		PictureURL: "https://example.com/avatar.png",
	}}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "test-secret",
		GoogleClient:  client,
		Google: common.GoogleConfig{
			ClientID:          "google-client-id",
			ClientSecret:      "google-client-secret",
			CallbackURL:       "https://jieli.example.com/web/api/auth/callback/google",
			PostLoginRedirect: "/threads",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/callback/google?code=oauth-code&state=oauth-state", nil)
	request.AddCookie(&http.Cookie{Name: "jieli_oauth_state_google", Value: "oauth-state"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != "/threads" {
		t.Fatalf("expected post-login redirect /threads, got %q", location)
	}
	var sessionCookie *http.Cookie
	var clearStateCookie *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == middleware.SessionCookieName {
			sessionCookie = cookie
		}
		if cookie.Name == "jieli_oauth_state_google" && cookie.MaxAge < 0 {
			clearStateCookie = cookie
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" || !sessionCookie.HttpOnly {
		t.Fatalf("expected http-only session cookie, got %#v", sessionCookie)
	}
	if clearStateCookie == nil {
		t.Fatalf("expected callback to clear oauth state cookie")
	}

	meRecorder := httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/web/api/auth/me", nil)
	meRequest.AddCookie(sessionCookie)
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("expected session cookie to authenticate, got %d body %s", meRecorder.Code, meRecorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(meRecorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["username"] != "alice" || data["google_id"] != "google-sub-123" {
		t.Fatalf("expected authenticated Google user alice, got %#v", body["data"])
	}
	if client.calls != 1 {
		t.Fatalf("expected callback to call Google once, got %d calls", client.calls)
	}
}

func TestGoogleOAuthCallbackShouldRedirectDuplicateEmailToSignin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	existing := &model.User{
		Username:     "alice-github",
		Email:        "alice@example.com",
		GitHubID:     "12345",
		APIKeyHash:   "github-hash",
		APIKeyPrefix: "github-prefix",
	}
	if err := model.UserDB.Save(existing); err != nil {
		t.Fatalf("save existing user: %v", err)
	}
	client := &countingGoogleClient{user: authservice.GoogleUser{
		ID:    "google-sub-123",
		Email: "alice@example.com",
		Name:  "Alice Doe",
	}}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "test-secret",
		GoogleClient:  client,
		Google: common.GoogleConfig{
			ClientID:          "google-client-id",
			ClientSecret:      "google-client-secret",
			CallbackURL:       "https://jieli.example.com/web/api/auth/callback/google",
			PostLoginRedirect: "/threads",
		},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/callback/google?code=oauth-code&state=oauth-state", nil)
	request.AddCookie(&http.Cookie{Name: "jieli_oauth_state_google", Value: "oauth-state"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != "/web/auth/signin?error=oauth_email_exists" {
		t.Fatalf("expected duplicate email signin redirect, got %q", location)
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == middleware.SessionCookieName && cookie.Value != "" {
			t.Fatalf("expected duplicate email not to create session cookie")
		}
	}
}

func TestDevLoginShouldCreateSessionAndRedirectToDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}

	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		DevMode:       true,
		SessionSecret: "test-secret",
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/web/api/auth/dev-login", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d body %s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != "/web/dashboard" {
		t.Fatalf("expected dev login redirect /web/dashboard, got %q", location)
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == middleware.SessionCookieName && cookie.Value != "" && cookie.HttpOnly {
			return
		}
	}
	t.Fatalf("expected dev login to set http-only session cookie")
}

type countingGitHubClient struct {
	calls int
	user  authservice.GitHubUser
}

func (client *countingGitHubClient) UserByCode(string) (authservice.GitHubUser, error) {
	client.calls++
	return client.user, nil
}

type countingGoogleClient struct {
	calls int
	user  authservice.GoogleUser
}

func (client *countingGoogleClient) UserByCode(string) (authservice.GoogleUser, error) {
	client.calls++
	return client.user, nil
}

type memoryStorage struct {
	driverName string
	objects    map[string][]byte
}

func newMemoryStorage(driverName string) *memoryStorage {
	return &memoryStorage{driverName: driverName, objects: map[string][]byte{}}
}

func (storage *memoryStorage) DriverName() string {
	return storage.driverName
}

func (storage *memoryStorage) Write(key string, data []byte) error {
	storage.objects[key] = append([]byte(nil), data...)
	return nil
}

func (storage *memoryStorage) Read(key string) ([]byte, error) {
	data, ok := storage.objects[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (storage *memoryStorage) Delete(key string) error {
	delete(storage.objects, key)
	return nil
}

func uploadThread(t *testing.T, router http.Handler, apiKey string, id string, title string, content string, repo string, label string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"provider": model.ProviderCodex,
		"thread": map[string]any{
			"id":    id,
			"title": title,
			"messages": []map[string]string{
				{"role": "user", "content": content},
			},
		},
		"repo":   repo,
		"labels": []string{label},
	})
	if err != nil {
		t.Fatalf("marshal upload payload: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+apiKey)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload %s expected status 200, got %d body %s", id, recorder.Code, recorder.Body.String())
	}
}

func uploadThreadRequest(t *testing.T, router http.Handler, apiKey string, id string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"provider": model.ProviderCodex,
		"thread": map[string]any{
			"id":    id,
			"title": "Rate limited thread",
			"messages": []map[string]string{
				{"role": "user", "content": "same upload"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal upload payload: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+apiKey)
	router.ServeHTTP(recorder, request)
	return recorder
}

func internalUploadThreadRequest(t *testing.T, router http.Handler, apiKey string, id string) *httptest.ResponseRecorder {
	return internalUploadThreadRequestWithMessages(t, router, apiKey, id, []map[string]string{{"role": "user", "content": "same internal upload"}})
}

func internalUploadThreadRequestWithMessages(t *testing.T, router http.Handler, apiKey string, id string, messages []map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"method": "uploadThread",
		"params": map[string]any{
			"thread": map[string]any{
				"id":       id,
				"title":    "Rate limited internal thread",
				"messages": messages,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal internal upload payload: %v", err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/internal?uploadThread", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+apiKey)
	router.ServeHTTP(recorder, request)
	return recorder
}

func closeModelDB(t *testing.T) func() {
	t.Helper()
	return func() {
		if err := model.CloseDB(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}

func TestPluginUploadRejectsUnsupportedProviderWithoutWriting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "retired.db")); err != nil {
		t.Fatal(err)
	}
	user := &model.User{Username: "alice", GitHubID: "retired-test"}
	credential, err := authservice.ProvisionUser(user)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	route.Register(router)
	req := httptest.NewRequest(http.MethodPost, "/plugin/threads/upload", strings.NewReader(`{"provider":"amp","thread":{"id":"T-retired","messages":[{"role":"user","content":"must not write"}]}}`))
	req.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 400 {
		t.Fatalf("upload response: %d %s", res.Code, res.Body.String())
	}
	count, err := model.ThreadDB.Where("user_id = ?", user.ID).WithDeleted().Count()
	if err != nil || count != 0 {
		t.Fatalf("rejected upload persisted threads: %d %v", count, err)
	}
}
