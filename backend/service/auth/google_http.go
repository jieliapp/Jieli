package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jieliapp/jieli/backend/common"
)

const (
	defaultGoogleTokenURL    = "https://oauth2.googleapis.com/token"
	defaultGoogleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

type GoogleEndpoints struct {
	TokenURL    string
	UserInfoURL string
}

type GoogleHTTPClient struct {
	config    common.GoogleConfig
	endpoints GoogleEndpoints
	client    *http.Client
}

type googleOAuthTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type googleUserInfoResponse struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func NewGoogleHTTPClient(config common.GoogleConfig, endpoints GoogleEndpoints) GoogleHTTPClient {
	if endpoints.TokenURL == "" {
		endpoints.TokenURL = defaultGoogleTokenURL
	}
	if endpoints.UserInfoURL == "" {
		endpoints.UserInfoURL = defaultGoogleUserInfoURL
	}
	return GoogleHTTPClient{
		config:    config,
		endpoints: endpoints,
		client:    &http.Client{Timeout: 5 * time.Second},
	}
}

func (client GoogleHTTPClient) UserByCode(code string) (GoogleUser, error) {
	if code == "" {
		return GoogleUser{}, ErrInvalidOAuthCode
	}
	token, err := client.exchangeCode(code)
	if err != nil {
		return GoogleUser{}, err
	}
	return client.fetchUser(token)
}

func (client GoogleHTTPClient) exchangeCode(code string) (string, error) {
	body := url.Values{}
	body.Set("client_id", client.config.ClientID)
	body.Set("client_secret", client.config.ClientSecret)
	body.Set("code", code)
	body.Set("redirect_uri", client.config.CallbackURL)
	body.Set("grant_type", "authorization_code")
	request, err := http.NewRequest(http.MethodPost, client.endpoints.TokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("google token exchange failed: %s", response.Status)
	}
	var payload googleOAuthTokenResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.AccessToken == "" {
		return "", errors.New("google token response missing access_token")
	}
	return payload.AccessToken, nil
}

func (client GoogleHTTPClient) fetchUser(token string) (GoogleUser, error) {
	request, err := http.NewRequest(http.MethodGet, client.endpoints.UserInfoURL, nil)
	if err != nil {
		return GoogleUser{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	response, err := client.client.Do(request)
	if err != nil {
		return GoogleUser{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return GoogleUser{}, fmt.Errorf("google userinfo fetch failed: %s", response.Status)
	}
	var payload googleUserInfoResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return GoogleUser{}, err
	}
	if payload.Sub == "" || payload.Email == "" {
		return GoogleUser{}, errors.New("google userinfo response missing identity")
	}
	return GoogleUser{
		ID:         payload.Sub,
		Email:      payload.Email,
		Name:       payload.Name,
		PictureURL: payload.Picture,
	}, nil
}
