//go:build !cloud

package route

import (
	"errors"
	"github.com/gin-gonic/gin"
)

const commercialEnabled = false

// PrepareCommercial rejects hosted billing in a core-only build.
func PrepareCommercial(mode string) error {
	if mode == "cloud" {
		return errors.New("cloud deployment requires a build with -tags cloud")
	}
	return nil
}
func registerCommercial(api *gin.RouterGroup, cfg Config) {}
