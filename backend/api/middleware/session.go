package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	subscriptionservice "github.com/jieliapp/jieli/backend/service/subscription"
)

const (
	SessionCookieName     = "jieli_session"
	SessionMaxAge         = 30 * 24 * 3600 // 30 days
	currentSessionUserKey = "current_session_user"
)

func NewSessionCookie(userID int64, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("session secret is required")
	}
	user, err := model.UserDB.ByID(userID)
	if err != nil {
		return "", err
	}
	return NewSessionCookieForUser(user, secret)
}

// NewSessionCookieForUser signs only the credentials already verified by the caller.
func NewSessionCookieForUser(user *model.User, secret string) (string, error) {
	if user == nil || user.ID <= 0 || user.Deleted || secret == "" {
		return "", fmt.Errorf("user and session secret required")
	}
	payload := strconv.FormatInt(user.ID, 10) + ":" + strconv.FormatInt(time.Now().Unix()+SessionMaxAge, 10)
	return payload + "." + signSession(payload, secret+user.PasswordHash), nil
}

func SessionAuth(secret string, cookieSecure bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := ResolveSessionUser(c, secret)
		if !ok {
			unauthorized(c)
			return
		}
		if user.MustChangePassword && c.Request.URL.Path != "/web/api/auth/password" && c.Request.URL.Path != "/web/api/auth/me" && c.Request.URL.Path != "/web/api/auth/logout" {
			c.AbortWithStatusJSON(http.StatusForbidden, common.Response{Message: "password change required"})
			return
		}
		// Sliding renewal refreshes both the signed expiry and browser cookie.
		cookie, _ := NewSessionCookieForUser(user, secret)
		c.SetCookie(SessionCookieName, cookie, SessionMaxAge, "/", "", cookieSecure, true)
		c.Next()
	}
}

func ResolveSessionUser(c *gin.Context, secret string) (*model.User, bool) {
	cookie, err := c.Cookie(SessionCookieName)
	if err != nil {
		return nil, false
	}
	idText, _, _ := strings.Cut(cookie, ":")
	userID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return nil, false
	}
	user, err := model.UserDB.ByID(userID)
	if err != nil {
		return nil, false
	}
	if user.Deleted {
		return nil, false
	}
	if _, ok := parseSessionCookie(cookie, secret+user.PasswordHash); !ok || secret == "" {
		return nil, false
	}
	resolved := *user
	resolved.UnlimitedStorage = c.GetBool("unlimited_storage")
	user = &resolved
	if err := subscriptionservice.ApplyEntitlement(user, time.Now()); err != nil {
		return nil, false
	}
	c.Set(currentSessionUserKey, user)
	return user, true
}

func CurrentSessionUser(c *gin.Context) *model.User {
	value, ok := c.Get(currentSessionUserKey)
	if !ok {
		return nil
	}
	user, ok := value.(*model.User)
	if !ok {
		return nil
	}
	return user
}

func parseSessionCookie(cookie string, secret string) (int64, bool) {
	payload, signature, ok := strings.Cut(cookie, ".")
	if !ok || payload == "" || signature == "" || secret == "" {
		return 0, false
	}
	expected := signSession(payload, secret)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return 0, false
	}
	idText, expiryText, ok := strings.Cut(payload, ":")
	if !ok {
		return 0, false
	}
	expiry, err := strconv.ParseInt(expiryText, 10, 64)
	if err != nil || expiry <= time.Now().Unix() {
		return 0, false
	}
	userID, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		return 0, false
	}
	return userID, true
}

func signSession(payload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func unauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, common.Response{Success: false, Message: "unauthorized"})
}

// RequireAdmin authorizes an already authenticated session for administrator routes.
func RequireAdmin(c *gin.Context) {
	user := CurrentSessionUser(c)
	if user == nil || !user.IsAdmin {
		c.AbortWithStatusJSON(403, common.Response{Message: "administrator required"})
		return
	}
	c.Next()
}
