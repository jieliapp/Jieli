package route_test

import (
	"context"
	"encoding/json"
	"fmt"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
	"github.com/jieliapp/jieli/backend/service/subscription"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/api/route"
	"github.com/jieliapp/jieli/backend/model"
)

func TestDeploymentPolicyDoesNotGrantSyntheticPro(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	user := &model.User{Username: "alice", APIKeyHash: "alice", PlanLevel: model.PlanLevelFree}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatal(err)
	}
	cookie, err := middleware.NewSessionCookie(user.ID, "secret")
	if err != nil {
		t.Fatal(err)
	}
	for _, selfHosted := range []bool{true, false, true} {
		router := gin.New()
		route.RegisterWithConfig(router, route.Config{SessionSecret: "secret", SelfHosted: selfHosted})
		req := httptest.NewRequest(http.MethodGet, "/web/api/user/profile", nil)
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		var body struct {
			Data struct {
				MessageQuota int    `json:"message_quota"`
				PlanLevel    string `json:"plan_level"`
			}
		}
		if response.Code != 200 {
			t.Fatal(response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		want := 30000
		if selfHosted {
			want = -1
		}
		if body.Data.MessageQuota != want || body.Data.PlanLevel != "free" {
			t.Fatalf("selfHosted=%v response=%s", selfHosted, response.Body.String())
		}
	}
}

func TestActualPluginUploadHonorsDeploymentQuota(t *testing.T) {
	for _, tc := range []struct {
		name       string
		selfHosted bool
		plan       string
		status     int
	}{{"self-hosted", true, "free", 200}, {"cloud-free", false, "free", 409}, {"cloud-pro", false, "pro", 200}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := model.InitSQLite(filepath.Join(t.TempDir(), "quota.db")); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = model.CloseDB() })
			user := &model.User{Username: "alice", PlanLevel: tc.plan}
			credential, err := authservice.ProvisionUser(user)
			if err != nil {
				t.Fatal(err)
			}
			if err = model.ThreadDB.Save(&model.Thread{UserID: user.ID, Provider: model.ProviderCodex, ProviderThreadID: "T-existing", MessageCount: 30000, Visibility: model.ThreadVisibilityPrivate}); err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			route.RegisterWithConfig(router, route.Config{SelfHosted: tc.selfHosted})
			req := httptest.NewRequest("POST", "/plugin/threads/upload", strings.NewReader(`{"provider":"codex","thread":{"id":"fresh","messages":[{"role":"user","content":"hello"}]}}`))
			req.Header.Set("Authorization", "Bearer "+credential.Plaintext)
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != tc.status {
				t.Fatalf("status %d body %s", res.Code, res.Body.String())
			}
			stored, err := model.UserDB.ByID(user.ID)
			if err != nil || stored.PlanLevel != tc.plan {
				t.Fatal("upload changed membership identity")
			}
		})
	}
}

func TestMembershipExpiryPreservesPrivateHistoryAndExport(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "history.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.CloseDB() })
	user := &model.User{Username: "alice", PlanLevel: "pro"}
	authservice.SetAPIKeyEncryptionSecret("secret")
	key, err := authservice.ProvisionUser(user)
	if err != nil {
		t.Fatal(err)
	}
	cfg := route.Config{SessionSecret: "secret"}
	router := gin.New()
	route.RegisterWithConfig(router, cfg)
	router.NoRoute(func(c *gin.Context) {
		if !route.ServeThreadExport(c, cfg) {
			c.Status(404)
		}
	})
	uploadThread(t, router, key.Plaintext, "T-history", "Preserved history", "retain this message", "owner/repo", "main")
	rows, err := model.ThreadDB.Where("user_id = ?", user.ID).All()
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	now := time.Now()
	if err = model.SubscriptionDB.Save(&model.Subscription{UserID: user.ID, PlanLevel: "pro", Status: "active", StartsAtMS: now.Add(-48 * time.Hour).UnixMilli(), EndsAtMS: now.Add(-time.Hour).UnixMilli(), Source: model.SubscriptionSourceNeo}); err != nil {
		t.Fatal(err)
	}
	result := (subscription.Service{}).ExpireDue(context.Background())
	if result.Failed != 0 {
		t.Fatal(result)
	}
	cookie, err := middleware.NewSessionCookie(user.ID, "secret")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{fmt.Sprintf("/web/api/threads/%d", rows[0].ID), "/threads/T-history.md", "/threads/T-history.json"} {
		req := httptest.NewRequest("GET", path, nil)
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != 200 || !strings.Contains(res.Body.String(), "retain this message") {
			t.Fatalf("expired history %s: %d %s", path, res.Code, res.Body.String())
		}
	}
	count, err := model.ThreadDB.Where("user_id = ?", user.ID).Count()
	if err != nil || count != 1 {
		t.Fatal("expiry removed history")
	}
}
