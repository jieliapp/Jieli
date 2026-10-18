package route

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jieliapp/jieli/backend/api/middleware"
	"github.com/jieliapp/jieli/backend/common"
	"github.com/jieliapp/jieli/backend/model"
	threadservice "github.com/jieliapp/jieli/backend/service/thread"
)

func ServeThreadExport(c *gin.Context, cfg Config) bool {
	providerThreadID, format, ok := parseThreadExportPath(c.Request.URL.Path)
	if !ok {
		return false
	}
	user, ok := middleware.ResolveAPIKeyUser(c)
	if !ok {
		user, ok = middleware.ResolveSessionUser(c, cfg.SessionSecret)
	}
	detail, publicExport, err := exportThreadDetail(user, providerThreadID)
	if err != nil {
		if user == nil {
			respondUnauthorizedThreadExport(c, format, providerThreadID)
			return true
		}
		c.JSON(http.StatusNotFound, common.Response{Success: false, Message: "thread not found"})
		return true
	}
	if publicExport {
		c.Header("X-Robots-Tag", "noindex")
	}
	switch format {
	case "json":
		c.JSON(http.StatusOK, detail)
	case "md":
		c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(threadservice.ExportMarkdownWithOptions(detail, threadservice.ExportMarkdownOptions{TruncateToolResults: c.Query("truncate_tool_results") == "1"})))
	default:
		return false
	}
	return true
}

func exportThreadDetail(user *model.User, providerThreadID string) (threadservice.Detail, bool, error) {
	if user != nil {
		if detail, err := threadservice.GetThreadDetailByProviderID(user, providerThreadID); err == nil {
			return detail, false, nil
		}
	}
	detail, err := threadservice.GetPublicThreadDetailByProviderID(providerThreadID)
	return detail, true, err
}

func respondUnauthorizedThreadExport(c *gin.Context, format string, providerThreadID string) {
	message := "This Jieli thread is private. If you are an agent with the Jieli plugin installed, do not use WebFetch or curl for this URL. Use the jieli skill to read the thread, or fetch /threads/" + providerThreadID + ".md with Authorization: Bearer $JIELI_API_KEY."
	if format == "md" {
		c.Data(http.StatusUnauthorized, "text/markdown; charset=utf-8", []byte("# Jieli thread requires plugin access\n\n"+message+"\n"))
		return
	}
	c.JSON(http.StatusUnauthorized, gin.H{
		"success": false,
		"message": message,
		"hint":    "Use the installed Jieli jieli skill for /threads/T-... links instead of WebFetch/curl.",
	})
}

func parseThreadExportPath(rawPath string) (string, string, bool) {
	value := strings.TrimPrefix(rawPath, "/threads/")
	if value == rawPath || value == "" || strings.Contains(value, "/") {
		return "", "", false
	}
	if providerThreadID, ok := strings.CutSuffix(value, ".json"); ok && providerThreadID != "" {
		return providerThreadID, "json", true
	}
	if providerThreadID, ok := strings.CutSuffix(value, ".md"); ok && providerThreadID != "" {
		return providerThreadID, "md", true
	}
	return "", "", false
}
