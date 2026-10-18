package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	subscriptionservice "github.com/jieliapp/jieli/backend/service/subscription"
	"github.com/gin-gonic/gin"
)

const currentAPIUserKey = "current_api_user"

func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := ResolveAPIKeyUser(c); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, common.Response{Success: false, Message: "invalid api key"})
			return
		}
		c.Next()
	}
}

func ResolveAPIKeyUser(c *gin.Context) (*model.User, bool) {
	key := bearerToken(c.GetHeader("Authorization"))
	prefix := common.APIKeyPublicPrefix(key)
	if key == "" || prefix == "" {
		return nil, false
	}

	users, err := model.UserDB.Where("api_key_prefix = ?", prefix).Fetch(0, 2)
	if err != nil || len(users) != 1 {
		return nil, false
	}
	user := users[0]
	resolved := *user
	resolved.UnlimitedStorage = c.GetBool("unlimited_storage")
	user = &resolved
	if user.Deleted || user.MustChangePassword || !common.VerifyAPIKey(key, user.APIKeyHash) {
		return nil, false
	}
	if err := subscriptionservice.ApplyEntitlement(user, time.Now()); err != nil {
		return nil, false
	}
	c.Set(currentAPIUserKey, user)
	return user, true
}

func CurrentAPIUser(c *gin.Context) *model.User {
	value, ok := c.Get(currentAPIUserKey)
	if !ok {
		return nil
	}
	user, ok := value.(*model.User)
	if !ok {
		return nil
	}
	return user
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
