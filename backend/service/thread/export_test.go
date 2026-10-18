package thread_test

import (
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/thread"
)

func TestExportMarkdownShouldTruncateToolResultsWhenRequested(t *testing.T) {
	detail := thread.Detail{
		Provider:         model.ProviderClaudeCode,
		ProviderThreadID: "T-export-tool-result",
		Title:            "Tool result export",
		Messages: []byte(`[
			{"role":"assistant","content":[{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]},
			{"role":"tool","content":[{"type":"tool_result","content":"very long stdout that should not be included"}]}
		]`),
	}

	markdown := thread.ExportMarkdownWithOptions(detail, thread.ExportMarkdownOptions{TruncateToolResults: true})

	if !strings.Contains(markdown, "### Tool: Bash") {
		t.Fatalf("expected tool use heading, got %s", markdown)
	}
	if !strings.Contains(markdown, "### Tool result\n\n[tool result truncated]") {
		t.Fatalf("expected truncated tool result placeholder, got %s", markdown)
	}
	if strings.Contains(markdown, "very long stdout") {
		t.Fatalf("expected tool result stdout to be omitted, got %s", markdown)
	}
}
