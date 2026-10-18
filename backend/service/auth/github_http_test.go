package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/service/auth"
)

func TestGitHubHTTPClientShouldExchangeCodeAndFetchUser(t *testing.T) {
	var tokenRequest map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login/oauth/access_token":
			if r.Method != http.MethodPost {
				t.Fatalf("expected token exchange POST, got %s", r.Method)
			}
			if r.Header.Get("Accept") != "application/json" {
				t.Fatalf("expected JSON accept header, got %q", r.Header.Get("Accept"))
			}
			if err := json.NewDecoder(r.Body).Decode(&tokenRequest); err != nil {
				t.Fatalf("decode token request: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "github-token"})
		case "/user":
			if r.Header.Get("Authorization") != "Bearer github-token" {
				t.Fatalf("expected bearer token, got %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         12345,
				"login":      "alice",
				"name":       "Alice Doe",
				"email":      "alice@example.com",
				"avatar_url": "https://example.com/avatar.png",
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := auth.NewGitHubHTTPClient(common.GitHubConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}, auth.GitHubEndpoints{
		TokenURL: server.URL + "/login/oauth/access_token",
		UserURL:  server.URL + "/user",
	})

	user, err := client.UserByCode("oauth-code")
	if err != nil {
		t.Fatalf("fetch github user: %v", err)
	}
	if tokenRequest["client_id"] != "client-id" || tokenRequest["client_secret"] != "client-secret" || tokenRequest["code"] != "oauth-code" {
		t.Fatalf("unexpected token request %#v", tokenRequest)
	}
	if user.ID != 12345 || user.Login != "alice" || user.Name != "Alice Doe" || user.Email != "alice@example.com" || user.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected GitHub user %#v", user)
	}
}
