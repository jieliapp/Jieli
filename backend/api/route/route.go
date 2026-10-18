package route

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	attachmentservice "github.com/jieliapp/jieli/backend/service/attachment"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
	threadservice "github.com/jieliapp/jieli/backend/service/thread"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

type Config struct {
	SendMail               func(context.Context, common.SMTPConfig, string, string) error
	SetupToken             string
	SelfHosted             bool
	DevMode                bool
	SessionSecret          string
	CookieSecure           bool
	R2PublicBaseURL        string
	APIKeyEncryptionSecret string
	GitHub                 common.GitHubConfig
	GitHubClient           authservice.GitHubOAuthClient
	Google                 common.GoogleConfig
	GoogleClient           authservice.GoogleOAuthClient
}

const (
	githubOAuthStateCookieName = "jieli_oauth_state"
	googleOAuthStateCookieName = "jieli_oauth_state_google"
	threadUploadMaxBodyBytes   = 64 << 20
)

func Register(router *gin.Engine) {
	RegisterWithConfig(router, Config{})
}

func RegisterWithConfig(router *gin.Engine, cfg Config) {
	router.Use(func(c *gin.Context) { c.Set("unlimited_storage", cfg.SelfHosted); c.Next() })
	uploadLimiter := newUploadRateLimiter(10, time.Minute)
	uploadGate := newUserUploadGate()
	attachmentLimiter := newUploadRateLimiter(60, time.Minute)

	uploadThreadSnapshot := func(c *gin.Context) {
		user := middleware.CurrentAPIUser(c)
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, threadUploadMaxBodyBytes)
		release, err := uploadGate.acquire(c.Request.Context(), user.ID)
		if err != nil {
			c.JSON(http.StatusRequestTimeout, common.Response{Success: false, Message: "upload cancelled"})
			return
		}
		defer release()
		var payload threadservice.UploadPayload
		if err = c.ShouldBindJSON(&payload); err != nil {
			var maxBytesError *http.MaxBytesError
			if errors.As(err, &maxBytesError) {
				c.JSON(http.StatusRequestEntityTooLarge, common.Response{Success: false, Message: "upload payload too large"})
				return
			}
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid upload payload"})
			return
		}

		provider, providerThreadID, err := threadservice.UploadIdentity(payload)
		if err != nil {
			writeUploadThreadError(c, err)
			return
		}
		if !uploadLimiter.allowThread(user.ID, provider, providerThreadID, time.Now()) {
			c.JSON(http.StatusTooManyRequests, common.Response{Success: false, Message: "upload rate limit exceeded"})
			return
		}
		stored, err := threadservice.UploadThreadSnapshot(user, payload)
		if err != nil {
			writeUploadThreadError(c, err)
			return
		}
		common.OK(c, gin.H{
			"id":                 stored.ID,
			"provider":           stored.Provider,
			"provider_thread_id": stored.ProviderThreadID,
		})
	}
	listPluginThreads := func(c *gin.Context) {
		options, err := threadListOptionsFromQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid archived filter"})
			return
		}
		result, err := threadservice.ListThreads(middleware.CurrentAPIUser(c), options)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "list threads failed"})
			return
		}
		common.OK(c, result)
	}
	archivePluginThread := func(c *gin.Context) {
		var payload struct {
			Provider         string `json:"provider"`
			ProviderThreadID string `json:"provider_thread_id"`
			Archived         bool   `json:"archived"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil || !model.IsSupportedProvider(strings.TrimSpace(payload.Provider)) || strings.TrimSpace(payload.ProviderThreadID) == "" {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid archive payload"})
			return
		}
		stored, err := threadservice.SetProviderThreadArchived(
			middleware.CurrentAPIUser(c),
			payload.Provider,
			payload.ProviderThreadID,
			payload.Archived,
		)
		if err != nil {
			if errors.Is(err, threadservice.ErrThreadNotFound) {
				c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "thread not found"})
				return
			}
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "archive thread failed"})
			return
		}
		common.OK(c, gin.H{
			"provider":           stored.Provider,
			"provider_thread_id": stored.ProviderThreadID,
			"archived":           stored.Archived,
		})
	}
	uploadAttachment := func(c *gin.Context) {
		user := middleware.CurrentAPIUser(c)
		if !attachmentLimiter.allow(user.ID, time.Now()) {
			c.JSON(http.StatusTooManyRequests, common.Response{Success: false, Message: "attachment upload rate limit exceeded"})
			return
		}
		var payload attachmentservice.UploadPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid attachment payload"})
			return
		}
		stored, err := attachmentservice.Upload(user, payload)
		if err != nil {
			c.JSON(statusForAttachmentError(err), common.Response{Success: false, Message: "upload attachment failed"})
			return
		}
		baseURL, err := sitePublicBaseURL()
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "site settings unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"url": attachmentURL(c, baseURL, cfg.R2PublicBaseURL, stored)})
	}

	// Plugin endpoints are authenticated with the user API key.
	pluginAPI := router.Group("/plugin")
	pluginAPI.POST("/attachments", middleware.APIKeyAuth(), uploadAttachment)
	pluginAPI.POST("/threads/upload", middleware.APIKeyAuth(), uploadThreadSnapshot)
	pluginAPI.POST("/threads/archive", middleware.APIKeyAuth(), archivePluginThread)
	pluginAPI.GET("/threads", middleware.APIKeyAuth(), listPluginThreads)

	// Web frontend API — session-authenticated endpoints.
	api := router.Group("/web/api")
	registerAccounts(api, cfg)
	registerCommercial(api, cfg)
	api.GET("/health", func(c *gin.Context) {
		common.OK(c, gin.H{"status": "ok"})
	})
	api.GET("/config", func(c *gin.Context) {
		common.OK(c, PublicRuntimeConfig(cfg))
	})
	api.GET("/share/:slug", func(c *gin.Context) {
		detail, err := threadservice.GetSharedThread(c.Param("slug"))
		if err != nil {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "shared thread not found"})
			return
		}
		c.Header("X-Robots-Tag", "noindex")
		common.OK(c, detail)
	})
	api.GET("/users/:username/profile", func(c *gin.Context) {
		profile, err := userservice.GetPublicProfile(c.Param("username"))
		if err != nil {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "public profile not found"})
			return
		}
		common.OK(c, profile)
	})
	api.GET("/users/:username/activity", func(c *gin.Context) {
		activity, err := userservice.ListPublicActivity(c.Param("username"))
		if err != nil {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "public profile not found"})
			return
		}
		common.OK(c, activity)
	})

	router.GET("/attachments/:id", func(c *gin.Context) {
		stored, err := attachmentservice.GetPublic(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "attachment not found"})
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.Data(http.StatusOK, stored.MediaType, stored.Data)
	})

	auth := api.Group("/auth")
	auth.GET("/github", func(c *gin.Context) {
		if cfg.GitHub.ClientID == "" || cfg.GitHub.CallbackURL == "" {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "github oauth config is required"})
			return
		}
		query := url.Values{}
		query.Set("client_id", cfg.GitHub.ClientID)
		query.Set("redirect_uri", cfg.GitHub.CallbackURL)
		query.Set("scope", "read:user user:email")
		state, err := newOAuthState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "create oauth state failed"})
			return
		}
		query.Set("state", state)
		c.SetCookie(githubOAuthStateCookieName, state, 600, "/web/api/auth/callback/github", "", cfg.CookieSecure, true)
		c.Redirect(http.StatusTemporaryRedirect, "https://github.com/login/oauth/authorize?"+query.Encode())
	})
	auth.GET("/callback/github", func(c *gin.Context) {
		if cfg.SessionSecret == "" || cfg.GitHub.ClientID == "" || cfg.GitHub.ClientSecret == "" || cfg.GitHub.CallbackURL == "" || cfg.GitHub.PostLoginRedirect == "" {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "github oauth config is required"})
			return
		}
		state, err := c.Cookie(githubOAuthStateCookieName)
		if err != nil || state == "" || c.Query("state") == "" || subtle.ConstantTimeCompare([]byte(state), []byte(c.Query("state"))) != 1 {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid oauth state"})
			return
		}
		if c.Query("error") != "" {
			c.SetCookie(githubOAuthStateCookieName, "", -1, "/web/api/auth/callback/github", "", cfg.CookieSecure, true)
			c.Redirect(http.StatusTemporaryRedirect, "/web/auth/signin?error=oauth_cancelled")
			return
		}
		client := cfg.GitHubClient
		if client == nil {
			client = authservice.NewGitHubHTTPClient(cfg.GitHub, authservice.GitHubEndpoints{})
		}
		service := authservice.NewGitHubOAuthService(client)
		user, _, err := service.Authenticate(c.Query("code"))
		if err != nil {
			if errors.Is(err, authservice.ErrInactiveUser) {
				c.JSON(http.StatusUnauthorized, common.Response{Message: "account inactive"})
				return
			}
			if errors.Is(err, authservice.ErrOAuthEmailAccountExists) {
				c.SetCookie(githubOAuthStateCookieName, "", -1, "/web/api/auth/callback/github", "", cfg.CookieSecure, true)
				c.Redirect(http.StatusTemporaryRedirect, "/web/auth/signin?error=oauth_email_exists")
				return
			}
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "github oauth callback failed"})
			return
		}
		cookie, err := middleware.NewSessionCookie(user.ID, cfg.SessionSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "create session failed"})
			return
		}
		c.SetCookie(githubOAuthStateCookieName, "", -1, "/web/api/auth/callback/github", "", cfg.CookieSecure, true)
		c.SetCookie(middleware.SessionCookieName, cookie, middleware.SessionMaxAge, "/", "", cfg.CookieSecure, true)
		c.Redirect(http.StatusTemporaryRedirect, cfg.GitHub.PostLoginRedirect)
	})
	auth.GET("/google", func(c *gin.Context) {
		if cfg.Google.ClientID == "" || cfg.Google.CallbackURL == "" {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "google oauth config is required"})
			return
		}
		query := url.Values{}
		query.Set("client_id", cfg.Google.ClientID)
		query.Set("redirect_uri", cfg.Google.CallbackURL)
		query.Set("scope", "openid email profile")
		query.Set("response_type", "code")
		state, err := newOAuthState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "create oauth state failed"})
			return
		}
		query.Set("state", state)
		c.SetCookie(googleOAuthStateCookieName, state, 600, "/web/api/auth/callback/google", "", cfg.CookieSecure, true)
		c.Redirect(http.StatusTemporaryRedirect, "https://accounts.google.com/o/oauth2/v2/auth?"+query.Encode())
	})
	auth.GET("/callback/google", func(c *gin.Context) {
		if cfg.SessionSecret == "" || cfg.Google.ClientID == "" || cfg.Google.ClientSecret == "" || cfg.Google.CallbackURL == "" || cfg.Google.PostLoginRedirect == "" {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "google oauth config is required"})
			return
		}
		state, err := c.Cookie(googleOAuthStateCookieName)
		if err != nil || state == "" || c.Query("state") == "" || subtle.ConstantTimeCompare([]byte(state), []byte(c.Query("state"))) != 1 {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid oauth state"})
			return
		}
		if c.Query("error") != "" {
			c.SetCookie(googleOAuthStateCookieName, "", -1, "/web/api/auth/callback/google", "", cfg.CookieSecure, true)
			c.Redirect(http.StatusTemporaryRedirect, "/web/auth/signin?error=oauth_cancelled")
			return
		}
		client := cfg.GoogleClient
		if client == nil {
			client = authservice.NewGoogleHTTPClient(cfg.Google, authservice.GoogleEndpoints{})
		}
		service := authservice.NewGoogleOAuthService(client)
		user, _, err := service.Authenticate(c.Query("code"))
		if err != nil {
			if errors.Is(err, authservice.ErrInactiveUser) {
				c.JSON(http.StatusUnauthorized, common.Response{Message: "account inactive"})
				return
			}
			if errors.Is(err, authservice.ErrOAuthEmailAccountExists) {
				c.SetCookie(googleOAuthStateCookieName, "", -1, "/web/api/auth/callback/google", "", cfg.CookieSecure, true)
				c.Redirect(http.StatusTemporaryRedirect, "/web/auth/signin?error=oauth_email_exists")
				return
			}
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "google oauth callback failed"})
			return
		}
		cookie, err := middleware.NewSessionCookie(user.ID, cfg.SessionSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "create session failed"})
			return
		}
		c.SetCookie(googleOAuthStateCookieName, "", -1, "/web/api/auth/callback/google", "", cfg.CookieSecure, true)
		c.SetCookie(middleware.SessionCookieName, cookie, middleware.SessionMaxAge, "/", "", cfg.CookieSecure, true)
		c.Redirect(http.StatusTemporaryRedirect, cfg.Google.PostLoginRedirect)
	})
	auth.GET("/me", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure), func(c *gin.Context) {
		common.OK(c, middleware.CurrentSessionUser(c))
	})
	auth.POST("/logout", func(c *gin.Context) {
		c.SetCookie(middleware.SessionCookieName, "", -1, "/", "", false, true)
		common.OK(c, nil)
	})
	if cfg.DevMode {
		auth.GET("/dev-login", func(c *gin.Context) {
			user, err := authservice.EnsureDevUser()
			if err != nil {
				if errors.Is(err, authservice.ErrInactiveUser) {
					c.JSON(http.StatusUnauthorized, common.Response{Message: "account inactive"})
					return
				}
				c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "dev login failed"})
				return
			}
			cookie, err := middleware.NewSessionCookie(user.ID, cfg.SessionSecret)
			if err != nil {
				c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "create session failed"})
				return
			}
			c.SetCookie(middleware.SessionCookieName, cookie, middleware.SessionMaxAge, "/", "", cfg.CookieSecure, true)
			c.Redirect(http.StatusTemporaryRedirect, "/web/dashboard")
		})
	}

	user := api.Group("/user", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure))
	user.GET("/profile", func(c *gin.Context) {
		profile, err := userservice.GetProfile(middleware.CurrentSessionUser(c))
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "get profile failed"})
			return
		}
		common.OK(c, profile)
	})
	user.PATCH("/profile", func(c *gin.Context) {
		var update userservice.ProfileUpdate
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid profile payload"})
			return
		}
		updated, err := userservice.UpdateProfile(middleware.CurrentSessionUser(c), update)
		if err != nil {
			if errors.Is(err, userservice.ErrUsernameTaken) {
				c.JSON(http.StatusConflict, common.Response{Success: false, Message: "username unavailable"})
				return
			}
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid profile"})
			return
		}
		common.OK(c, updated)
	})
	user.POST("/avatar", func(c *gin.Context) {
		sessionUser := middleware.CurrentSessionUser(c)
		if !attachmentLimiter.allow(sessionUser.ID, time.Now()) {
			c.JSON(http.StatusTooManyRequests, common.Response{Success: false, Message: "attachment upload rate limit exceeded"})
			return
		}
		var payload attachmentservice.UploadPayload
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid avatar payload"})
			return
		}
		stored, err := attachmentservice.Upload(sessionUser, payload)
		if err != nil {
			c.JSON(statusForAttachmentError(err), common.Response{Success: false, Message: "upload avatar failed"})
			return
		}
		baseURL, err := sitePublicBaseURL()
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "site settings unavailable"})
			return
		}
		updated, err := userservice.UpdateAvatarURL(middleware.CurrentSessionUser(c), attachmentURL(c, baseURL, cfg.R2PublicBaseURL, stored))
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "update avatar failed"})
			return
		}
		common.OK(c, updated)
	})
	user.GET("/activity", func(c *gin.Context) {
		activity, err := userservice.ListActivity(middleware.CurrentSessionUser(c))
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "list activity failed"})
			return
		}
		common.OK(c, activity)
	})
	user.PATCH("/locale", func(c *gin.Context) {
		var payload struct {
			Locale string `json:"locale"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid locale payload"})
			return
		}
		updated, err := userservice.UpdateLocale(middleware.CurrentSessionUser(c), payload.Locale)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid locale"})
			return
		}
		common.OK(c, updated)
	})
	user.PUT("/auto-cleanup", func(c *gin.Context) {
		var payload struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid auto-cleanup payload"})
			return
		}
		updated, err := userservice.SetAutoCleanup(middleware.CurrentSessionUser(c), payload.Enabled)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "update auto-cleanup failed"})
			return
		}
		common.OK(c, gin.H{"auto_cleanup_enabled": updated.AutoCleanupEnabled})
	})
	user.POST("/restore-threads", func(c *gin.Context) {
		currentUser := middleware.CurrentSessionUser(c)
		if model.MessageQuotaForUser(currentUser) >= 0 {
			c.JSON(http.StatusForbidden, common.Response{Success: false, Message: "restore requires unlimited message storage"})
			return
		}
		restored, err := threadservice.RestoreDeletedThreads(currentUser)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "restore threads failed"})
			return
		}
		common.OK(c, gin.H{"restored": restored})
	})
	user.GET("/api-key", func(c *gin.Context) {
		currentUser := middleware.CurrentSessionUser(c)
		var plaintext string
		if currentUser.APIKeyEncrypted != "" {
			if decrypted, err := common.DecryptSecret(currentUser.APIKeyEncrypted, cfg.APIKeyEncryptionSecret); err == nil {
				plaintext = decrypted
			}
		}
		common.OK(c, gin.H{
			"prefix":  currentUser.APIKeyPrefix,
			"has_key": currentUser.APIKeyHash != "",
			"api_key": plaintext,
		})
	})
	user.POST("/api-key/rotate", func(c *gin.Context) {
		currentUser := middleware.CurrentSessionUser(c)
		credential, err := authservice.RotateAPIKey(currentUser)
		if err != nil {
			c.JSON(http.StatusInternalServerError, common.Response{Success: false, Message: "rotate api key failed"})
			return
		}
		common.OK(c, gin.H{
			"api_key": credential.Plaintext,
			"prefix":  credential.Prefix,
		})
	})

	threads := api.Group("/threads")
	threads.GET("", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure), func(c *gin.Context) {
		options, err := threadListOptionsFromQuery(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid archived filter"})
			return
		}
		result, err := threadservice.ListThreads(middleware.CurrentSessionUser(c), options)
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "list threads failed"})
			return
		}
		common.OK(c, result)
	})
	threads.GET("/facets", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure), func(c *gin.Context) {
		facets, err := threadservice.ThreadFacets(middleware.CurrentSessionUser(c))
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "list facets failed"})
			return
		}
		common.OK(c, facets)
	})
	threads.GET("/:id", func(c *gin.Context) {
		rawID := c.Param("id")
		currentUser, hasSession := middleware.ResolveSessionUser(c, cfg.SessionSecret)
		// Try numeric DB id for authenticated owners only, then fall back to provider_thread_id lookup.
		if threadID, err := strconv.ParseInt(rawID, 10, 64); err == nil && threadID > 0 {
			if !hasSession {
				c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "thread not found"})
				return
			}
			detail, err := threadservice.GetThreadDetail(currentUser, threadID)
			if err != nil {
				c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "thread not found"})
				return
			}
			common.OK(c, detail)
			return
		}
		if hasSession {
			if detail, err := threadservice.GetThreadDetailByProviderID(currentUser, rawID); err == nil {
				common.OK(c, detail)
				return
			}
		}
		detail, err := threadservice.GetPublicThreadDetailByProviderID(rawID)
		if err != nil {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "thread not found"})
			return
		}
		c.Header("X-Robots-Tag", "noindex")
		common.OK(c, detail)
	})
	threads.PATCH("/:id/meta", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure), func(c *gin.Context) {
		threadID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || threadID == 0 {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid thread id"})
			return
		}
		var update threadservice.MetaUpdate
		if err := c.ShouldBindJSON(&update); err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid thread meta"})
			return
		}
		stored, err := threadservice.UpdateThreadMeta(middleware.CurrentSessionUser(c), threadID, update)
		if err != nil {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "thread not found"})
			return
		}
		common.OK(c, stored)
	})
	threads.POST("/:id/archive", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure), func(c *gin.Context) {
		threadID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || threadID == 0 {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid thread id"})
			return
		}
		var payload struct {
			Archived bool `json:"archived"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid archive payload"})
			return
		}
		stored, err := threadservice.SetThreadArchived(middleware.CurrentSessionUser(c), threadID, payload.Archived)
		if err != nil {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "thread not found"})
			return
		}
		common.OK(c, stored)
	})
	threads.DELETE("/:id", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure), func(c *gin.Context) {
		threadID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || threadID == 0 {
			c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "invalid thread id"})
			return
		}
		if err := threadservice.DeleteThread(middleware.CurrentSessionUser(c), threadID); err != nil {
			c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "thread not found"})
			return
		}
		common.OK(c, gin.H{"deleted": true})
	})

}

func attachmentURL(c *gin.Context, publicBaseURL string, r2PublicBaseURL string, stored *model.Attachment) string {
	if stored.StorageDriver == common.R2AttachmentStorageDriver {
		baseURL := strings.TrimRight(r2PublicBaseURL, "/")
		if baseURL != "" && stored.StorageKey != "" {
			return baseURL + "/" + strings.TrimLeft(stored.StorageKey, "/")
		}
	}
	baseURL := strings.TrimRight(publicBaseURL, "/")
	if baseURL == "" {
		scheme := c.GetHeader("X-Forwarded-Proto")
		if scheme == "" {
			if c.Request.TLS != nil {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}
		host := c.GetHeader("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}
		baseURL = scheme + "://" + host
	}
	return baseURL + "/attachments/" + stored.PublicID + "." + stored.Extension
}

func writeUploadThreadError(c *gin.Context, err error) {
	if errors.Is(err, threadservice.ErrMessageQuotaExceeded) {
		c.JSON(http.StatusConflict, common.Response{Success: false, Message: "message quota exceeded"})
		return
	}
	c.JSON(http.StatusBadRequest, common.Response{Success: false, Message: "upload thread failed"})
}

func PublicRuntimeConfig(cfg Config) map[string]bool {
	return map[string]bool{
		"cloud_billing": commercialEnabled && !cfg.SelfHosted,
		"self_hosted":   cfg.SelfHosted,
		"dev_mode":      cfg.DevMode,
		"github_oauth":  cfg.GitHub.ClientID != "" && cfg.GitHub.CallbackURL != "",
		"google_oauth":  cfg.Google.ClientID != "" && cfg.Google.CallbackURL != "",
	}
}

func statusForAttachmentError(err error) int {
	switch err {
	case attachmentservice.ErrUnsupportedMediaType:
		return http.StatusUnsupportedMediaType
	case attachmentservice.ErrTooLarge:
		return http.StatusRequestEntityTooLarge
	case attachmentservice.ErrInvalidPayload, attachmentservice.ErrInvalidBase64, attachmentservice.ErrMediaTypeMismatch, attachmentservice.ErrUserRequired:
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func newOAuthState() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func threadListOptionsFromQuery(c *gin.Context) (threadservice.ListOptions, error) {
	archived, err := optionalBool(c.Query("archived"))
	if err != nil {
		return threadservice.ListOptions{}, err
	}
	return threadservice.ListOptions{
		Search:     c.Query("search"),
		Provider:   c.Query("provider"),
		Repo:       c.Query("repo"),
		Label:      c.Query("label"),
		Visibility: c.Query("visibility"),
		Archived:   archived,
		Sort:       c.Query("sort"),
		Page:       queryInt(c, "page"),
		PageSize:   queryInt(c, "page_size"),
	}, nil
}

func threadFindOptionsFromQuery(c *gin.Context) threadservice.FindOptions {
	return threadservice.FindOptions{
		Query:    c.Query("q"),
		Provider: c.Query("provider"),
		Limit:    queryInt(c, "limit"),
		Offset:   queryInt(c, "offset"),
	}
}

func queryInt(c *gin.Context, key string) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return 0
	}
	return value
}

func optionalBool(value string) (*bool, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

type userUploadGate struct {
	sync.Mutex
	byUserID map[int64]*userUploadGateEntry
}

type userUploadGateEntry struct {
	permit chan struct{}
	refs   int
}

func newUserUploadGate() *userUploadGate {
	return &userUploadGate{byUserID: make(map[int64]*userUploadGateEntry)}
}

func (gate *userUploadGate) acquire(ctx context.Context, userID int64) (func(), error) {
	gate.Lock()
	entry := gate.byUserID[userID]
	if entry == nil {
		entry = &userUploadGateEntry{permit: make(chan struct{}, 1)}
		entry.permit <- struct{}{}
		gate.byUserID[userID] = entry
	}
	entry.refs++
	gate.Unlock()

	if err := ctx.Err(); err != nil {
		gate.releaseRef(userID, entry)
		return nil, err
	}
	select {
	case <-ctx.Done():
		gate.releaseRef(userID, entry)
		return nil, ctx.Err()
	case <-entry.permit:
		var once sync.Once
		return func() {
			once.Do(func() {
				entry.permit <- struct{}{}
				gate.releaseRef(userID, entry)
			})
		}, nil
	}
}

func (gate *userUploadGate) releaseRef(userID int64, entry *userUploadGateEntry) {
	gate.Lock()
	defer gate.Unlock()
	entry.refs--
	if entry.refs == 0 && gate.byUserID[userID] == entry {
		delete(gate.byUserID, userID)
	}
}

type uploadRateLimiter struct {
	limit       int
	window      time.Duration
	mu          sync.Mutex
	keys        map[string]uploadRateWindow
	lastCleanup time.Time
}

type uploadRateWindow struct {
	start time.Time
	count int
}

func newUploadRateLimiter(limit int, window time.Duration) *uploadRateLimiter {
	return &uploadRateLimiter{limit: limit, window: window, keys: map[string]uploadRateWindow{}}
}

func (limiter *uploadRateLimiter) allow(userID int64, now time.Time) bool {
	return limiter.allowKey(strconv.FormatInt(userID, 10), now)
}

func (limiter *uploadRateLimiter) allowThread(userID int64, provider, providerThreadID string, now time.Time) bool {
	return limiter.allowKey(strconv.FormatInt(userID, 10)+"\x00"+provider+"\x00"+providerThreadID, now)
}

func (limiter *uploadRateLimiter) allowKey(key string, now time.Time) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if limiter.lastCleanup.IsZero() || now.Sub(limiter.lastCleanup) >= limiter.window {
		for existingKey, existingWindow := range limiter.keys {
			if now.Sub(existingWindow.start) >= limiter.window {
				delete(limiter.keys, existingKey)
			}
		}
		limiter.lastCleanup = now
	}
	window := limiter.keys[key]
	if window.start.IsZero() || now.Sub(window.start) >= limiter.window {
		limiter.keys[key] = uploadRateWindow{start: now, count: 1}
		return true
	}
	if window.count >= limiter.limit {
		return false
	}
	window.count++
	limiter.keys[key] = window
	return true
}
