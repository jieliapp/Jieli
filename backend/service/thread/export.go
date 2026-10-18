package thread

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ExportMarkdownOptions struct {
	TruncateToolResults bool
}

func ExportMarkdown(detail Detail) string {
	return ExportMarkdownWithOptions(detail, ExportMarkdownOptions{})
}

func ExportMarkdownWithOptions(detail Detail, options ExportMarkdownOptions) string {
	lines := []string{
		"# " + firstNonEmpty(detail.Title, detail.ProviderThreadID),
		"",
		"- Provider: " + detail.Provider,
		"- Thread ID: " + detail.ProviderThreadID,
	}
	if detail.Repo != "" {
		lines = append(lines, "- Repo: "+detail.Repo)
	}
	if detail.Branch != "" {
		lines = append(lines, "- Branch: "+detail.Branch)
	}
	lines = append(lines, "", "---", "")

	var messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(detail.Messages, &messages); err == nil {
		for _, message := range messages {
			body := markdownForContentWithOptions(message.Content, options)
			if body == "" {
				continue
			}
			lines = append(lines, "## "+firstNonEmpty(message.Role, "message"), "", body, "")
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

func markdownForContent(raw json.RawMessage) string {
	return markdownForContentWithOptions(raw, ExportMarkdownOptions{})
}

func markdownForContentWithOptions(raw json.RawMessage, options ExportMarkdownOptions) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var blocks []json.RawMessage
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		part := markdownForBlockWithOptions(block, options)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n\n")
}

func markdownForBlock(raw json.RawMessage) string {
	return markdownForBlockWithOptions(raw, ExportMarkdownOptions{})
}

func markdownForBlockWithOptions(raw json.RawMessage, options ExportMarkdownOptions) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var block struct {
		Type     string          `json:"type"`
		Text     string          `json:"text"`
		Thinking string          `json:"thinking"`
		Name     string          `json:"name"`
		Input    json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(raw, &block); err != nil {
		return ""
	}
	if block.Text != "" {
		return strings.TrimSpace(block.Text)
	}
	if block.Thinking != "" {
		return strings.TrimSpace(block.Thinking)
	}
	if block.Type == "tool_use" {
		input := block.Input
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		return fmt.Sprintf("### Tool: %s\n\n```json\n%s\n```", firstNonEmpty(block.Name, "tool"), prettyJSON(input))
	}
	if block.Type == "tool_result" {
		if options.TruncateToolResults {
			return "### Tool result\n\n[tool result truncated]"
		}
		return "### Tool result\n\n```json\n" + prettyJSON(raw) + "\n```"
	}
	return ""
}

func prettyJSON(raw json.RawMessage) string {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(pretty)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
