package route_test

import (
	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/route"
	"strings"
	"testing"
)

func TestStandaloneDoesNotRegisterRetiredServices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	route.Register(router)
	for _, registered := range router.Routes() {
		for _, prefix := range []string{"/api/", "/v1/", "/neo/", "/actors/", "/web/api/neo/"} {
			if strings.HasPrefix(registered.Path, prefix) {
				t.Fatalf("retired route remains registered: %s %s", registered.Method, registered.Path)
			}
		}
	}
}
