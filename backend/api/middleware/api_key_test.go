package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/auth"
	"github.com/gin-gonic/gin"
)

func TestAPIKeyAuthShouldResolveBearerUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	credential, err := auth.ProvisionUser(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}

	router := gin.New()
	router.GET("/protected", middleware.APIKeyAuth(), func(c *gin.Context) {
		resolved := middleware.CurrentAPIUser(c)
		if resolved == nil {
			t.Fatalf("expected current api user")
		}
		c.JSON(http.StatusOK, gin.H{"user_id": resolved.ID})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+credential.Plaintext)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body %s", recorder.Code, recorder.Body.String())
	}
}

func closeModelDB(t *testing.T) func() {
	t.Helper()
	return func() {
		if err := model.CloseDB(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}
