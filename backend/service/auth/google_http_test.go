package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/service/auth"
)

func TestGoogleHTTPClientShouldExchangeCodeAndFetchUser(t *testing.T) {
	var tokenRequest url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.Method != http.MethodPost {
				t.Fatalf("expected token exchange POST, got %s", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse token request: %v", err)
			}
			tokenRequest = r.PostForm
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "google-token"})
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer google-token" {
				t.Fatalf("expected bearer token, got %q", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":     "google-sub-123",
				"email":   "alice@example.com",
				"name":    "Alice Doe",
				"picture": "https://example.com/avatar.png",
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client := auth.NewGoogleHTTPClient(common.GoogleConfig{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		CallbackURL:  "https://jieli.example.com/web/api/auth/callback/google",
	}, auth.GoogleEndpoints{
		TokenURL:    server.URL + "/token",
		UserInfoURL: server.URL + "/userinfo",
	})

	user, err := client.UserByCode("oauth-code")
	if err != nil {
		t.Fatalf("fetch google user: %v", err)
	}
	if tokenRequest.Get("client_id") != "client-id" || tokenRequest.Get("client_secret") != "client-secret" || tokenRequest.Get("code") != "oauth-code" {
		t.Fatalf("unexpected token request %#v", tokenRequest)
	}
	if tokenRequest.Get("redirect_uri") != "https://jieli.example.com/web/api/auth/callback/google" || tokenRequest.Get("grant_type") != "authorization_code" {
		t.Fatalf("expected redirect_uri and grant_type in token request, got %#v", tokenRequest)
	}
	if user.ID != "google-sub-123" || user.Email != "alice@example.com" || user.Name != "Alice Doe" || user.PictureURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected Google user %#v", user)
	}
}
