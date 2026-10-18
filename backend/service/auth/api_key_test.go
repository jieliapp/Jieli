package auth_test

import (
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/auth"
)

func TestRotateAPIKeyShouldStorePrefixHashAndEncryptedKey(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	const encryptionSecret = "test-encryption-secret"
	auth.SetAPIKeyEncryptionSecret(encryptionSecret)
	user := &model.User{
		Username:     "alice",
		GitHubID:     "github-alice",
		APIKeyHash:   "initial-hash",
		APIKeyPrefix: "initial-prefix",
	}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	credential, err := auth.RotateAPIKey(user)
	if err != nil {
		t.Fatalf("rotate api key: %v", err)
	}
	if credential.Plaintext == "" {
		t.Fatalf("expected plaintext key to be returned once")
	}
	if user.APIKeyPrefix != credential.Prefix {
		t.Fatalf("expected user prefix %q to match credential prefix %q", user.APIKeyPrefix, credential.Prefix)
	}
	if user.APIKeyHash != credential.Hash {
		t.Fatalf("expected user hash to be updated")
	}
	if user.APIKeyHash == credential.Plaintext {
		t.Fatalf("expected hash not to equal plaintext")
	}
	if user.APIKeyEncrypted == "" || user.APIKeyEncrypted == credential.Plaintext {
		t.Fatalf("expected api key to be stored encrypted, got %q", user.APIKeyEncrypted)
	}
	stored, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.APIKeyPrefix != credential.Prefix || stored.APIKeyHash != credential.Hash {
		t.Fatalf("expected stored user to contain rotated prefix/hash")
	}
	decrypted, err := common.DecryptSecret(stored.APIKeyEncrypted, encryptionSecret)
	if err != nil {
		t.Fatalf("decrypt stored api key: %v", err)
	}
	if decrypted != credential.Plaintext {
		t.Fatalf("expected decrypted stored key %q to match credential plaintext %q", decrypted, credential.Plaintext)
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
