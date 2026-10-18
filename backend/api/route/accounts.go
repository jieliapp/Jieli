package route

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	accountservice "github.com/jieliapp/jieli/backend/service/account"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

func registerAccounts(api *gin.RouterGroup, cfg Config) {
	accounts := accountservice.Service{EncryptionSecret: cfg.APIKeyEncryptionSecret}
	limiter := newUploadRateLimiter(10, time.Minute)
	auth := api.Group("/auth")
	auth.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		if !limiter.allowKey(c.ClientIP(), time.Now()) {
			c.AbortWithStatusJSON(429, common.Response{Message: "too many attempts"})
			return
		}
		c.Next()
	})
	respondError := func(c *gin.Context, status int, message string) { c.JSON(status, common.Response{Message: message}) }
	login := func(c *gin.Context, user *model.User) {
		cookie, err := middleware.NewSessionCookieForUser(user, cfg.SessionSecret)
		if err != nil {
			respondError(c, 500, "session unavailable")
			return
		}
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(middleware.SessionCookieName, cookie, middleware.SessionMaxAge, "/", "", cfg.CookieSecure, true)
		common.OK(c, userservice.SafeUser(user))
	}
	auth.POST("/setup", func(c *gin.Context) {
		var input struct {
			accountservice.Input
			PasswordConfirm string `json:"password_confirm"`
		}
		if c.ShouldBindJSON(&input) != nil {
			respondError(c, 400, "invalid setup request")
			return
		}
		if !cfg.SelfHosted && (cfg.SetupToken == "" || subtle.ConstantTimeCompare([]byte(input.Token), []byte(cfg.SetupToken)) != 1) {
			respondError(c, 403, "setup unavailable")
			return
		}
		if cfg.SelfHosted && (input.PasswordConfirm == "" || input.PasswordConfirm != input.Password) {
			respondError(c, 400, "password confirmation must match")
			return
		}
		user, err := accounts.Create(c.Request.Context(), input.Input, true, false)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, accountservice.ErrAlreadyInitialized) {
				status = http.StatusForbidden
			}
			respondError(c, status, err.Error())
			return
		}
		login(c, user)
	})
	auth.POST("/login", func(c *gin.Context) {
		var input accountservice.Input
		if c.ShouldBindJSON(&input) != nil {
			respondError(c, 400, "invalid login request")
			return
		}
		user, err := accountservice.Authenticate(input.Email, input.Password)
		if err != nil {
			respondError(c, 401, "invalid credentials")
			return
		}
		login(c, user)
	})
	auth.POST("/password", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure), func(c *gin.Context) {
		var input accountservice.Input
		user := middleware.CurrentSessionUser(c)
		if user.LoginEmail == "" {
			respondError(c, 400, "password changes require an email login account")
			return
		}
		if c.ShouldBindJSON(&input) != nil {
			respondError(c, 400, "invalid password request")
			return
		}
		hash, err := accountservice.PasswordHash(input.Password)
		if err != nil {
			respondError(c, 400, err.Error())
			return
		}
		changed, err := accountservice.ChangePassword(c.Request.Context(), user, hash)
		if err != nil {
			respondError(c, 500, "password update failed")
			return
		}
		if !changed {
			respondError(c, 409, "credentials changed; sign in again")
			return
		}
		user.PasswordHash = hash
		user.MustChangePassword = false
		login(c, user)
	})

	admin := api.Group("/admin", middleware.SessionAuth(cfg.SessionSecret, cfg.CookieSecure), middleware.RequireAdmin)
	registerEmailRoutes(auth, admin, cfg)
	registerAdminUserEditing(admin, login)
	registerAdminUserInactivity(admin)

	admin.POST("/users", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		var input accountservice.Input
		if c.ShouldBindJSON(&input) != nil {
			respondError(c, 400, "invalid user request")
			return
		}
		user, err := accounts.Create(c.Request.Context(), input, false, true)
		if err != nil {
			respondError(c, 400, err.Error())
			return
		}
		common.OK(c, userservice.SafeUser(user))
	})
	admin.GET("/users", func(c *gin.Context) {
		page := queryInt(c, "page")
		if page < 1 {
			page = 1
		}
		if page > 100000 {
			respondError(c, 400, "invalid page")
			return
		}
		rows, err := model.UserDB.Order("id DESC").WithDeleted().Fetch((page-1)*50, 50)
		if err != nil {
			respondError(c, 500, "users unavailable")
			return
		}
		users := make([]userservice.ProfileUser, 0, len(rows))
		for _, user := range rows {
			users = append(users, userservice.SafeUser(user))
		}
		common.OK(c, users)
	})
}
