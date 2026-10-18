package route_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/burugo/thing"
	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/api/route"
	"github.com/jieliapp/jieli/backend/model"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
	userservice "github.com/jieliapp/jieli/backend/service/user"
	"golang.org/x/crypto/bcrypt"
)

type adminUsersFixture struct {
	t             *testing.T
	router        *gin.Engine
	owner, target *model.User
	cookie        *http.Cookie
}

func newAdminUsersFixture(t *testing.T) *adminUsersFixture {
	t.Helper()
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "admin-users.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	hash, err := bcrypt.GenerateFromPassword([]byte("original-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	owner := &model.User{Username: "owner", Email: "owner@example.test", LoginEmail: "owner@example.test", PasswordHash: string(hash), IsAdmin: true, APIKeyHash: "owner"}
	target := &model.User{Username: "target", DisplayName: "Target Name", Email: "target@example.test", LoginEmail: "target@example.test", PasswordHash: string(hash), APIKeyHash: "target", GitHubID: "linked", PlanLevel: model.PlanLevelPro}
	for _, u := range []*model.User{owner, target} {
		if err := model.UserDB.Save(u); err != nil {
			t.Fatal(err)
		}
	}
	value, err := middleware.NewSessionCookieForUser(owner, "secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	route.RegisterWithConfig(router, route.Config{SessionSecret: "secret", APIKeyEncryptionSecret: "secret", SelfHosted: true})
	return &adminUsersFixture{t: t, router: router, owner: owner, target: target, cookie: &http.Cookie{Name: middleware.SessionCookieName, Value: value}}
}
func (f *adminUsersFixture) request(method, path, payload string, cookie *http.Cookie) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.12:12345"
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res := httptest.NewRecorder()
	f.router.ServeHTTP(res, req)
	return res
}
func (f *adminUsersFixture) edit(id int64, email, username, name, password string) *httptest.ResponseRecorder {
	f.t.Helper()
	payload, err := json.Marshal(map[string]string{"email": email, "username": username, "display_name": name, "password": password})
	if err != nil {
		f.t.Fatal(err)
	}
	return f.request("PUT", fmt.Sprintf("/web/api/admin/users/%d", id), string(payload), f.cookie)
}
func TestAdminEditUserKeepsBlankPasswordAndRefreshesCachedList(t *testing.T) {
	f := newAdminUsersFixture(t)
	sub := &model.Subscription{UserID: f.target.ID, PlanLevel: model.PlanLevelPro, Status: model.SubscriptionStatusActive, Source: "gift", StartsAtMS: 1000, EndsAtMS: 9000000000000}
	if err := model.SubscriptionDB.Save(sub); err != nil {
		t.Fatal(err)
	}
	if res := f.request("GET", "/web/api/admin/users", "", f.cookie); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	res := f.edit(f.target.ID, "  Renamed@Example.Test  ", " Renamed123 ", "  新名字😀  ", "")
	if res.Code != 200 {
		t.Fatalf("edit: %d %s", res.Code, res.Body.String())
	}
	var body struct{ Data userservice.ProfileUser }
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Email != "renamed@example.test" || body.Data.Username != "renamed123" || body.Data.DisplayName != "新名字😀" || body.Data.MustChangePassword {
		t.Fatalf("unexpected profile: %+v", body.Data)
	}
	list := f.request("GET", "/web/api/admin/users", "", f.cookie)
	if list.Code != 200 || !strings.Contains(list.Body.String(), "renamed123") || strings.Contains(list.Body.String(), "Target Name") {
		t.Fatalf("stale list: %s", list.Body.String())
	}
	login := f.request("POST", "/web/api/auth/login", `{"email":"renamed@example.test","password":"original-password"}`, nil)
	if login.Code != 200 {
		t.Fatalf("blank password changed credentials: %d %s", login.Code, login.Body.String())
	}
	if old := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"original-password"}`, nil); old.Code != 401 {
		t.Fatal("old email still logs in")
	}
	// Verify unrelated persisted ownership/credentials are retained.
	saved, err := model.UserDB.ByID(f.target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.GitHubID != "linked" || saved.APIKeyHash != "target" || saved.PlanLevel != model.PlanLevelPro || saved.IsAdmin {
		t.Fatal("unrelated fields changed")
	}
	retained, err := model.SubscriptionDB.ByID(sub.ID)
	if err != nil || retained.UserID != f.target.ID || retained.Source != "gift" || retained.EndsAtMS != 9000000000000 || retained.Status != model.SubscriptionStatusActive {
		t.Fatal("subscription changed")
	}
	if !saved.EmailManaged {
		t.Fatal("edited email was not protected from OAuth sync")
	}
	for _, key := range []string{"password_hash", "login_email", "original-password", "api_key_hash"} {
		if strings.Contains(res.Body.String(), key) || strings.Contains(list.Body.String(), key) {
			t.Fatalf("sensitive value leaked: %s", key)
		}
	}
}

func TestAdminEditPasswordRequiresChangeAndInvalidatesSessions(t *testing.T) {
	f := newAdminUsersFixture(t)
	original := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"original-password"}`, nil)
	oldCookie := original.Result().Cookies()[0]
	if res := f.edit(f.target.ID, f.target.Email, f.target.Username, f.target.DisplayName, "Ab1!xyzw"); res.Code != 200 || !strings.Contains(res.Body.String(), `"must_change_password":true`) {
		t.Fatalf("reset: %d %s", res.Code, res.Body.String())
	} else {
		assertAdminResponseSafe(t, res.Body.String(), "Ab1!xyzw")
	}
	assertAdminResponseSafe(t, f.request("GET", "/web/api/admin/users", "", f.cookie).Body.String(), "Ab1!xyzw")
	if res := f.request("GET", "/web/api/user/profile", "", oldCookie); res.Code != 401 {
		t.Fatal("reset kept old session")
	}
	login := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"Ab1!xyzw"}`, nil)
	if login.Code != 200 {
		t.Fatalf("reset login: %s", login.Body.String())
	}
	temporary := login.Result().Cookies()[0]
	if res := f.request("GET", "/web/api/user/profile", "", temporary); res.Code != 403 {
		t.Fatal("temporary password gate missing")
	}
	// Profile-only edits preserve an outstanding password-change requirement.
	if res := f.edit(f.target.ID, "later@example.test", f.target.Username, "", ""); res.Code != 200 || !strings.Contains(res.Body.String(), `"must_change_password":true`) {
		t.Fatal("blank password cleared gate")
	}
	if res := f.request("POST", "/web/api/auth/password", `{"password":"personal-password"}`, temporary); res.Code != 200 {
		t.Fatalf("first-login change failed: %s", res.Body.String())
	}
}

func TestAdminEditSelfRenewsSession(t *testing.T) {
	f := newAdminUsersFixture(t)
	res := f.edit(f.owner.ID, "newowner@example.test", "newowner", "Owner Updated", "Ab1!xyzw")
	if res.Code != 200 || strings.Contains(res.Body.String(), `"must_change_password":true`) {
		t.Fatalf("self edit: %d %s", res.Code, res.Body.String())
	}
	assertAdminResponseSafe(t, res.Body.String(), "Ab1!xyzw")
	cookies := res.Result().Cookies()
	fresh := cookies[len(cookies)-1]
	if res := f.request("GET", "/web/api/admin/users", "", fresh); res.Code != 200 {
		t.Fatalf("renewed session failed: %s", res.Body.String())
	} else {
		assertAdminResponseSafe(t, res.Body.String(), "Ab1!xyzw")
	}
	if res := f.request("GET", "/web/api/admin/users", "", f.cookie); res.Code != 401 {
		t.Fatal("self reset retained old session")
	}
}

func TestAdminEditValidationAndPermissionAreAtomic(t *testing.T) {
	f := newAdminUsersFixture(t)
	path := fmt.Sprintf("/web/api/admin/users/%d", f.target.ID)
	if res := f.request("PUT", path, `{}`, nil); res.Code != 401 {
		t.Fatal("anonymous edit permitted")
	}
	value, err := middleware.NewSessionCookieForUser(f.target, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if res := f.request("PUT", path, `{}`, &http.Cookie{Name: middleware.SessionCookieName, Value: value}); res.Code != 403 {
		t.Fatal("nonadmin edit permitted")
	}
	for _, id := range []string{"0", "-1", "nope", "999999999999999999999999999"} {
		if res := f.request("PUT", "/web/api/admin/users/"+id, `{}`, f.cookie); res.Code != 400 {
			t.Fatalf("invalid ID %s: %d", id, res.Code)
		}
	}
	if res := f.edit(99999, f.target.Email, f.target.Username, "", ""); res.Code != 404 {
		t.Fatalf("absent target: %d %s", res.Code, res.Body.String())
	}
	for _, payload := range []string{`{`, `{}`, `{"email":"good@example.test","username":"validname"}`, `{"email":"good@example.test","username":"validname","display_name":null}`} {
		if res := f.request("PUT", path, payload, f.cookie); res.Code != 400 {
			t.Fatalf("invalid body accepted: %s", payload)
		}
	}
	for _, invalid := range []map[string]any{
		{"username": "validname", "display_name": "Name"},
		{"email": "good@example.test", "display_name": "Name"},
	} {
		payload, _ := json.Marshal(invalid)
		if res := f.request("PUT", path, string(payload), f.cookie); res.Code != 400 {
			t.Fatal("missing required field accepted")
		}
	}
	cases := []struct {
		name, email, username, display, password string
		status                                   int
	}{
		{"invalid email", "broken", "validname", "Changed", "", 400},
		{"duplicate email", f.owner.Email, "validname", "Changed", "Ab1!xyzw", 409},
		{"duplicate username", "valid@example.test", f.owner.Username, "Changed", "Ab1!xyzw", 409},
		{"reserved username", "valid@example.test", "admin", "Changed", "", 400},
		{"invalid username", "valid@example.test", "bad_name", "Changed", "", 400},
		{"short username", "valid@example.test", "ab", "Changed", "", 400},
		{"long username", "valid@example.test", strings.Repeat("a", 33), "Changed", "", 400},
		{"long name", "valid@example.test", "validname", strings.Repeat("名", 81), "", 400},
		{"short password", "valid@example.test", "validname", "Changed", "Ab1!xyz", 400},
		{"long password", "valid@example.test", "validname", "Changed", strings.Repeat("a", 73), 400},
		{"space password", "valid@example.test", "validname", "Changed", "pass word", 400},
		{"unicode password", "valid@example.test", "validname", "Changed", "密码abcdef", 400},
		{"control password", "valid@example.test", "validname", "Changed", "abcdefg\n", 400},
		{"DEL password", "valid@example.test", "validname", "Changed", "abcdefg\x7f", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := f.edit(f.target.ID, tc.email, tc.username, tc.display, tc.password)
			if res.Code != tc.status {
				t.Fatalf("want %d: %d %s", tc.status, res.Code, res.Body.String())
			}
			list := f.request("GET", "/web/api/admin/users", "", f.cookie)
			if !strings.Contains(list.Body.String(), `"username":"target"`) || !strings.Contains(list.Body.String(), `"display_name":"Target Name"`) || !strings.Contains(list.Body.String(), f.target.Email) {
				t.Fatal("failed edit partially applied")
			}
		})
	}
	if res := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"original-password"}`, nil); res.Code != 200 {
		t.Fatal("failed edit changed credentials")
	}
	if res := f.edit(f.target.ID, f.target.Email, f.target.Username, strings.Repeat("名😀", 40), strings.Repeat("a", 72)); res.Code != 200 {
		t.Fatalf("valid upper limits rejected: %s", res.Body.String())
	}
	for _, username := range []string{"abc", strings.Repeat("z", 32)} {
		if res := f.edit(f.target.ID, f.target.Email, username, "", ""); res.Code != 200 {
			t.Fatalf("valid username length %d rejected: %s", len(username), res.Body.String())
		}
	}
}

func assertAdminResponseSafe(t *testing.T, body, password string) {
	t.Helper()
	for _, sensitive := range []string{password, "password_hash", "login_email", "api_key_hash", "api_key_encrypted", "email_managed", "$2a$", "$2b$"} {
		if strings.Contains(body, sensitive) {
			t.Fatalf("administrative response leaks %q", sensitive)
		}
	}
}

func TestAdminEditWriteFailureRollsBackAndKeepsCacheConsistent(t *testing.T) {
	f := newAdminUsersFixture(t)
	f.request("GET", "/web/api/admin/users", "", f.cookie)
	if err := thing.Transaction(context.Background(), func(tx *thing.TxSession) error {
		_, err := tx.Exec(fmt.Sprintf("CREATE TRIGGER reject_admin_edit BEFORE UPDATE ON users WHEN NEW.id = %d BEGIN SELECT RAISE(ABORT, 'fixture write failure'); END", f.target.ID))
		tx.Touch("users")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	res := f.edit(f.target.ID, "changed@example.test", "changedname", "Changed Name", "Ab1!xyzw")
	if res.Code != 500 || !strings.Contains(res.Body.String(), "user update failed") {
		t.Fatalf("write failure: %d %s", res.Code, res.Body.String())
	}
	list := f.request("GET", "/web/api/admin/users", "", f.cookie)
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"username":"target"`) || strings.Contains(list.Body.String(), "Changed Name") {
		t.Fatal("failed write corrupted list cache")
	}
	for _, cached := range []bool{true, false} {
		var saved *model.User
		var err error
		if cached {
			saved, err = model.UserDB.ByID(f.target.ID)
		} else {
			err = thing.Transaction(context.Background(), func(tx *thing.TxSession) error {
				users, e := thing.UseTx[*model.User](tx)
				if e != nil {
					return e
				}
				saved, e = users.ByID(f.target.ID)
				return e
			})
		}
		if err != nil {
			t.Fatal(err)
		}
		if saved.Email != f.target.Email || saved.Username != f.target.Username || saved.DisplayName != f.target.DisplayName || saved.PasswordHash != f.target.PasswordHash || saved.MustChangePassword || saved.EmailManaged {
			t.Fatal("failed write partially persisted or cached")
		}
	}
	if login := f.request("POST", "/web/api/auth/login", `{"email":"target@example.test","password":"original-password"}`, nil); login.Code != 200 {
		t.Fatal("failed write changed login credentials")
	}
}

func TestAdminEditOAuthLoginEligibility(t *testing.T) {
	f := newAdminUsersFixture(t)
	oauth := &model.User{Username: "MixedCaseStored", Email: "oauth@example.test", GitHubID: "24680", APIKeyHash: "oauth"}
	if err := model.UserDB.Save(oauth); err != nil {
		t.Fatal(err)
	}
	if res := f.edit(oauth.ID, "updated@example.test", oauth.Username, "OAuth Updated", ""); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	saved, err := model.UserDB.ByID(oauth.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LoginEmail != "" || saved.PasswordHash != "" || saved.GitHubID != "24680" || saved.Username != oauth.Username || !saved.EmailManaged {
		t.Fatal("profile edit changed OAuth eligibility or stored username")
	}
	provider := authservice.NewGitHubOAuthService(adminEditGitHubClient{})
	returned, _, err := provider.Authenticate("provider-code")
	if err != nil || returned.Email != "updated@example.test" {
		t.Fatal("OAuth sign-in replaced the administrator email")
	}
	if res := f.request("POST", "/web/api/auth/login", `{"email":"updated@example.test","password":"original-password"}`, nil); res.Code != 401 {
		t.Fatal("OAuth user got password login without a password")
	}
	if res := f.edit(oauth.ID, "updated@example.test", "oauthnew", "OAuth Updated", "Ab1!xyzw"); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	login := f.request("POST", "/web/api/auth/login", `{"email":"updated@example.test","password":"Ab1!xyzw"}`, nil)
	if login.Code != 200 || !strings.Contains(login.Body.String(), `"must_change_password":true`) {
		t.Fatalf("OAuth password login failed: %s", login.Body.String())
	}
	// A login email can differ from the display email; it is still unavailable.
	other := &model.User{Username: "differentemail", Email: "display@example.test", LoginEmail: "occupied@example.test", APIKeyHash: "different"}
	if err := model.UserDB.Save(other); err != nil {
		t.Fatal(err)
	}
	if res := f.edit(oauth.ID, other.LoginEmail, "oauthnew", "Changed", ""); res.Code != 409 {
		t.Fatal("existing login email not protected")
	}
}

type adminEditGitHubClient struct{}

func (adminEditGitHubClient) UserByCode(string) (authservice.GitHubUser, error) {
	return authservice.GitHubUser{ID: 24680, Login: "providername", Email: "provider@example.test", Name: "Provider Name"}, nil
}

func TestConcurrentAdminEditsCannotClaimSameEmailOrUsername(t *testing.T) {
	for _, claim := range []string{"email", "username"} {
		t.Run(claim, func(t *testing.T) {
			f := newAdminUsersFixture(t)
			second := &model.User{Username: "secondtarget", DisplayName: "Second Target", Email: "second@example.test", APIKeyHash: "second"}
			if err := model.UserDB.Save(second); err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			type result struct {
				user     *model.User
				response *httptest.ResponseRecorder
			}
			results := make(chan result, 2)
			for index, user := range []*model.User{f.target, second} {
				go func(index int, user *model.User) {
					<-start
					email := fmt.Sprintf("claimed%d@example.test", index)
					username := fmt.Sprintf("claimeduser%d", index)
					if claim == "email" {
						email = "shared@example.test"
					} else {
						username = "sharedusername"
					}
					results <- result{user: user, response: f.edit(user.ID, email, username, "New Name", "")}
				}(index, user)
			}
			close(start)
			a, b := <-results, <-results
			if a.response.Code == 409 {
				a, b = b, a
			}
			if a.response.Code != 200 || b.response.Code != 409 {
				t.Fatalf("concurrent claim: %d/%d %s %s", a.response.Code, b.response.Code, a.response.Body.String(), b.response.Body.String())
			}
			list := f.request("GET", "/web/api/admin/users", "", f.cookie)
			var users struct{ Data []userservice.ProfileUser }
			if err := json.Unmarshal(list.Body.Bytes(), &users); err != nil {
				t.Fatal(err)
			}
			occupied := 0
			for _, user := range users.Data {
				if claim == "email" && user.Email == "shared@example.test" || claim == "username" && user.Username == "sharedusername" {
					occupied++
				}
				if user.ID == b.user.ID && (user.Email != b.user.Email || user.Username != b.user.Username || user.DisplayName != b.user.DisplayName) {
					t.Fatal("losing edit partially applied")
				}
			}
			if occupied != 1 {
				t.Fatalf("shared claim count: %d", occupied)
			}
		})
	}
}
