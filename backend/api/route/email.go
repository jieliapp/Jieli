package route

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	accountservice "github.com/jieliapp/jieli/backend/service/account"
	authservice "github.com/jieliapp/jieli/backend/service/auth"
)

func registerEmailRoutes(auth, admin *gin.RouterGroup, cfg Config) {
	send := cfg.SendMail
	if send == nil {
		send = common.SendSMTP
	}
	fail := func(c *gin.Context, status int, message string) { c.JSON(status, common.Response{Message: message}) }
	auth.GET("/options", func(c *gin.Context) {
		settings, err := loadSiteSettings()
		if err != nil {
			fail(c, 500, "email settings unavailable")
			return
		}
		count, err := model.UserDB.Where("is_admin = ?", true).Count()
		if err != nil {
			fail(c, 500, "account settings unavailable")
			return
		}
		common.OK(c, gin.H{"registration_enabled": settings.RegistrationEnabled, "recovery_enabled": settings.Tested && settings.PublicBaseURL != "", "setup_available": count == 0 && cfg.SetupToken != ""})
	})

	getSiteSettings := func(c *gin.Context) {
		settings, err := loadSiteSettings()
		if err != nil {
			fail(c, 500, "site settings unavailable")
			return
		}
		common.OK(c, gin.H{"public_base_url": settings.PublicBaseURL, "host": settings.Host, "port": settings.Port, "username": settings.Username, "from": settings.From, "tls_mode": settings.TLSMode, "tested": settings.Tested, "registration_enabled": settings.RegistrationEnabled, "has_password": settings.PasswordEncrypted != ""})
	}
	admin.GET("/site-settings", getSiteSettings)
	admin.POST("/site-settings", func(c *gin.Context) {
		var input struct {
			PublicBaseURL string `json:"public_base_url"`
		}
		if c.ShouldBindJSON(&input) != nil {
			fail(c, 400, "invalid site settings")
			return
		}
		baseURL := strings.TrimRight(strings.TrimSpace(input.PublicBaseURL), "/")
		if baseURL != "" {
			parsed, parseErr := url.Parse(baseURL)
			if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
				fail(c, 400, "invalid public site URL")
				return
			}
		}
		if err := model.UpdateSiteSettings(c.Request.Context(), func(settings *model.SiteSettingsValues) error {
			settings.PublicBaseURL = baseURL
			if baseURL == "" {
				settings.RegistrationEnabled = false
			}
			return nil
		}); err != nil {
			fail(c, 500, "site settings save failed")
			return
		}
		common.OK(c, gin.H{"public_base_url": baseURL})
	})
	admin.POST("/smtp/test", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		var input common.SMTPConfig
		if c.ShouldBindJSON(&input) != nil {
			fail(c, 400, "invalid SMTP configuration")
			return
		}
		if err := input.Validate(); err != nil {
			fail(c, 400, err.Error())
			return
		}
		settings, err := loadSiteSettings()
		if err != nil {
			fail(c, 500, "email settings unavailable")
			return
		}
		if input.Password == "" {
			if settings.PasswordEncrypted == "" {
				fail(c, 400, "SMTP password required")
				return
			}
			input.Password, err = common.DecryptSecret(settings.PasswordEncrypted, cfg.APIKeyEncryptionSecret)
			if err != nil {
				fail(c, 500, "SMTP credentials unavailable")
				return
			}
		}
		if err := send(c.Request.Context(), input, middleware.CurrentSessionUser(c).Email, "Jieli SMTP connection test succeeded."); err != nil {
			fail(c, 502, "SMTP test failed; configuration was not saved")
			return
		}
		encrypted, err := common.EncryptSecret(input.Password, cfg.APIKeyEncryptionSecret)
		if err != nil {
			fail(c, 500, "SMTP configuration encryption failed")
			return
		}
		if err = model.UpdateSiteSettings(c.Request.Context(), func(settings *model.SiteSettingsValues) error {
			settings.Host = input.Host
			settings.Port = input.Port
			settings.Username = input.Username
			settings.PasswordEncrypted = encrypted
			settings.From = input.From
			settings.TLSMode = input.TLSMode
			settings.Tested = true
			settings.RegistrationEnabled = false
			return nil
		}); err != nil {
			fail(c, 500, "email settings save failed")
			return
		}
		common.OK(c, gin.H{"tested": true, "registration_enabled": false})
	})
	admin.POST("/registration", func(c *gin.Context) {
		var input struct {
			Enabled bool `json:"enabled"`
		}
		if c.ShouldBindJSON(&input) != nil {
			fail(c, 400, "invalid registration setting")
			return
		}
		prerequisiteErr := errors.New("tested SMTP and public site URL are required")
		err := model.UpdateSiteSettings(c.Request.Context(), func(settings *model.SiteSettingsValues) error {
			if input.Enabled && (!settings.Tested || settings.PublicBaseURL == "") {
				return prerequisiteErr
			}
			settings.RegistrationEnabled = input.Enabled
			return nil
		})
		if errors.Is(err, prerequisiteErr) {
			fail(c, 400, prerequisiteErr.Error())
			return
		}
		if err != nil {
			fail(c, 500, "email settings save failed")
			return
		}
		common.OK(c, gin.H{"enabled": input.Enabled})
	})
	requestToken := func(purpose string) gin.HandlerFunc {
		return func(c *gin.Context) {
			settings, err := loadSiteSettings()
			if err != nil {
				fail(c, 500, "email settings unavailable")
				return
			}
			if !settings.Tested || purpose == "register" && !settings.RegistrationEnabled {
				fail(c, 403, "email registration or recovery is disabled")
				return
			}
			if settings.PublicBaseURL == "" {
				fail(c, 400, "public site URL is required")
				return
			}
			var input accountservice.Input
			if c.ShouldBindJSON(&input) != nil {
				fail(c, 400, "invalid email request")
				return
			}
			email, err := accountservice.Email(input.Email)
			if err != nil {
				fail(c, 400, err.Error())
				return
			}
			hash := ""
			if purpose == "register" {
				input.DisplayName = strings.TrimSpace(input.DisplayName)
				if utf8.RuneCountInString(input.DisplayName) > 80 {
					fail(c, 400, "display name must not exceed 80 characters")
					return
				}
				hash, err = accountservice.PasswordHash(input.Password)
				if err != nil {
					fail(c, 400, err.Error())
					return
				}
			}
			token, err := accountservice.IssueToken(email, purpose, input.DisplayName, hash)
			if err != nil {
				fail(c, 500, "email request failed")
				return
			}
			if token == "" {
				common.OK(c, gin.H{"sent": true})
				return
			}
			password, err := common.DecryptSecret(settings.PasswordEncrypted, cfg.APIKeyEncryptionSecret)
			if err != nil {
				fail(c, 500, "SMTP credentials unavailable")
				return
			}
			link := strings.TrimRight(settings.PublicBaseURL, "/") + "/web/auth/email?purpose=" + purpose + "#" + token
			if err = send(c.Request.Context(), common.SMTPConfig{Host: settings.Host, Port: settings.Port, Username: settings.Username, Password: password, From: settings.From, TLSMode: settings.TLSMode}, email, link); err != nil {
				fail(c, 502, "verification email could not be sent")
				return
			}
			common.OK(c, gin.H{"sent": true})
		}
	}
	auth.POST("/register", requestToken("register"))
	auth.POST("/reset", requestToken("reset"))
	auth.POST("/email/complete", func(c *gin.Context) {
		var input accountservice.Input
		if c.ShouldBindJSON(&input) != nil || len(input.Token) != 64 {
			fail(c, 400, "invalid verification token")
			return
		}
		err := (accountservice.Service{EncryptionSecret: cfg.APIKeyEncryptionSecret}).CompleteToken(c.Request.Context(), input)
		if err != nil {
			if errors.Is(err, authservice.ErrInactiveUser) {
				fail(c, 401, "account inactive")
				return
			}
			fail(c, 400, "verification failed; token may be expired or already used")
			return
		}
		common.OK(c, gin.H{"verified": true})
	})
}
