package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jieliapp/jieli/backend/common"
)

const (
	defaultGitHubTokenURL = "https://github.com/login/oauth/access_token"
	defaultGitHubUserURL  = "https://api.github.com/user"
)

type GitHubEndpoints struct {
	TokenURL string
	UserURL  string
}

type GitHubHTTPClient struct {
	config    common.GitHubConfig
	endpoints GitHubEndpoints
	client    *http.Client
}

type githubOAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type githubUserResponse struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

func NewGitHubHTTPClient(config common.GitHubConfig, endpoints GitHubEndpoints) GitHubHTTPClient {
	if endpoints.TokenURL == "" {
		endpoints.TokenURL = defaultGitHubTokenURL
	}
	if endpoints.UserURL == "" {
		endpoints.UserURL = defaultGitHubUserURL
	}
	return GitHubHTTPClient{
		config:    config,
		endpoints: endpoints,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (client GitHubHTTPClient) UserByCode(code string) (GitHubUser, error) {
	if code == "" {
		return GitHubUser{}, ErrInvalidOAuthCode
	}
	token, err := client.exchangeCode(code)
	if err != nil {
		return GitHubUser{}, err
	}
	return client.fetchUser(token)
}

func (client GitHubHTTPClient) exchangeCode(code string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"client_id":     client.config.ClientID,
		"client_secret": client.config.ClientSecret,
		"code":          code,
	})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest(http.MethodPost, client.endpoints.TokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("github token exchange failed: %s", response.Status)
	}
	var payload githubOAuthTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", errors.New("github token response missing access_token")
	}
	return payload.AccessToken, nil
}

func (client GitHubHTTPClient) fetchUser(token string) (GitHubUser, error) {
	request, err := http.NewRequest(http.MethodGet, client.endpoints.UserURL, nil)
	if err != nil {
		return GitHubUser{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return GitHubUser{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return GitHubUser{}, fmt.Errorf("github user fetch failed: %s", response.Status)
	}
	var payload githubUserResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return GitHubUser{}, err
	}
	if payload.ID == 0 || payload.Login == "" {
		return GitHubUser{}, errors.New("github user response missing identity")
	}
	return GitHubUser{
		ID:        payload.ID,
		Login:     payload.Login,
		Name:      payload.Name,
		Email:     payload.Email,
		AvatarURL: payload.AvatarURL,
	}, nil
}
