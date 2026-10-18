package route_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"golang.org/x/crypto/bcrypt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/route"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
)

func TestEmailRegistrationRequiresTestedSMTPAndSingleUseVerification(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "email.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	var messages []string
	var smtpPasswords []string
	smtpFails := true
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "secret", APIKeyEncryptionSecret: "secret", SetupToken: "setup-secret", SendMail: func(_ context.Context, smtp common.SMTPConfig, _ string, body string) error {
		if smtpFails {
			return errors.New("test SMTP unavailable")
		}
		smtpPasswords = append(smtpPasswords, smtp.Password)
		messages = append(messages, body)
		return nil
	}})
	send := func(path, payload string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", path, strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.0.2.10:12345"
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	res := send("/web/api/auth/setup", `{"token":"setup-secret","email":"owner@example.com","password":"correct-horse-battery"}`, nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	cookie := res.Result().Cookies()[0]
	if res = send("/web/api/admin/registration", `{"enabled":true}`, cookie); res.Code != 400 {
		t.Fatalf("enabled without tested SMTP: %d", res.Code)
	}
	failed := send("/web/api/admin/smtp/test", `{"host":"smtp.example.com","port":465,"from":"noreply@example.com","username":"mailer","password":"mail-secret","tls_mode":"tls"}`, cookie)
	if failed.Code != 502 {
		t.Fatalf("SMTP failure not reported: %d", failed.Code)
	}
	if res = send("/web/api/admin/registration", `{"enabled":true}`, cookie); res.Code != 400 {
		t.Fatal("failed SMTP allowed registration")
	}
	optionsReq := httptest.NewRequest("GET", "/web/api/auth/options", nil)
	optionsReq.RemoteAddr = "192.0.2.30:1000"
	optionsRes := httptest.NewRecorder()
	router.ServeHTTP(optionsRes, optionsReq)
	var options struct {
		Data struct {
			Recovery     bool `json:"recovery_enabled"`
			Registration bool `json:"registration_enabled"`
		}
	}
	if err := json.Unmarshal(optionsRes.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if optionsRes.Code != 200 || options.Data.Recovery || options.Data.Registration {
		t.Fatal("failed SMTP persisted a tested configuration")
	}
	smtpFails = false
	res = send("/web/api/admin/smtp/test", `{"host":"smtp.example.com","port":465,"from":"noreply@example.com","username":"mailer","password":"mail-secret","tls_mode":"tls"}`, cookie)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res = send("/web/api/admin/site-settings", `{"public_base_url":"http://example.test"}`, cookie); res.Code != 200 {
		t.Fatalf("site URL save failed: %d %s", res.Code, res.Body.String())
	}
	res = send("/web/api/admin/smtp/test", `{"host":"smtp.example.com","port":465,"from":"noreply@example.com","username":"mailer","password":"","tls_mode":"tls"}`, cookie)
	if res.Code != 200 || len(smtpPasswords) != 2 || smtpPasswords[1] != "mail-secret" {
		t.Fatalf("saved SMTP password was not reused: %d passwords=%v body=%s", res.Code, smtpPasswords, res.Body.String())
	}
	req := httptest.NewRequest("GET", "/web/api/admin/site-settings", nil)
	req.AddCookie(cookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if strings.Contains(res.Body.String(), "mail-secret") {
		t.Fatal("SMTP credential leaked")
	}
	if res = send("/web/api/auth/register", `{"email":"alice@example.com","password":"correct-horse-battery"}`, nil); res.Code != 403 {
		t.Fatal("SMTP test implicitly enabled registration")
	}
	if res = send("/web/api/admin/registration", `{"enabled":true}`, cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	invalidName, err := json.Marshal(map[string]string{"email": "alice@example.com", "password": "correct-horse-battery", "display_name": strings.Repeat("名", 81)})
	if err != nil {
		t.Fatal(err)
	}
	invalidReq := httptest.NewRequest("POST", "/web/api/auth/register", bytes.NewReader(invalidName))
	invalidReq.RemoteAddr = "192.0.2.40:1000"
	invalidRes := httptest.NewRecorder()
	router.ServeHTTP(invalidRes, invalidReq)
	if invalidRes.Code != 400 || !strings.Contains(invalidRes.Body.String(), "display name") {
		t.Fatal("oversized registration name accepted")
	}
	displayName := strings.Repeat("名😀", 40)
	registration, err := json.Marshal(map[string]string{"email": "alice@example.com", "password": "correct-horse-battery", "display_name": "  " + displayName + "  "})
	if err != nil {
		t.Fatal(err)
	}
	if res = send("/web/api/auth/register", string(registration), nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res = send("/web/api/auth/login", `{"email":"alice@example.com","password":"correct-horse-battery"}`, nil); res.Code != 401 {
		t.Fatal("unverified email logged in")
	}
	link, err := url.Parse(strings.TrimSpace(messages[len(messages)-1]))
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"token": link.Fragment})
	if res = send("/web/api/auth/email/complete", string(payload), nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res = send("/web/api/auth/email/complete", string(payload), nil); res.Code != 400 {
		t.Fatal("verification token reused")
	}
	if res = send("/web/api/auth/login", `{"email":"alice@example.com","password":"correct-horse-battery"}`, nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	var registered struct {
		Data struct {
			DisplayName string `json:"display_name"`
			Username    string `json:"username"`
		}
	}
	if err := json.Unmarshal(res.Body.Bytes(), &registered); err != nil {
		t.Fatal(err)
	}
	if registered.Data.DisplayName != displayName {
		t.Fatalf("verified registration lost display name: %s", res.Body.String())
	}
	if !regexp.MustCompile(`^user[0-9a-f]{8}$`).MatchString(registered.Data.Username) {
		t.Fatalf("verified registration did not use an eight-character hash: %q", registered.Data.Username)
	}
	oldCookie := res.Result().Cookies()[0]
	if res = send("/web/api/auth/reset", `{"email":"alice@example.com"}`, nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	resetLink, err := url.Parse(strings.TrimSpace(messages[len(messages)-1]))
	if err != nil {
		t.Fatal(err)
	}
	resetPayload, _ := json.Marshal(map[string]string{"token": resetLink.Fragment, "password": "reset-new-password"})
	if res = send("/web/api/auth/email/complete", string(resetPayload), nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	req = httptest.NewRequest("GET", "/web/api/user/profile", nil)
	req.AddCookie(oldCookie)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 401 {
		t.Fatalf("reset did not invalidate old session: %d", res.Code)
	}
	// Use a second client to exercise credential validation rather than the per-IP rate cap.
	req = httptest.NewRequest("POST", "/web/api/auth/login", strings.NewReader(`{"email":"alice@example.com","password":"reset-new-password"}`))
	req.RemoteAddr = "192.0.2.20:1000"
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 200 {
		t.Fatalf("reset password login: %d %s", res.Code, res.Body.String())
	}

}

func TestHTTPBaseURLAllowsEmailRegistration(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "email-dev.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	admin := &model.User{Username: "admin", Email: "admin@example.com", APIKeyHash: "admin", IsAdmin: true}
	if err := model.UserDB.Save(admin); err != nil {
		t.Fatal(err)
	}
	cookie, err := middleware.NewSessionCookieForUser(admin, "secret")
	if err != nil {
		t.Fatal(err)
	}
	var messages []string
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{
		SessionSecret: "secret", APIKeyEncryptionSecret: "secret",
		SendMail: func(_ context.Context, _ common.SMTPConfig, _ string, body string) error {
			messages = append(messages, body)
			return nil
		},
	})
	post := func(path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	if res := post("/web/api/admin/site-settings", `{"public_base_url":"http://saved.example:8080"}`); res.Code != 200 {
		t.Fatalf("site settings save failed: %d %s", res.Code, res.Body.String())
	}
	if res := post("/web/api/admin/smtp/test", `{"host":"smtp.example.com","port":465,"from":"noreply@example.com","username":"mailer","password":"mail-secret","tls_mode":"tls"}`); res.Code != 200 {
		t.Fatalf("SMTP test failed: %d %s", res.Code, res.Body.String())
	}
	if res := post("/web/api/admin/registration", `{"enabled":true}`); res.Code != 200 {
		t.Fatalf("dev registration was rejected: %d %s", res.Code, res.Body.String())
	}
	if res := post("/web/api/auth/register", `{"email":"new@example.com","password":"correct-horse-battery"}`); res.Code != 200 {
		t.Fatalf("email registration failed: %d %s", res.Code, res.Body.String())
	}
	if len(messages) < 2 || !strings.HasPrefix(messages[len(messages)-1], "http://saved.example:8080/web/auth/email?") {
		t.Fatalf("registration did not use saved public URL: %v", messages)
	}
	if res := post("/web/api/admin/site-settings", `{"public_base_url":""}`); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res := post("/web/api/admin/registration", `{"enabled":true}`); res.Code != 400 {
		t.Fatal("enabled registration without URL")
	}
	before := len(messages)
	if res := post("/web/api/auth/reset", `{"email":"admin@example.com"}`); res.Code != 400 {
		t.Fatal("sent recovery without URL")
	}
	if len(messages) != before {
		t.Fatal("mail sent without URL")
	}
	options := httptest.NewRecorder()
	router.ServeHTTP(options, httptest.NewRequest("GET", "/web/api/auth/options", nil))
	if !strings.Contains(options.Body.String(), `"recovery_enabled":false`) || !strings.Contains(options.Body.String(), `"registration_enabled":false`) {
		t.Fatal("options advertise unavailable email flows")
	}

}

func TestSiteSettingsPersistPublicBaseURL(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "site-settings.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	admin := &model.User{Username: "admin", Email: "admin@example.com", APIKeyHash: "admin", IsAdmin: true}
	if err := model.UserDB.Save(admin); err != nil {
		t.Fatal(err)
	}
	cookie, err := middleware.NewSessionCookieForUser(admin, "secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "secret"})
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	if res := request(http.MethodGet, "/web/api/admin/site-settings", ""); res.Code != 200 || !strings.Contains(res.Body.String(), `"public_base_url":""`) {
		t.Fatalf("site settings GET: %d %s", res.Code, res.Body.String())
	}
	if res := request(http.MethodPost, "/web/api/admin/site-settings", `{`); res.Code != http.StatusBadRequest {
		t.Fatalf("invalid site settings JSON should be rejected: %d %s", res.Code, res.Body.String())
	}
	if res := request(http.MethodPost, "/web/api/admin/site-settings", `{"public_base_url":"not a URL"}`); res.Code != http.StatusBadRequest {
		t.Fatalf("invalid site URL should be rejected: %d %s", res.Code, res.Body.String())
	}
	if res := request(http.MethodPost, "/web/api/admin/site-settings", `{"public_base_url":"http://saved.example:8080"}`); res.Code != 200 {
		t.Fatalf("site settings POST: %d %s", res.Code, res.Body.String())
	}
	if res := request(http.MethodGet, "/web/api/admin/site-settings", ""); res.Code != 200 || !strings.Contains(res.Body.String(), `"public_base_url":"http://saved.example:8080"`) {
		t.Fatalf("site settings GET after save: %d %s", res.Code, res.Body.String())
	}
	for _, badURL := range []string{"ftp://example.test", "https://user:pass@example.test", "https://example.test?x=1", "https://example.test#fragment", "http://:8080"} {
		payload, _ := json.Marshal(map[string]string{"public_base_url": badURL})
		if res := request(http.MethodPost, "/web/api/admin/site-settings", string(payload)); res.Code != 400 {
			t.Fatalf("invalid site URL accepted: %q status=%d", badURL, res.Code)
		}
	}
	if res := request(http.MethodGet, "/web/api/admin/site-settings", ""); !strings.Contains(res.Body.String(), `"public_base_url":"http://saved.example:8080"`) {
		t.Fatal("invalid request changed stored URL")
	}
	regular := &model.User{Username: "regular", Email: "regular@example.com", APIKeyHash: "regular", IsAdmin: false}
	if err = model.UserDB.Save(regular); err != nil {
		t.Fatal(err)
	}
	regularCookie, err := middleware.NewSessionCookieForUser(regular, "secret")
	if err != nil {
		t.Fatal(err)
	}
	requestAs := func(cookieValue, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookieValue})
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	if res := requestAs(regularCookie, http.MethodPost, "/web/api/admin/site-settings", `{"public_base_url":"https://untrusted.example"}`); res.Code != http.StatusForbidden {
		t.Fatalf("non-admin changed site URL: %d", res.Code)
	}
	if res := requestAs(regularCookie, http.MethodGet, "/web/api/admin/site-settings", ""); res.Code != http.StatusForbidden {
		t.Fatalf("non-admin site settings GET should be forbidden: %d %s", res.Code, res.Body.String())
	}
}

func TestAdminSetupAndUserCreationWithoutSMTP(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "accounts.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "test-secret", APIKeyEncryptionSecret: "test-secret", SetupToken: "operator-only-setup-token", SelfHosted: true})
	send := func(path, payload string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.0.2.10:12345"
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	if res := send("/web/api/auth/register", `{"email":"alice@example.com","password":"correct-horse-battery"}`, nil); res.Code != 403 {
		t.Fatalf("registration should be closed: %d", res.Code)
	}
	if res := send("/web/api/auth/setup", `{"token":"wrong","email":"owner@example.com","password":"correct-horse-battery"}`, nil); res.Code != 403 {
		t.Fatalf("setup accepted wrong token: %d", res.Code)
	}
	res := send("/web/api/auth/setup", `{"token":"operator-only-setup-token","email":"owner@example.com","password":"correct-horse-battery"}`, nil)
	if res.Code != 200 || len(res.Result().Cookies()) == 0 {
		t.Fatalf("setup: %d %s", res.Code, res.Body.String())
	}
	adminCookie := res.Result().Cookies()[0]
	tooLong, _ := json.Marshal(map[string]string{"email": "alice@example.com", "password": strings.Repeat("a", 73)})
	if rejected := send("/web/api/admin/users", string(tooLong), adminCookie); rejected.Code != 400 {
		t.Fatal("overlong temporary password accepted")
	}
	if rejected := send("/web/api/admin/users", `{"email":"alice@example.com","password":"Ab1!xyz"}`, adminCookie); rejected.Code != 400 {
		t.Fatalf("seven-character temporary password accepted: %d %s", rejected.Code, rejected.Body.String())
	}
	res = send("/web/api/admin/users", `{"email":"alice@example.com","password":"Ab1!xyzw"}`, adminCookie)
	if res.Code != 200 {
		t.Fatalf("create user: %d %s", res.Code, res.Body.String())
	}
	res = send("/web/api/auth/login", `{"email":"alice@example.com","password":"Ab1!xyzw"}`, nil)
	if res.Code != 200 {
		t.Fatalf("login: %d %s", res.Code, res.Body.String())
	}
	userCookie := res.Result().Cookies()[0]
	users, err := model.UserDB.Where("login_email = ?", "alice@example.com").Fetch(0, 1)
	if err != nil || len(users) != 1 {
		t.Fatalf("user: %v", err)
	}
	key, err := common.DecryptSecret(users[0].APIKeyEncrypted, "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	checkAccess := func(cookie *http.Cookie, want int) {
		t.Helper()
		for _, path := range []string{"/web/api/user/profile", "/plugin/threads"} {
			req := httptest.NewRequest("GET", path, nil)
			if path == "/plugin/threads" {
				req.Header.Set("Authorization", "Bearer "+key)
			} else {
				req.AddCookie(cookie)
			}
			result := httptest.NewRecorder()
			router.ServeHTTP(result, req)
			expected := want
			if path == "/plugin/threads" && want == 403 {
				expected = 401
			}
			if result.Code != expected {
				t.Fatalf("temporary-password gate %s: want %d got %d %s", path, want, result.Code, result.Body.String())
			}
		}
	}
	checkAccess(userCookie, 403)

	if res = send("/web/api/admin/users", `{"email":"mallory@example.com","password":"temporary-password"}`, userCookie); res.Code != 403 {
		t.Fatalf("non-admin created user: %d", res.Code)
	}
	if denied := send("/web/api/auth/password", `{"password":"new-secure-password"}`, nil); denied.Code != 401 {
		t.Fatalf("anonymous password change: %d", denied.Code)
	}
	res = send("/web/api/auth/password", `{"password":"Ab1!xy"}`, userCookie)
	if res.Code != 200 {
		t.Fatalf("change temporary password: %d %s", res.Code, res.Body.String())
	}
	cookies := res.Result().Cookies()
	newCookie := cookies[len(cookies)-1]
	checkAccess(newCookie, 200)
	for _, path := range []string{"/web/api/admin/users", "/web/api/admin/smtp/test", "/web/api/admin/registration"} {
		denied := send(path, `{}`, newCookie)
		if denied.Code != 403 || !strings.Contains(denied.Body.String(), "administrator required") {
			t.Fatalf("role check did not reject %s: %d %s", path, denied.Code, denied.Body.String())
		}
	}
	if res = send("/web/api/auth/password", `{"password":"another-password"}`, userCookie); res.Code != 401 {
		t.Fatalf("old session still valid: %d", res.Code)
	}
	if res = send("/web/api/auth/setup", `{"token":"operator-only-setup-token","email":"second@example.com","password":"correct-horse-battery"}`, nil); res.Code != 403 {
		t.Fatalf("setup replay: %d", res.Code)
	}
}

// firstRead runs after session authentication and before decoding the password.
type firstRead struct {
	io.Reader
	before func()
}

func (r *firstRead) Read(p []byte) (int, error) {
	if r.before != nil {
		f := r.before
		r.before = nil
		f()
	}
	return r.Reader.Read(p)
}

func TestPasswordChangeCannotOverwriteConcurrentReset(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "race.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password-value"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	user := &model.User{Username: "alice", LoginEmail: "alice@example.test", APIKeyHash: "alice", PasswordHash: string(oldHash)}
	if err = model.UserDB.Save(user); err != nil {
		t.Fatal(err)
	}
	cookie, err := middleware.NewSessionCookieForUser(user, "secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "secret"})
	body := &firstRead{Reader: strings.NewReader(`{"password":"replacement-password"}`), before: func() {
		reset := *user
		reset.PasswordHash = "concurrent-reset-hash"
		if err := model.UserDB.Save(&reset); err != nil {
			t.Fatal(err)
		}
	}}
	req := httptest.NewRequest("POST", "/web/api/auth/password", body)
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 409 {
		t.Fatalf("stale password change accepted: %d %s", res.Code, res.Body.String())
	}
	saved, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.PasswordHash != "concurrent-reset-hash" {
		t.Fatal("concurrent reset overwritten")
	}
}

func TestPasswordChangeDoesNotEnableOAuthPasswordLogin(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "oauth.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	user := &model.User{Username: "oauth", APIKeyHash: "oauth", Email: "oauth@example.test"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatal(err)
	}
	cookie, err := middleware.NewSessionCookieForUser(user, "secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "secret"})
	req := httptest.NewRequest("POST", "/web/api/auth/password", strings.NewReader(`{"password":"replacement-password"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 400 {
		t.Fatalf("OAuth password enabled: %d %s", res.Code, res.Body.String())
	}
}

func TestSMTPTestPreservesSiteURLSavedDuringDelivery(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "settings-concurrent.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	admin := &model.User{Username: "admin", Email: "admin@example.test", APIKeyHash: "admin", IsAdmin: true}
	if err := model.UserDB.Save(admin); err != nil {
		t.Fatal(err)
	}
	cookie, err := middleware.NewSessionCookieForUser(admin, "secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	post := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	route.RegisterWithConfig(router, route.Config{SessionSecret: "secret", APIKeyEncryptionSecret: "secret", SendMail: func(context.Context, common.SMTPConfig, string, string) error {
		res := post("/web/api/admin/site-settings", `{"public_base_url":"http://updated.example"}`)
		if res.Code != 200 {
			t.Fatalf("save URL during delivery: %d %s", res.Code, res.Body.String())
		}
		return nil
	}})
	res := post("/web/api/admin/smtp/test", `{"host":"smtp.example.test","port":465,"from":"noreply@example.test","username":"mailer","password":"test-secret","tls_mode":"tls"}`)
	if res.Code != 200 {
		t.Fatalf("SMTP test: %d %s", res.Code, res.Body.String())
	}
	settings, err := model.LoadSiteSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.PublicBaseURL != "http://updated.example" || !settings.Tested || settings.Host != "smtp.example.test" {
		t.Fatalf("SMTP save overwrote concurrent URL update: %+v", settings)
	}
}
