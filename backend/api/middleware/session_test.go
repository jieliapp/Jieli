package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/model"
	"github.com/gin-gonic/gin"
)

func TestSessionAuthShouldResolveSignedSessionUser(t *testing.T) {
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
	router.GET("/me", middleware.SessionAuth("test-secret", false), func(c *gin.Context) {
		resolved := middleware.CurrentSessionUser(c)
		if resolved == nil {
			t.Fatalf("expected current session user")
		}
		c.JSON(http.StatusOK, gin.H{"user_id": resolved.ID})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestSessionAuthShouldRejectTamperedCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}

	router := gin.New()
	router.GET("/me", middleware.SessionAuth("test-secret", false), func(c *gin.Context) {
		t.Fatalf("handler should not run for invalid session")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/me", nil)
	request.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: "1.invalid"})
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func TestVerifiedCredentialSnapshotCannotCrossPasswordReset(t *testing.T) {
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "reset.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(closeModelDB(t))
	user := &model.User{Username: "alice", PasswordHash: "old-hash"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatal(err)
	}
	verified := *user
	user.PasswordHash = "reset-hash"
	if err := model.UserDB.Save(user); err != nil {
		t.Fatal(err)
	}
	cookie, err := middleware.NewSessionCookieForUser(&verified, "secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/me", middleware.SessionAuth("secret", false), func(c *gin.Context) { c.Status(200) })
	req := httptest.NewRequest("GET", "/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: cookie})
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != 401 {
		t.Fatal("in-flight old credentials obtained a post-reset session")
	}
}
