package auth_test

import (
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/auth"
)

func TestProvisionUserShouldPersistUserAndIssueAPIKey(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	const encryptionSecret = "test-encryption-secret"
	auth.SetAPIKeyEncryptionSecret(encryptionSecret)

	user := &model.User{Username: "newcomer", GitHubID: "github-newcomer"}
	credential, err := auth.ProvisionUser(user)
	if err != nil {
		t.Fatalf("provision user: %v", err)
	}
	if user.ID == 0 {
		t.Fatalf("expected provisioned user to be persisted with an ID")
	}
	if credential.Plaintext == "" || credential.Prefix == "" || credential.Hash == "" {
		t.Fatalf("expected provisioned credential to be populated, got %#v", credential)
	}

	stored, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.APIKeyHash != credential.Hash || stored.APIKeyPrefix != credential.Prefix {
		t.Fatalf("expected stored user to carry issued prefix/hash")
	}
	decrypted, err := common.DecryptSecret(stored.APIKeyEncrypted, encryptionSecret)
	if err != nil {
		t.Fatalf("decrypt stored api key: %v", err)
	}
	if decrypted != credential.Plaintext {
		t.Fatalf("expected decrypted stored key to match issued plaintext")
	}
}
