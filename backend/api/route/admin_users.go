package route

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	accountservice "github.com/jieliapp/jieli/backend/service/account"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

func registerAdminUserEditing(admin *gin.RouterGroup, login func(*gin.Context, *model.User)) {
	admin.PUT("/users/:id", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(400, common.Response{Message: "invalid user ID"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		var input struct {
			Email       string  `json:"email"`
			Username    string  `json:"username"`
			DisplayName *string `json:"display_name"`
			Password    string  `json:"password"`
		}
		if c.ShouldBindJSON(&input) != nil || input.DisplayName == nil {
			c.JSON(400, common.Response{Message: "email, username and display name required"})
			return
		}
		email, err := accountservice.Email(input.Email)
		if err != nil {
			c.JSON(400, common.Response{Message: err.Error()})
			return
		}
		name := strings.TrimSpace(*input.DisplayName)
		if utf8.RuneCountInString(name) > 80 {
			c.JSON(400, common.Response{Message: "display name must not exceed 80 characters"})
			return
		}
		hash := ""
		if input.Password != "" {
			if len(input.Password) < 8 {
				c.JSON(400, common.Response{Message: "temporary password must contain at least 8 characters"})
				return
			}
			hash, err = accountservice.PasswordHash(input.Password)
			if err != nil {
				c.JSON(400, common.Response{Message: err.Error()})
				return
			}
		}
		updated, status, err := accountservice.Edit(c.Request.Context(), id, middleware.CurrentSessionUser(c).ID, email, input.Username, name, hash)
		if err != nil {
			message := err.Error()
			if status == 500 {
				message = "user update failed"
			}
			c.JSON(status, common.Response{Message: message})
			return
		}
		if id == middleware.CurrentSessionUser(c).ID && hash != "" {
			login(c, updated)
			return
		}
		common.OK(c, userservice.SafeUser(updated))
	})
}
