package auth_test

import (
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/auth"
)

func TestEnsureDevUserShouldCreateUsableAPIKey(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}

	user, err := auth.EnsureDevUser()
	if err != nil {
		t.Fatalf("ensure dev user: %v", err)
	}
	if user.APIKeyHash == "" {
		t.Fatalf("expected dev user to have an api key hash")
	}
	if user.APIKeyPrefix == "" {
		t.Fatalf("expected dev user to have an api key prefix")
	}
	if user.APIKeyEncrypted == "" {
		t.Fatalf("expected dev user to have a stored encrypted api key")
	}
	if user.PlanLevel != model.PlanLevelFree {
		t.Fatalf("expected dev user to default to free plan, got %q", user.PlanLevel)
	}
}
