package auth_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/auth"
)

type fakeGitHubOAuthClient struct {
	code string
	user auth.GitHubUser
}

func (client fakeGitHubOAuthClient) UserByCode(code string) (auth.GitHubUser, error) {
	if code != client.code {
		return auth.GitHubUser{}, auth.ErrInvalidOAuthCode
	}
	return client.user, nil
}

func TestGitHubOAuthShouldProvisionAPIKeyForNewGitHubIdentity(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	auth.SetAPIKeyEncryptionSecret("test-encryption-secret")
	service := auth.NewGitHubOAuthService(fakeGitHubOAuthClient{
		code: "oauth-code",
		user: auth.GitHubUser{
			ID:        12345,
			Login:     "alice",
			Name:      "Alice Doe",
			Email:     "alice@example.com",
			AvatarURL: "https://example.com/avatar.png",
		},
	})

	user, credential, err := service.Authenticate("oauth-code")
	if err != nil {
		t.Fatalf("authenticate github user: %v", err)
	}
	if user.ID == 0 {
		t.Fatalf("expected saved user ID")
	}
	if user.GitHubID != "12345" {
		t.Fatalf("expected GitHub numeric ID to be stored, got %q", user.GitHubID)
	}
	if user.Username != "alice" || user.DisplayName != "Alice Doe" || user.Email != "alice@example.com" || user.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("expected GitHub profile fields to be copied, got %#v", user)
	}
	if credential.Plaintext == "" || credential.Prefix == "" || credential.Hash == "" {
		t.Fatalf("expected new login to return a provisioned API key credential, got %#v", credential)
	}
	stored, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.APIKeyHash == "" || stored.APIKeyPrefix == "" || stored.APIKeyEncrypted == "" {
		t.Fatalf("expected new GitHub user to be provisioned an API key, got hash=%q prefix=%q encrypted=%q", stored.APIKeyHash, stored.APIKeyPrefix, stored.APIKeyEncrypted)
	}
}

func TestGitHubOAuthShouldGenerateAvailableUsernameForReservedLogin(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	auth.SetAPIKeyEncryptionSecret("test-encryption-secret")
	service := auth.NewGitHubOAuthService(fakeGitHubOAuthClient{
		code: "oauth-code",
		user: auth.GitHubUser{
			ID:    12345,
			Login: "cli",
			Name:  "CLI User",
			Email: "cli@example.com",
		},
	})

	user, _, err := service.Authenticate("oauth-code")
	if err != nil {
		t.Fatalf("authenticate github user: %v", err)
	}
	if user.Username == "cli" || !strings.HasPrefix(user.Username, "cli") || len(user.Username) <= len("cli") {
		t.Fatalf("expected generated username derived from reserved login, got %q", user.Username)
	}
}

func TestGitHubOAuthShouldPreserveExistingUserProfileAndAPIKey(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	existing := &model.User{
		Username:     "old-login",
		DisplayName:  "Old Name",
		Email:        "old@example.com",
		AvatarURL:    "https://example.com/old.png",
		GitHubID:     "12345",
		APIKeyHash:   "keep-hash",
		APIKeyPrefix: "keep-prefix",
	}
	if err := model.UserDB.Save(existing); err != nil {
		t.Fatalf("save existing user: %v", err)
	}
	service := auth.NewGitHubOAuthService(fakeGitHubOAuthClient{
		code: "oauth-code",
		user: auth.GitHubUser{
			ID:        12345,
			Login:     "new-login",
			Name:      "New Name",
			Email:     "New@Example.COM",
			AvatarURL: "https://example.com/new.png",
		},
	})

	user, credential, err := service.Authenticate("oauth-code")
	if err != nil {
		t.Fatalf("authenticate github user: %v", err)
	}
	if credential.Plaintext != "" || credential.Prefix != "" || credential.Hash != "" {
		t.Fatalf("expected existing login not to return or rotate API key, got %#v", credential)
	}
	if user.ID != existing.ID {
		t.Fatalf("expected existing user ID %d, got %d", existing.ID, user.ID)
	}
	stored, err := model.UserDB.ByID(existing.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.Username != "old-login" || stored.DisplayName != "Old Name" || stored.AvatarURL != "https://example.com/old.png" {
		t.Fatalf("expected custom profile fields to be preserved, got %#v", stored)
	}
	if stored.Email != "new@example.com" {
		t.Fatalf("expected GitHub email to stay synced, got %q", stored.Email)
	}
	if stored.APIKeyHash != "keep-hash" || stored.APIKeyPrefix != "keep-prefix" {
		t.Fatalf("expected existing API key to be preserved, got hash=%q prefix=%q", stored.APIKeyHash, stored.APIKeyPrefix)
	}
}

func TestGitHubOAuthShouldRejectNewGitHubIdentityWhenEmailAlreadyExists(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	auth.SetAPIKeyEncryptionSecret("test-encryption-secret")
	googleUser := &model.User{
		Username:     "alice-google",
		DisplayName:  "Alice Google",
		Email:        "alice@example.com",
		GoogleID:     "google-sub-123",
		APIKeyHash:   "google-hash",
		APIKeyPrefix: "google-prefix",
	}
	if err := model.UserDB.Save(googleUser); err != nil {
		t.Fatalf("save existing Google user: %v", err)
	}
	service := auth.NewGitHubOAuthService(fakeGitHubOAuthClient{
		code: "oauth-code",
		user: auth.GitHubUser{
			ID:    12345,
			Login: "alice",
			Email: "Alice@Example.COM",
		},
	})

	user, credential, err := service.Authenticate("oauth-code")
	if err == nil {
		t.Fatalf("expected duplicate email to be rejected, got user %#v", user)
	}
	if !errors.Is(err, auth.ErrOAuthEmailAccountExists) {
		t.Fatalf("expected ErrOAuthEmailAccountExists, got %v", err)
	}
	if credential.Plaintext != "" || credential.Prefix != "" || credential.Hash != "" {
		t.Fatalf("expected duplicate email not to provision API key, got %#v", credential)
	}
	users, err := model.UserDB.Where("email = ?", "alice@example.com").Fetch(0, 10)
	if err != nil {
		t.Fatalf("list users by email: %v", err)
	}
	if len(users) != 1 || users[0].ID != googleUser.ID {
		t.Fatalf("expected only existing Google user to remain, got %#v", users)
	}
}

func TestGitHubOAuthShouldRejectEmptyCodeBeforeCallingClient(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	client := &countingGitHubOAuthClient{user: auth.GitHubUser{ID: 12345, Login: "alice"}}
	service := auth.NewGitHubOAuthService(client)

	_, _, err := service.Authenticate("")
	if err == nil {
		t.Fatalf("expected empty oauth code to be rejected")
	}
	if client.calls != 0 {
		t.Fatalf("expected empty code not to call client, got %d calls", client.calls)
	}
}

type countingGitHubOAuthClient struct {
	calls int
	user  auth.GitHubUser
}

func (client *countingGitHubOAuthClient) UserByCode(string) (auth.GitHubUser, error) {
	client.calls++
	return client.user, nil
}

func TestGitHubOAuthPreservesAdministratorManagedEmail(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "managed-email.db")); err != nil {
		t.Fatal(err)
	}
	existing := &model.User{Username: "manageduser", Email: "managed@example.test", EmailManaged: true, GitHubID: "24680", APIKeyHash: "managed"}
	if err := model.UserDB.Save(existing); err != nil {
		t.Fatal(err)
	}
	service := auth.NewGitHubOAuthService(fakeGitHubOAuthClient{code: "code", user: auth.GitHubUser{ID: 24680, Login: "provider", Email: "provider@example.test"}})
	user, _, err := service.Authenticate("code")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "managed@example.test" {
		t.Fatal("OAuth replaced administrator email")
	}
}
