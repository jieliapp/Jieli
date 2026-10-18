package route_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/burugo/thing"
	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/route"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
)

func TestSelfHostedFirstAdministrator(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "setup.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	// A previously configured setting must survive initialization; registration
	// must be explicitly closed even if it was enabled before setup.
	if err := model.UpdateSiteSettings(context.Background(), func(s *model.SiteSettingsValues) error {
		s.PublicBaseURL = "https://example.test"
		s.RegistrationEnabled = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SelfHosted: true, SessionSecret: "setup-test", APIKeyEncryptionSecret: "setup-test", SendMail: func(context.Context, common.SMTPConfig, string, string) error {
		t.Error("first administrator must not send verification mail")
		return nil
	}})
	var requests atomic.Int32
	send := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.RemoteAddr = fmt.Sprintf("192.0.2.%d:12345", requests.Add(1))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	options := send("GET", "/web/api/auth/options", "", nil)
	if options.Code != 200 || !strings.Contains(options.Body.String(), `"setup_available":true`) {
		t.Fatalf("fresh installation must offer setup without a token: %d %s", options.Code, options.Body.String())
	}
	for _, body := range []string{
		`{"email":"owner@example.com","password":"owner-password"}`,
		`{"email":"owner@example.com","password":"owner-password","password_confirm":"different"}`,
		`{"email":"invalid","password":"owner-password","password_confirm":"owner-password"}`,
		`{"email":"owner@example.com","password":"short","password_confirm":"short"}`,
	} {
		if res := send("POST", "/web/api/auth/setup", body, nil); res.Code != 400 {
			t.Fatalf("invalid setup must be rejected without consuming setup: %d %s", res.Code, res.Body.String())
		}
	}
	body := `{"email":"owner@example.com","password":"owner-password","password_confirm":"owner-password"}`
	// A real SQLite write failure must roll back the new administrator too.
	if err := thing.Transaction(context.Background(), func(tx *thing.TxSession) error {
		_, err := tx.Exec("CREATE TRIGGER reject_setup_registration BEFORE UPDATE ON site_settings WHEN NEW.name = 'registration_enabled' BEGIN SELECT RAISE(ABORT, 'test settings failure'); END")
		tx.Touch("site_settings")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if res := send("POST", "/web/api/auth/setup", body, nil); res.Code == 200 || len(res.Result().Cookies()) != 0 {
		t.Fatalf("failed settings write must not issue a session: %d %s", res.Code, res.Body.String())
	}
	options = send("GET", "/web/api/auth/options", "", nil)
	if options.Code != 200 || !strings.Contains(options.Body.String(), `"setup_available":true`) || !strings.Contains(options.Body.String(), `"registration_enabled":true`) {
		t.Fatalf("failed initialization did not roll back: %s", options.Body.String())
	}
	if err := thing.Transaction(context.Background(), func(tx *thing.TxSession) error {
		_, err := tx.Exec("DROP TRIGGER reject_setup_registration")
		tx.Touch("site_settings")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// Two clients submit against the same uninitialized instance.
	results := make(chan *httptest.ResponseRecorder, 2)
	for _, email := range []string{"owner@example.com", "other@example.com"} {
		go func() {
			results <- send("POST", "/web/api/auth/setup", strings.Replace(body, "owner@example.com", email, 1), nil)
		}()
	}
	var cookie *http.Cookie
	var ownerEmail string
	winners := 0
	for i := 0; i < 2; i++ {
		res := <-results
		switch res.Code {
		case 200:
			winners++
			if len(res.Result().Cookies()) == 0 {
				t.Fatal("administrator setup did not create a session")
			}
			cookie = res.Result().Cookies()[0]
			var result struct {
				Data struct {
					Email string `json:"email"`
				}
			}
			if err := json.Unmarshal(res.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			ownerEmail = result.Data.Email
		case 403:
		default:
			t.Fatalf("setup: %d %s", res.Code, res.Body.String())
		}
	}
	if winners != 1 {
		t.Fatalf("expected exactly one administrator initialization, got %d", winners)
	}
	res := send("GET", "/web/api/admin/users", "", cookie)
	var users struct {
		Data []struct {
			IsAdmin bool `json:"is_admin"`
		}
	}
	if err := json.Unmarshal(res.Body.Bytes(), &users); err != nil || res.Code != 200 || len(users.Data) != 1 || !users.Data[0].IsAdmin {
		t.Fatalf("setup session must have administrator access: %d %s", res.Code, res.Body.String())
	}
	if res = send("POST", "/web/api/auth/login", fmt.Sprintf(`{"email":%q,"password":"owner-password"}`, ownerEmail), nil); res.Code != 200 {
		t.Fatalf("administrator must log in without verification: %d %s", res.Code, res.Body.String())
	}
	options = send("GET", "/web/api/auth/options", "", nil)
	if options.Code != 200 || !strings.Contains(options.Body.String(), `"setup_available":false`) || !strings.Contains(options.Body.String(), `"registration_enabled":false`) {
		t.Fatalf("setup and ordinary registration must now be closed: %s", options.Body.String())
	}
	if res = send("GET", "/web/api/admin/site-settings", "", cookie); res.Code != 200 || !strings.Contains(res.Body.String(), "https://example.test") {
		t.Fatalf("initialization overwrote unrelated settings: %s", res.Body.String())
	}
	if res = send("POST", "/web/api/auth/register", `{"email":"next@example.com","password":"owner-password"}`, nil); res.Code != 403 {
		t.Fatalf("ordinary registration remained open: %d %s", res.Code, res.Body.String())
	}
	// Simulate a disabled administrator at the persistence boundary. Normal UI
	// prevents administrators disabling themselves; this also covers maintenance.
	rows, err := model.UserDB.Where("is_admin = ?", true).Fetch(0, 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("administrator missing: %v", err)
	}
	if err = model.UserDB.SoftDelete(rows[0]); err != nil {
		t.Fatal(err)
	}
	options = send("GET", "/web/api/auth/options", "", nil)
	if options.Code != 200 || !strings.Contains(options.Body.String(), `"setup_available":false`) {
		t.Fatalf("disabled administrator reopened setup: %s", options.Body.String())
	}
	if res = send("POST", "/web/api/auth/setup", `{"email":"replacement@example.com","password":"owner-password","password_confirm":"owner-password"}`, nil); res.Code != 403 {
		t.Fatalf("disabled administrator allowed reinitialization: %d %s", res.Code, res.Body.String())
	}
}
