package user_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/auth"
	userservice "github.com/jieliapp/jieli/backend/service/user"
	"github.com/gin-gonic/gin"
)

func TestAccountPreferencesCannotRestoreResetCredentials(t *testing.T) {
	for _, operation := range []struct {
		name   string
		update func(*model.User) error
	}{
		{"profile", func(u *model.User) error {
			_, e := userservice.UpdateProfile(u, userservice.ProfileUpdate{Username: "alice", DisplayName: "Updated"})
			return e
		}},
		{"avatar", func(u *model.User) error {
			_, e := userservice.UpdateAvatarURL(u, "https://example.com/avatar.png")
			return e
		}},
		{"locale", func(u *model.User) error { _, e := userservice.UpdateLocale(u, "zh"); return e }},
		{"cleanup", func(u *model.User) error { _, e := userservice.SetAutoCleanup(u, true); return e }},
		{"api_key", func(u *model.User) error {
			auth.SetAPIKeyEncryptionSecret("test-secret")
			_, e := auth.RotateAPIKey(u)
			return e
		}},
	} {
		t.Run(operation.name, func(t *testing.T) {
			if err := model.InitSQLite(filepath.Join(t.TempDir(), "user.db")); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = model.CloseDB() })
			stale := &model.User{Username: "alice", APIKeyHash: "old-key", PasswordHash: "old-password-hash"}
			if err := model.UserDB.Save(stale); err != nil {
				t.Fatal(err)
			}
			oldCookie, err := middleware.NewSessionCookieForUser(stale, "secret")
			if err != nil {
				t.Fatal(err)
			}
			updated := *stale
			updated.PasswordHash = "reset-password-hash"
			updated.IsAdmin = true
			if err = model.UserDB.Save(&updated); err != nil {
				t.Fatal(err)
			}
			if err = operation.update(stale); err != nil {
				t.Fatal(err)
			}
			saved, err := model.UserDB.ByID(stale.ID)
			if err != nil {
				t.Fatal(err)
			}
			if saved.PasswordHash != "reset-password-hash" || !saved.IsAdmin {
				t.Fatal("unrelated account update restored stale security fields")
			}
			router := gin.New()
			router.GET("/private", middleware.SessionAuth("secret", false), func(c *gin.Context) { c.Status(200) })
			req := httptest.NewRequest("GET", "/private", nil)
			req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: oldCookie})
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if res.Code != 401 {
				t.Fatalf("reset session became valid again: %d", res.Code)
			}
		})
	}
}
