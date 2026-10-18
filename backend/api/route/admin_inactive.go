package route

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/common"
	accountservice "github.com/jieliapp/jieli/backend/service/account"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

func registerAdminUserInactivity(admin *gin.RouterGroup) {
	admin.PUT("/users/:id/inactive", func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(400, common.Response{Message: "invalid user ID"})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024)
		var input struct {
			Inactive *bool `json:"inactive"`
		}
		if c.ShouldBindJSON(&input) != nil || input.Inactive == nil {
			c.JSON(400, common.Response{Message: "inactive boolean required"})
			return
		}
		updated, status, err := accountservice.SetInactive(c.Request.Context(), id, middleware.CurrentSessionUser(c).ID, *input.Inactive)
		if err != nil {
			message := "user status update failed"
			if status == 404 || status == 400 {
				message = err.Error()
			}
			c.JSON(status, common.Response{Message: message})
			return
		}
		common.OK(c, userservice.SafeUser(updated))
	})
}
