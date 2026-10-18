package route_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/burugo/thing"
	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/route"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
)

func TestAdminInactiveUserRejectsWarmSessionsAndKeysAndCanRestore(t *testing.T) {
	f := newAdminUsersFixture(t)
	credential, err := common.NewAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	f.target.APIKeyHash = credential.Hash
	f.target.APIKeyPrefix = credential.Prefix
	f.target.IsAdmin = true
	if err := model.UserDB.Save(f.target); err != nil {
		t.Fatal(err)
	}
	login := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"original-password"}`, nil)
	if login.Code != 200 {
		t.Fatal(login.Body.String())
	}
	cookie := login.Result().Cookies()[0]
	if res := f.request("GET", "/web/api/user/profile", "", cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	plugin := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+credential.Plaintext)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		f.router.ServeHTTP(res, req)
		return res
	}
	if res := plugin("GET", "/plugin/threads"); res.Code != 200 {
		t.Fatalf("warm API key: %d %s", res.Code, res.Body.String())
	}
	thread := &model.Thread{UserID: f.target.ID, Provider: model.ProviderCodex, ProviderThreadID: "inactive-test", Title: "History", Visibility: model.ThreadVisibilityPrivate}
	if err := model.ThreadDB.Save(thread); err != nil {
		t.Fatal(err)
	}
	shared := &model.Thread{UserID: f.target.ID, Provider: model.ProviderCodex, ProviderThreadID: "inactive-shared", Title: "Shared history", Visibility: model.ThreadVisibilityUnlisted}
	if err := model.ThreadDB.Save(shared); err != nil {
		t.Fatal(err)
	}
	if res := f.request("GET", "/web/api/threads/inactive-shared", "", nil); res.Code != 200 {
		t.Fatal("shared fixture was not public before disable")
	}
	subscription := &model.Subscription{UserID: f.target.ID, PlanLevel: model.PlanLevelPro, Status: model.SubscriptionStatusActive, EndsAtMS: 9000000000000, Source: "gift"}
	if err := model.SubscriptionDB.Save(subscription); err != nil {
		t.Fatal(err)
	}
	attachment := &model.Attachment{UserID: f.target.ID, PublicID: "inactive-attachment", StorageKey: "retained-file", StorageDriver: "local", MediaType: "text/plain"}
	if err := model.AttachmentDB.Save(attachment); err != nil {
		t.Fatal(err)
	}
	f.request("GET", "/web/api/admin/users", "", f.cookie)
	path := fmt.Sprintf("/web/api/admin/users/%d/inactive", f.target.ID)
	res := f.request("PUT", path, `{"inactive":true}`, f.cookie)
	if res.Code != 200 {
		t.Fatalf("disable: %d %s", res.Code, res.Body.String())
	}
	if res := f.request("GET", "/web/api/threads/inactive-shared", "", nil); res.Code != 200 || !strings.Contains(res.Body.String(), "Shared history") {
		t.Fatal("disable changed shared-history access")
	}
	if res := f.request("GET", "/web/api/threads/inactive-test", "", nil); res.Code != 404 {
		t.Fatal("disable exposed private history")
	}
	for _, endpoint := range []string{"profile", "activity"} {
		if res := f.request("GET", "/web/api/users/target/"+endpoint, "", nil); res.Code != 404 {
			t.Fatal("soft-deleted user remains publicly discoverable")
		}
	}
	for _, path := range []string{"/plugin/threads/upload", "/plugin/threads/archive", "/plugin/attachments"} {
		if res := plugin("POST", path); res.Code != 401 {
			t.Fatalf("disabled plugin %s: %d %s", path, res.Code, res.Body.String())
		}
	}
	for _, op := range []struct{ method, path string }{{"GET", "/web/api/auth/me"}, {"PATCH", "/web/api/user/profile"}, {"POST", "/web/api/user/api-key/rotate"}, {"PATCH", "/web/api/threads/123/meta"}, {"POST", "/web/api/threads/123/archive"}, {"DELETE", "/web/api/threads/123"}, {"POST", "/web/api/auth/password"}} {
		if res := f.request(op.method, op.path, `{}`, cookie); res.Code != 401 {
			t.Fatalf("disabled session %s: %d %s", op.path, res.Code, res.Body.String())
		}
	}
	list := f.request("GET", "/web/api/admin/users", "", f.cookie)
	var body struct {
		Data []struct {
			ID       int64
			Inactive bool
		}
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, u := range body.Data {
		if u.ID == f.target.ID {
			found = u.Inactive
		}
	}
	if !found {
		t.Fatal("inactive user missing from administrative list")
	}
	if res := f.request("PUT", path, `{"inactive":true}`, f.cookie); res.Code != 200 {
		t.Fatal("repeat disable failed")
	}
	if res := f.edit(f.target.ID, f.target.Email, f.target.Username, "Changed", ""); res.Code != 400 {
		t.Fatal("inactive credentials remained editable")
	}
	if res := f.request("GET", "/web/api/user/profile", "", cookie); res.Code != 401 {
		t.Fatalf("disabled session: %d", res.Code)
	}
	if res := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"original-password"}`, nil); res.Code != 401 {
		t.Fatalf("disabled login: %d", res.Code)
	}
	if res := f.request("PUT", path, `{"inactive":false}`, f.cookie); res.Code != 200 {
		t.Fatalf("restore: %d %s", res.Code, res.Body.String())
	}
	if res := f.request("PUT", path, `{"inactive":false}`, f.cookie); res.Code != 200 {
		t.Fatal("repeat restore failed")
	}
	if res := f.request("GET", "/web/api/user/profile", "", cookie); res.Code != 200 {
		t.Fatalf("restored session: %d %s", res.Code, res.Body.String())
	}
	if res := plugin("GET", "/plugin/threads"); res.Code != 200 {
		t.Fatalf("restored key: %d", res.Code)
	}
	saved, err := model.UserDB.ByID(f.target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Deleted || saved.IsAdmin != f.target.IsAdmin || saved.LoginEmail != f.target.LoginEmail || saved.Username != f.target.Username || saved.Email != f.target.Email || saved.APIKeyHash != credential.Hash || saved.PasswordHash != f.target.PasswordHash || saved.GitHubID != f.target.GitHubID || saved.PlanLevel != f.target.PlanLevel {
		t.Fatal("disable/restore changed identity or credentials")
	}
	storedAttachment, err := model.AttachmentDB.ByID(attachment.ID)
	if err != nil || storedAttachment.Deleted || storedAttachment.StorageKey != "retained-file" {
		t.Fatal("attachment changed")
	}
	storedThread, err := model.ThreadDB.ByID(thread.ID)
	if err != nil || storedThread.Title != "History" || storedThread.Deleted {
		t.Fatal("history changed")
	}
	storedSub, err := model.SubscriptionDB.ByID(subscription.ID)
	if err != nil || storedSub.Status != model.SubscriptionStatusActive {
		t.Fatal("subscription changed")
	}
}

func TestAdminInactiveValidationPermissionsAndReservedIdentity(t *testing.T) {
	f := newAdminUsersFixture(t)
	path := fmt.Sprintf("/web/api/admin/users/%d/inactive", f.target.ID)
	if res := f.request("PUT", path, `{"inactive":true}`, nil); res.Code != 401 {
		t.Fatal("anonymous disabled user")
	}
	login := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"original-password"}`, nil)
	if res := f.request("PUT", path, `{"inactive":true}`, login.Result().Cookies()[0]); res.Code != 403 {
		t.Fatal("ordinary user disabled user")
	}
	for _, payload := range []string{`{}`, `{"inactive":null}`, `{"inactive":"true"}`, `{"inactive":1}`, `{`} {
		if res := f.request("PUT", path, payload, f.cookie); res.Code != 400 {
			t.Fatalf("invalid body accepted: %s", payload)
		}
	}
	for _, id := range []string{"0", "-1", "bad", "99999999999999999999999999"} {
		if res := f.request("PUT", "/web/api/admin/users/"+id+"/inactive", `{"inactive":true}`, f.cookie); res.Code != 400 {
			t.Fatal("invalid target accepted")
		}
	}
	if res := f.request("PUT", "/web/api/admin/users/99999/inactive", `{"inactive":true}`, f.cookie); res.Code != 404 {
		t.Fatal("missing target accepted")
	}
	if res := f.request("PUT", fmt.Sprintf("/web/api/admin/users/%d/inactive", f.owner.ID), `{"inactive":true}`, f.cookie); res.Code != 400 {
		t.Fatal("administrator disabled self")
	}
	f.target.Email = "display@example.test"
	if err := model.UserDB.Save(f.target); err != nil {
		t.Fatal(err)
	}
	if res := f.request("PUT", path, `{"inactive":true}`, f.cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	for _, email := range []string{f.target.Email, f.target.LoginEmail} {
		payload, _ := json.Marshal(map[string]string{"email": email, "password": "Ab1!xyzw"})
		if res := f.request("POST", "/web/api/admin/users", string(payload), f.cookie); res.Code != 400 {
			t.Fatalf("inactive email reused: %d %s", res.Code, res.Body.String())
		}
	}
	if res := f.edit(f.owner.ID, "owner@example.test", "target", "Owner", ""); res.Code != 409 {
		t.Fatal("inactive username reused")
	}
}

func TestInactivePasswordResetTokenCannotWriteOrBeConsumed(t *testing.T) {
	f := newAdminUsersFixture(t)
	token := strings.Repeat("r", 64)
	digest := sha256.Sum256([]byte(token))
	stored := &model.EmailToken{TokenHash: hex.EncodeToString(digest[:]), Email: f.target.Email, Purpose: "reset", ExpiresAtMS: time.Now().Add(time.Hour).UnixMilli()}
	tokens, err := thing.Use[*model.EmailToken]()
	if err != nil {
		t.Fatal(err)
	}
	if err := tokens.Save(stored); err != nil {
		t.Fatal(err)
	}
	if res := f.request("PUT", fmt.Sprintf("/web/api/admin/users/%d/inactive", f.target.ID), `{"inactive":true}`, f.cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	payload, err := json.Marshal(map[string]string{"token": token, "password": "replacement-password"})
	if err != nil {
		t.Fatal(err)
	}
	if res := f.request("POST", "/web/api/auth/email/complete", string(payload), nil); res.Code != 401 {
		t.Fatalf("inactive reset: %d %s", res.Code, res.Body.String())
	}
	saved, err := model.UserDB.ByID(f.target.ID)
	if err != nil || saved.PasswordHash != f.target.PasswordHash {
		t.Fatal("inactive password changed")
	}
	retained, err := tokens.ByID(stored.ID)
	if err != nil || retained.Used {
		t.Fatal("inactive reset consumed token")
	}
}

func TestInactiveOAuthAndDevHTTPLoginReturnsUnauthorized(t *testing.T) {
	for _, scenario := range []string{"github", "google", "dev", "github-email", "google-email", "github-login-email", "google-login-email"} {
		t.Run(scenario, func(t *testing.T) {
			provider := strings.SplitN(scenario, "-", 2)[0]
			f := newAdminUsersFixture(t)
			switch scenario {
			case "github":
				f.target.GitHubID = "24680"
			case "google":
				f.target.GoogleID = "inactive-google"
			case "dev":
				f.target.GitHubID = authservice.DevUserGitHubID
			case "github-email":
				f.target.Email = "provider@example.test"
			case "google-email":
				f.target.Email = "replacement@example.test"
			case "github-login-email":
				f.target.LoginEmail = "provider@example.test"
			case "google-login-email":
				f.target.LoginEmail = "replacement@example.test"
			}
			if err := model.UserDB.Save(f.target); err != nil {
				t.Fatal(err)
			}
			if res := f.request("PUT", fmt.Sprintf("/web/api/admin/users/%d/inactive", f.target.ID), `{"inactive":true}`, f.cookie); res.Code != 200 {
				t.Fatal(res.Body.String())
			}
			router := gin.New()
			route.RegisterWithConfig(router, route.Config{SessionSecret: "secret", DevMode: true,
				GitHub: common.GitHubConfig{ClientID: "id", ClientSecret: "secret", CallbackURL: "http://localhost/web/api/auth/callback/github", PostLoginRedirect: "/web/dashboard"}, GitHubClient: adminEditGitHubClient{},
				Google: common.GoogleConfig{ClientID: "id", ClientSecret: "secret", CallbackURL: "http://localhost/web/api/auth/callback/google", PostLoginRedirect: "/web/dashboard"}, GoogleClient: inactiveGoogleClient{},
			})
			f.router = router
			var res *httptest.ResponseRecorder
			if provider == "dev" {
				res = f.request("GET", "/web/api/auth/dev-login", "", nil)
			} else {
				start := f.request("GET", "/web/api/auth/"+provider, "", nil)
				if start.Code != 307 {
					t.Fatal(start.Body.String())
				}
				location, err := url.Parse(start.Header().Get("Location"))
				if err != nil {
					t.Fatal(err)
				}
				res = f.request("GET", "/web/api/auth/callback/"+provider+"?code=provider-code&state="+url.QueryEscape(location.Query().Get("state")), "", start.Result().Cookies()[0])
			}
			if res.Code != 401 {
				t.Fatalf("disabled %s login: %d %s", provider, res.Code, res.Body.String())
			}
			count, err := model.UserDB.Where("id > ?", 0).WithDeleted().Count()
			if err != nil || count != 2 {
				t.Fatal("login provisioned replacement")
			}
		})
	}
}

type inactiveGoogleClient struct{}

func (inactiveGoogleClient) UserByCode(string) (authservice.GoogleUser, error) {
	return authservice.GoogleUser{ID: "inactive-google", Email: "replacement@example.test", Name: "Replacement"}, nil
}

func TestAdminInactiveWriteFailurePreservesStateAndCache(t *testing.T) {
	for _, initiallyInactive := range []bool{false, true} {
		t.Run(fmt.Sprintf("initially-inactive-%t", initiallyInactive), func(t *testing.T) {
			f := newAdminUsersFixture(t)
			login := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"original-password"}`, nil)
			if login.Code != 200 {
				t.Fatal(login.Body.String())
			}
			cookie := login.Result().Cookies()[0]
			path := fmt.Sprintf("/web/api/admin/users/%d/inactive", f.target.ID)
			if initiallyInactive {
				if res := f.request("PUT", path, `{"inactive":true}`, f.cookie); res.Code != 200 {
					t.Fatal(res.Body.String())
				}
			}
			f.request("GET", "/web/api/admin/users", "", f.cookie)
			if _, err := model.UserDB.ByID(f.target.ID); err != nil {
				t.Fatal(err)
			}
			if err := thing.Transaction(context.Background(), func(tx *thing.TxSession) error {
				_, err := tx.Exec(fmt.Sprintf("CREATE TRIGGER reject_inactive_write BEFORE UPDATE ON users WHEN NEW.id = %d BEGIN SELECT RAISE(ABORT, 'fixture write failure'); END", f.target.ID))
				tx.Touch("users")
				return err
			}); err != nil {
				t.Fatal(err)
			}
			payload := fmt.Sprintf(`{"inactive":%t}`, !initiallyInactive)
			res := f.request("PUT", path, payload, f.cookie)
			if res.Code != 500 || !strings.Contains(res.Body.String(), "user status update failed") {
				t.Fatalf("failed status write: %d %s", res.Code, res.Body.String())
			}
			for _, fresh := range []bool{false, true} {
				var saved *model.User
				var err error
				if fresh {
					err = thing.Transaction(context.Background(), func(tx *thing.TxSession) error {
						users, e := thing.UseTx[*model.User](tx)
						if e != nil {
							return e
						}
						saved, e = users.ByID(f.target.ID)
						return e
					})
				} else {
					saved, err = model.UserDB.ByID(f.target.ID)
				}
				if err != nil || saved.Deleted != initiallyInactive || saved.PasswordHash != f.target.PasswordHash {
					t.Fatal("failed write changed cached or stored state")
				}
			}
			list := f.request("GET", "/web/api/admin/users", "", f.cookie)
			var body struct {
				Data []struct {
					ID       int64
					Inactive bool
				}
			}
			if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil || list.Code != 200 {
				t.Fatal("failed to read administrative list")
			}
			found := false
			for _, user := range body.Data {
				if user.ID == f.target.ID {
					found = true
					if user.Inactive != initiallyInactive {
						t.Fatal("failed write changed list cache")
					}
				}
			}
			if !found {
				t.Fatal("failed write removed user from list")
			}
			want := 200
			if initiallyInactive {
				want = 401
			}
			if res := f.request("GET", "/web/api/user/profile", "", cookie); res.Code != want {
				t.Fatalf("failed write changed access: %d", res.Code)
			}
		})
	}
}

func TestInactiveEmailCannotRegisterOrRequestRecovery(t *testing.T) {
	f := newAdminUsersFixture(t)
	f.target.Email = "display@example.test"
	if err := model.UserDB.Save(f.target); err != nil {
		t.Fatal(err)
	}
	sent := 0
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "secret", APIKeyEncryptionSecret: "secret", SendMail: func(context.Context, common.SMTPConfig, string, string) error {
		sent++
		return nil
	}})
	f.router = router
	if res := f.request("POST", "/web/api/admin/smtp/test", `{"host":"smtp.example.test","port":465,"from":"noreply@example.test","username":"mailer","password":"fixture-secret","tls_mode":"tls"}`, f.cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res := f.request("POST", "/web/api/admin/site-settings", `{"public_base_url":"http://example.test"}`, f.cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if res := f.request("POST", "/web/api/admin/registration", `{"enabled":true}`, f.cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	// Represent a registration token issued before this identity was claimed.
	token := strings.Repeat("v", 64)
	digest := sha256.Sum256([]byte(token))
	stored := &model.EmailToken{TokenHash: hex.EncodeToString(digest[:]), Email: f.target.LoginEmail, Purpose: "register", PasswordHash: f.target.PasswordHash, ExpiresAtMS: time.Now().Add(time.Hour).UnixMilli()}
	tokens, err := thing.Use[*model.EmailToken]()
	if err != nil {
		t.Fatal(err)
	}
	if err := tokens.Save(stored); err != nil {
		t.Fatal(err)
	}
	if res := f.request("PUT", fmt.Sprintf("/web/api/admin/users/%d/inactive", f.target.ID), `{"inactive":true}`, f.cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	for _, path := range []string{"/web/api/auth/register", "/web/api/auth/reset"} {
		for _, email := range []string{f.target.Email, f.target.LoginEmail} {
			payload, _ := json.Marshal(map[string]string{"email": email, "password": "Ab1!xyzw"})
			if res := f.request("POST", path, string(payload), nil); res.Code != 200 {
				t.Fatalf("opaque request: %d %s", res.Code, res.Body.String())
			}
		}
	}
	if sent != 1 {
		t.Fatal("inactive identity received verification or recovery mail")
	}
	if count, err := tokens.Where("id > ?", 0).Count(); err != nil || count != 1 {
		t.Fatal("inactive identity received a new token")
	}
	payload, _ := json.Marshal(map[string]string{"token": token})
	if res := f.request("POST", "/web/api/auth/email/complete", string(payload), nil); res.Code != 400 {
		t.Fatalf("existing registration token bypassed reserved email: %d %s", res.Code, res.Body.String())
	}
	if count, err := model.UserDB.Where("id > ?", 0).WithDeleted().Count(); err != nil || count != 2 {
		t.Fatal("registration provisioned a replacement")
	}
	if retained, err := tokens.ByID(stored.ID); err != nil || retained.Used {
		t.Fatal("rejected registration consumed token")
	}
}
