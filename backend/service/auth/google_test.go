package auth_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/auth"
)

type fakeGoogleOAuthClient struct {
	code string
	user auth.GoogleUser
}

func (client fakeGoogleOAuthClient) UserByCode(code string) (auth.GoogleUser, error) {
	if code != client.code {
		return auth.GoogleUser{}, auth.ErrInvalidOAuthCode
	}
	return client.user, nil
}

func TestGoogleOAuthShouldProvisionAPIKeyForNewGoogleIdentity(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	auth.SetAPIKeyEncryptionSecret("test-encryption-secret")
	service := auth.NewGoogleOAuthService(fakeGoogleOAuthClient{
		code: "oauth-code",
		user: auth.GoogleUser{
			ID:         "google-sub-123",
			Email:      "alice@example.com",
			Name:       "Alice Doe",
			PictureURL: "https://example.com/avatar.png",
		},
	})

	user, credential, err := service.Authenticate("oauth-code")
	if err != nil {
		t.Fatalf("authenticate google user: %v", err)
	}
	if user.ID == 0 {
		t.Fatalf("expected saved user ID")
	}
	if user.GoogleID != "google-sub-123" {
		t.Fatalf("expected Google subject ID to be stored, got %q", user.GoogleID)
	}
	if user.GitHubID != "" {
		t.Fatalf("expected Google login not to set GitHub ID, got %q", user.GitHubID)
	}
	if user.Username != "alice" || user.DisplayName != "Alice Doe" || user.Email != "alice@example.com" || user.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("expected Google profile fields to be copied, got %#v", user)
	}
	if credential.Plaintext == "" || credential.Prefix == "" || credential.Hash == "" {
		t.Fatalf("expected new login to return a provisioned API key credential, got %#v", credential)
	}
}

func TestGoogleOAuthShouldGenerateAvailableUsernameForShortTakenEmailName(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	existing := &model.User{Username: "ab", GitHubID: "github-existing", APIKeyHash: "existing-hash", APIKeyPrefix: "existing-prefix"}
	if err := model.UserDB.Save(existing); err != nil {
		t.Fatalf("save existing user: %v", err)
	}
	auth.SetAPIKeyEncryptionSecret("test-encryption-secret")
	service := auth.NewGoogleOAuthService(fakeGoogleOAuthClient{
		code: "oauth-code",
		user: auth.GoogleUser{
			ID:    "google-sub-123",
			Email: "ab@example.com",
			Name:  "AB User",
		},
	})

	user, _, err := service.Authenticate("oauth-code")
	if err != nil {
		t.Fatalf("authenticate google user: %v", err)
	}
	if user.Username == "ab" || !strings.HasPrefix(user.Username, "ab") || len(user.Username) <= len("ab") {
		t.Fatalf("expected generated username derived from short email name, got %q", user.Username)
	}
}

func TestGoogleOAuthShouldPreserveExistingUserProfileAndAPIKey(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	existing := &model.User{
		Username:     "old-login",
		DisplayName:  "Old Name",
		Email:        "old@example.com",
		AvatarURL:    "https://example.com/old.png",
		GoogleID:     "google-sub-123",
		APIKeyHash:   "keep-hash",
		APIKeyPrefix: "keep-prefix",
	}
	if err := model.UserDB.Save(existing); err != nil {
		t.Fatalf("save existing user: %v", err)
	}
	service := auth.NewGoogleOAuthService(fakeGoogleOAuthClient{
		code: "oauth-code",
		user: auth.GoogleUser{
			ID:         "google-sub-123",
			Email:      "New@Example.COM",
			Name:       "New Name",
			PictureURL: "https://example.com/new.png",
		},
	})

	user, credential, err := service.Authenticate("oauth-code")
	if err != nil {
		t.Fatalf("authenticate google user: %v", err)
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
		t.Fatalf("expected Google email to stay synced, got %q", stored.Email)
	}
	if stored.APIKeyHash != "keep-hash" || stored.APIKeyPrefix != "keep-prefix" {
		t.Fatalf("expected existing API key to be preserved, got hash=%q prefix=%q", stored.APIKeyHash, stored.APIKeyPrefix)
	}
}

func TestGoogleOAuthShouldRejectNewGoogleIdentityWhenEmailAlreadyExists(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	auth.SetAPIKeyEncryptionSecret("test-encryption-secret")
	githubUser := &model.User{
		Username:     "alice-github",
		DisplayName:  "Alice GitHub",
		Email:        "alice@example.com",
		GitHubID:     "12345",
		APIKeyHash:   "github-hash",
		APIKeyPrefix: "github-prefix",
	}
	if err := model.UserDB.Save(githubUser); err != nil {
		t.Fatalf("save existing GitHub user: %v", err)
	}
	service := auth.NewGoogleOAuthService(fakeGoogleOAuthClient{
		code: "oauth-code",
		user: auth.GoogleUser{
			ID:    "google-sub-123",
			Email: "Alice@Example.COM",
			Name:  "Alice Google",
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
	if len(users) != 1 || users[0].ID != githubUser.ID {
		t.Fatalf("expected only existing GitHub user to remain, got %#v", users)
	}
}

func TestGoogleOAuthShouldRejectEmptyCodeBeforeCallingClient(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	client := &countingGoogleOAuthClient{user: auth.GoogleUser{ID: "google-sub-123", Email: "alice@example.com"}}
	service := auth.NewGoogleOAuthService(client)

	_, _, err := service.Authenticate("")
	if err == nil {
		t.Fatalf("expected empty oauth code to be rejected")
	}
	if client.calls != 0 {
		t.Fatalf("expected empty code not to call client, got %d calls", client.calls)
	}
}

type countingGoogleOAuthClient struct {
	calls int
	user  auth.GoogleUser
}

func (client *countingGoogleOAuthClient) UserByCode(string) (auth.GoogleUser, error) {
	client.calls++
	return client.user, nil
}

func TestGoogleOAuthPreservesAdministratorManagedEmail(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "managed-email.db")); err != nil {
		t.Fatal(err)
	}
	existing := &model.User{Username: "manageduser", Email: "managed@example.test", EmailManaged: true, GoogleID: "managed-google", APIKeyHash: "managed"}
	if err := model.UserDB.Save(existing); err != nil {
		t.Fatal(err)
	}
	service := auth.NewGoogleOAuthService(fakeGoogleOAuthClient{code: "code", user: auth.GoogleUser{ID: "managed-google", Email: "provider@example.test"}})
	user, _, err := service.Authenticate("code")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "managed@example.test" {
		t.Fatal("OAuth replaced administrator email")
	}
}
