package thread_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/thread"
)

func mustRawJSON(t *testing.T, message *model.ThreadMessage) string {
	t.Helper()
	rawJSON, err := message.RawJSON()
	if err != nil {
		t.Fatalf("decode raw json: %v", err)
	}
	return rawJSON
}

func legacyUploadFingerprintForTest(provider string, providerThreadID string, payload thread.UploadPayload) string {
	labels := make([]string, 0, len(payload.Labels))
	for _, label := range payload.Labels {
		label = strings.TrimSpace(label)
		if label != "" {
			labels = append(labels, label)
		}
	}
	sort.Strings(labels)
	raw, _ := json.Marshal(map[string]any{
		"provider":           provider,
		"provider_thread_id": providerThreadID,
		"thread":             json.RawMessage(payload.Thread),
		"repo":               payload.Repo,
		"repo_url":           payload.RepoURL,
		"branch":             payload.Branch,
		"source_url":         payload.SourceURL,
		"labels":             labels,
	})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func TestUploadThreadSnapshotShouldRedactSecretsBeforeStore(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread: json.RawMessage(`{
			"id":"T-redact-service",
			"title":"Fix leaked key sk-proj-titleSECRET12345678901234567890",
			"messages":[
				{"role":"user","message_id":"m-1","content":"please hide sk-proj-userSECRET12345678901234567890 and https://user:secret@example.com:notaport/path?token=query-secret"},
				{"role":"assistant","message_id":"m-2","content":[{"type":"tool_use","name":"edit_file","input":{"path":"/repo/a.go","new_str":"const token = \"jieli_abcdefghijklmnopqrstuvwxyz1234567890\""}}]}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("upload codex snapshot: %v", err)
	}

	for field, value := range map[string]string{
		"title":      stored.Title,
		"preview":    stored.Preview,
		"searchText": stored.SearchText,
	} {
		if strings.Contains(value, "sk-proj-titleSECRET") || strings.Contains(value, "sk-proj-userSECRET") || strings.Contains(value, "query-secret") || strings.Contains(value, "jieli_abcdefghijklmnopqrstuvwxyz") {
			t.Fatalf("expected %s to be redacted, got %q", field, value)
		}
	}
	if !strings.Contains(stored.Title, "[REDACTED:openai-api-key]") {
		t.Fatalf("expected title redaction marker, got %q", stored.Title)
	}
	if !strings.Contains(stored.Preview, "[REDACTED:openai-api-key]") {
		t.Fatalf("expected preview redaction marker, got %q", stored.Preview)
	}
	if !strings.Contains(stored.SearchText, "[REDACTED:url-query-token]") {
		t.Fatalf("expected search text redaction marker, got %q", stored.SearchText)
	}

	messages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Order("position ASC").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected two messages, got %d", len(messages))
	}
	for _, message := range messages {
		raw := mustRawJSON(t, message)
		if strings.Contains(raw, "sk-proj-userSECRET") || strings.Contains(raw, "query-secret") || strings.Contains(raw, "jieli_abcdefghijklmnopqrstuvwxyz") || strings.Contains(raw, "user:secret") {
			t.Fatalf("expected raw message to be redacted, got %s", raw)
		}
	}
	if !strings.Contains(mustRawJSON(t, messages[0]), "[REDACTED:url-userinfo]@example.com:notaport") || !strings.Contains(mustRawJSON(t, messages[0]), "token=[REDACTED:url-query-token]") {
		t.Fatalf("expected malformed-port URL to preserve host and redact secret, got %s", mustRawJSON(t, messages[0]))
	}
	if !strings.Contains(mustRawJSON(t, messages[1]), "[REDACTED:jieli-api-key]") {
		t.Fatalf("expected jieli api key marker, got %s", mustRawJSON(t, messages[1]))
	}
}

func TestUploadIdentityShouldNormalizeProviderID(t *testing.T) {
	provider, providerThreadID, err := thread.UploadIdentity(thread.UploadPayload{Provider: model.ProviderClaudeCode, Thread: json.RawMessage(`{"id":"raw-uuid","messages":[]}`)})
	if err != nil {
		t.Fatalf("upload identity: %v", err)
	}
	if provider != model.ProviderClaudeCode || providerThreadID != "T-raw-uuid" {
		t.Fatalf("upload identity = %q/%q, want claude_code/T-raw-uuid", provider, providerThreadID)
	}
}

func TestUploadIdentityShouldRejectUnsupportedProviderAndMissingThreadID(t *testing.T) {
	if _, _, err := thread.UploadIdentity(thread.UploadPayload{Provider: "unknown", Thread: json.RawMessage(`{"id":"T-any"}`)}); err == nil {
		t.Fatal("unsupported provider should be rejected")
	}
	if _, _, err := thread.UploadIdentity(thread.UploadPayload{Provider: model.ProviderAmp, Thread: json.RawMessage(`{"messages":[]}`)}); err == nil {
		t.Fatal("missing thread ID should be rejected")
	}
}

func TestUploadThreadSnapshotShouldRedactUploadMetadataBeforeStoreAndFingerprint(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	payloadWithMetadataSecret := func(secret string) thread.UploadPayload {
		return thread.UploadPayload{
			Provider:  model.ProviderCodex,
			Thread:    json.RawMessage(`{"id":"T-redact-metadata","title":"Safe title","messages":[{"role":"user","message_id":"m-1","content":"safe content"}]}`),
			Repo:      "owner/" + secret,
			RepoURL:   "https://user:" + secret + "@example.com/owner/repo.git",
			Branch:    "feature/" + secret,
			SourceURL: "https://example.com/thread?token=" + secret,
			Labels:    []string{"label-" + secret},
		}
	}

	first, err := thread.UploadThreadSnapshot(user, payloadWithMetadataSecret("sk-proj-metaFIRST12345678901234567890"))
	if err != nil {
		t.Fatalf("upload first snapshot: %v", err)
	}
	second, err := thread.UploadThreadSnapshot(user, payloadWithMetadataSecret("sk-proj-metaSECOND1234567890123456789"))
	if err != nil {
		t.Fatalf("upload second snapshot: %v", err)
	}
	if first.UploadFingerprint != second.UploadFingerprint {
		t.Fatalf("expected metadata fingerprint to be based on redacted payload, got %q then %q", first.UploadFingerprint, second.UploadFingerprint)
	}
	for field, value := range map[string]string{
		"repo":       second.Repo,
		"repoURL":    second.RepoURL,
		"branch":     second.Branch,
		"sourceURL":  second.SourceURL,
		"searchText": second.SearchText,
	} {
		if strings.Contains(value, "sk-proj-meta") {
			t.Fatalf("expected %s metadata to be redacted, got %q", field, value)
		}
	}
	labels, err := model.ThreadLabelDB.Where("thread_id = ?", second.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch labels: %v", err)
	}
	if len(labels) != 1 || strings.Contains(labels[0].Label, "sk-proj-meta") || !strings.Contains(labels[0].Label, "[REDACTED:openai-api-key]") {
		t.Fatalf("expected redacted label, got %#v", labels)
	}
}

func TestUploadThreadSnapshotShouldFingerprintRedactedPayload(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	payloadWithSecret := func(secret string) thread.UploadPayload {
		return thread.UploadPayload{
			Provider: model.ProviderCodex,
			Thread:   json.RawMessage(`{"id":"T-redacted-fingerprint","title":"Leak ` + secret + `","messages":[{"role":"user","message_id":"m-1","content":"hide ` + secret + `"}]}`),
		}
	}
	first, err := thread.UploadThreadSnapshot(user, payloadWithSecret("sk-proj-firstSECRET12345678901234567890"))
	if err != nil {
		t.Fatalf("upload first amp snapshot: %v", err)
	}
	second, err := thread.UploadThreadSnapshot(user, payloadWithSecret("sk-proj-secondSECRET1234567890123456789"))
	if err != nil {
		t.Fatalf("upload second amp snapshot: %v", err)
	}
	if first.UploadFingerprint != second.UploadFingerprint {
		t.Fatalf("expected fingerprint to be based on redacted payload, got %q then %q", first.UploadFingerprint, second.UploadFingerprint)
	}
}

func TestUploadThreadSnapshotShouldRedactSecretsForNonAmpProviders(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	cases := []struct {
		name     string
		provider string
		thread   json.RawMessage
	}{
		{
			name:     "claude code",
			provider: model.ProviderClaudeCode,
			thread:   json.RawMessage(`{"id":"cc-redact","title":"Leak sk-proj-claudeTITLE12345678901234567890","created_ms":1730000000000,"updated_ms":1730000000000,"messages":[{"role":"user","message_id":"u-1","content":"hide sk-proj-claudeBODY12345678901234567890"}]}`),
		},
		{
			name:     "codex",
			provider: model.ProviderCodex,
			thread:   json.RawMessage(`{"id":"codex-redact","title":"Leak sk-proj-codexTITLE12345678901234567890","created_ms":1730000000000,"updated_ms":1730000000000,"messages":[{"role":"user","message_id":"u-1","content":"hide sk-proj-codexBODY12345678901234567890"}]}`),
		},
		{
			name:     "opencode",
			provider: model.ProviderOpenCode,
			thread:   json.RawMessage(`{"id":"opencode-redact","title":"Leak sk-proj-openTITLE12345678901234567890","created_ms":1730000000000,"updated_ms":1730000000000,"messages":[{"role":"user","message_id":"u-1","content":"hide sk-proj-openBODY12345678901234567890"}]}`),
		},
		{
			name:     "pi",
			provider: model.ProviderPi,
			thread:   json.RawMessage(`{"id":"pi-redact","title":"Leak sk-proj-piTITLE12345678901234567890","created_ms":1730000000000,"updated_ms":1730000000000,"messages":[{"role":"user","message_id":"u-1","content":"hide sk-proj-piBODY12345678901234567890"}]}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{Provider: tc.provider, Thread: tc.thread})
			if err != nil {
				t.Fatalf("upload snapshot: %v", err)
			}
			if strings.Contains(stored.Title, "sk-proj-") || !strings.Contains(stored.Title, "[REDACTED:openai-api-key]") {
				t.Fatalf("expected title redacted, got %q", stored.Title)
			}
			if strings.Contains(stored.SearchText, "sk-proj-") {
				t.Fatalf("expected search text redacted, got %q", stored.SearchText)
			}
			messages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
			if err != nil {
				t.Fatalf("fetch messages: %v", err)
			}
			if len(messages) != 1 {
				t.Fatalf("expected one message, got %d", len(messages))
			}
			raw := mustRawJSON(t, messages[0])
			if strings.Contains(raw, "sk-proj-") || !strings.Contains(raw, "[REDACTED:openai-api-key]") {
				t.Fatalf("expected raw message redacted, got %s", raw)
			}
		})
	}
}

func TestUploadThreadSnapshotShouldReprocessLegacyUnredactedFingerprint(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	payload := thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-legacy-redact","title":"Leak sk-proj-legacySECRET12345678901234567890","messages":[{"role":"user","message_id":"m-1","content":"hide sk-proj-legacySECRET12345678901234567890"}]}`),
	}
	stored := &model.Thread{
		UserID:            user.ID,
		Provider:          model.ProviderCodex,
		ProviderThreadID:  "T-legacy-redact",
		Title:             "Leak sk-proj-legacySECRET12345678901234567890",
		Visibility:        model.ThreadVisibilityPrivate,
		UploadFingerprint: legacyUploadFingerprintForTest(model.ProviderCodex, "T-legacy-redact", payload),
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("save legacy thread: %v", err)
	}
	legacyMessage := &model.ThreadMessage{ThreadID: stored.ID, Position: 0, ProviderMessageID: "m-1", Role: "user"}
	legacyMessage.SetRawJSON(`{"role":"user","message_id":"m-1","content":"hide sk-proj-legacySECRET12345678901234567890"}`)
	if err := model.ThreadMessageDB.Save(legacyMessage); err != nil {
		t.Fatalf("save legacy message: %v", err)
	}

	updated, err := thread.UploadThreadSnapshot(user, payload)
	if err != nil {
		t.Fatalf("upload codex snapshot: %v", err)
	}
	if strings.Contains(updated.Title, "sk-proj-legacySECRET") {
		t.Fatalf("expected legacy no-op upload to be reprocessed, got title %q", updated.Title)
	}
	messages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d", len(messages))
	}
	raw := mustRawJSON(t, messages[0])
	if strings.Contains(raw, "sk-proj-legacySECRET") || !strings.Contains(raw, "[REDACTED:openai-api-key]") {
		t.Fatalf("expected legacy raw message redacted, got %s", raw)
	}
}

func TestUploadThreadSnapshotShouldCreateThreadForUser(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	payload := thread.UploadPayload{
		Provider:  model.ProviderCodex,
		Thread:    json.RawMessage(`{"id":"T-service-one","title":"Fix auth flow","messages":[{"role":"user","message_id":"m-1","protocol_message_id":"p-1","content":"please fix oauth","state":{"kind":"complete"},"usage":{"inputTokens":12}}],"v":42}`),
		Repo:      "owner/repo",
		Branch:    "main",
		SourceURL: "https://ampcode.com/threads/T-service-one",
		Labels:    []string{"auth", "bug"},
	}

	stored, err := thread.UploadThreadSnapshot(user, payload)
	if err != nil {
		t.Fatalf("upload codex snapshot: %v", err)
	}
	if stored.ID == 0 {
		t.Fatalf("expected saved thread ID")
	}
	if stored.UserID != user.ID || stored.Provider != model.ProviderCodex || stored.ProviderThreadID != "T-service-one" {
		t.Fatalf("unexpected identity fields %#v", stored)
	}
	if stored.Title != "Fix auth flow" || stored.Visibility != model.ThreadVisibilityPrivate || stored.Repo != "owner/repo" || stored.Branch != "main" || stored.SourceURL != "https://ampcode.com/threads/T-service-one" {
		t.Fatalf("unexpected stored thread %#v", stored)
	}
	if stored.MessageCount != 1 {
		t.Fatalf("expected message count 1, got %d", stored.MessageCount)
	}
	if !strings.Contains(stored.SearchText, "Fix auth flow") || !strings.Contains(stored.SearchText, "please fix oauth") || !strings.Contains(stored.SearchText, "owner/repo") || !strings.Contains(stored.SearchText, "auth") {
		t.Fatalf("expected search text to include upload metadata, got %q", stored.SearchText)
	}
	messages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Position != 0 || messages[0].ProviderMessageID != "m-1" || messages[0].ProtocolMessageID != "p-1" || !strings.Contains(mustRawJSON(t, messages[0]), "please fix oauth") || !strings.Contains(mustRawJSON(t, messages[0]), "inputTokens") {
		t.Fatalf("expected stored message row preserving raw content, got %#v", messages)
	}
	labels, err := model.ThreadLabelDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch labels: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected two labels, got %d", len(labels))
	}
}

func TestUploadThreadSnapshotShouldPreserveChineseTitle(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-chinese-title","title":"部署后中文标题不要乱码","messages":[{"role":"user","content":"这里也不要乱码"}]}`),
	})
	if err != nil {
		t.Fatalf("upload codex snapshot: %v", err)
	}
	if stored.Title != "部署后中文标题不要乱码" {
		t.Fatalf("expected Chinese title to be preserved, got %q", stored.Title)
	}
	if strings.Contains(stored.Title, "\uFFFD") {
		t.Fatalf("expected title without replacement rune, got %q", stored.Title)
	}
}

func TestUploadThreadSnapshotShouldCreateClaudeCodeThreadForUser(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider:  model.ProviderClaudeCode,
		Thread:    json.RawMessage(`{"id":"cc-session-1","title":"Fix OAuth callback","model":"gpt-5.5","reasoning_effort":"xhigh","cwd":"/Users/alice/work/jieli","created_ms":1730000000000,"updated_ms":1730000300000,"messages":[{"role":"user","message_id":"u-1","content":"please fix oauth callback"},{"role":"assistant","message_id":"a-1","content":[{"type":"text","text":"patched"}],"usage":{"totalInputTokens":25000,"maxInputTokens":100000}}]}`),
		Repo:      "burugo/jieli",
		Branch:    "feature/oauth",
		SourceURL: "https://jieli.test/threads/T-cc-session-1",
		Labels:    []string{"claude-code"},
	})
	if err != nil {
		t.Fatalf("upload claude code snapshot: %v", err)
	}

	if stored.Provider != model.ProviderClaudeCode || stored.ProviderThreadID != "T-cc-session-1" || stored.UserID != user.ID {
		t.Fatalf("unexpected identity fields %#v", stored)
	}
	if stored.Title != "Fix OAuth callback" || stored.Repo != "burugo/jieli" || stored.Branch != "feature/oauth" || stored.MessageCount != 2 {
		t.Fatalf("unexpected stored thread %#v", stored)
	}
	if stored.AgentMode != "" || stored.ReasoningEffort != "xhigh" || stored.ProviderCreatedMillis != 1730000000000 || stored.UserLastInteractedAtMS != 1730000300000 {
		t.Fatalf("expected Claude metadata to be stored, got %#v", stored)
	}
	detail, err := thread.GetThreadDetail(user, stored.ID)
	if err != nil {
		t.Fatalf("get claude detail: %v", err)
	}
	if detail.Mode != "" || detail.Model != "gpt-5.5" || detail.ReasoningEffort != "xhigh" {
		t.Fatalf("expected Claude model separate from mode/reasoning, got mode=%q model=%q reasoning=%q", detail.Mode, detail.Model, detail.ReasoningEffort)
	}
	if detail.ContextPercent == nil || *detail.ContextPercent != 25 || detail.ContextWindow == nil || *detail.ContextWindow != 100000 {
		t.Fatalf("expected Claude usage to produce context stats, got percent=%v window=%v", detail.ContextPercent, detail.ContextWindow)
	}
	if !strings.Contains(stored.SearchText, "please fix oauth callback") || !strings.Contains(stored.SearchText, "patched") || !strings.Contains(stored.SearchText, "claude-code") {
		t.Fatalf("expected search text to include Claude messages and labels, got %q", stored.SearchText)
	}
	messages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Order("position ASC").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected two messages, got %#v", messages)
	}
	if messages[0].Role != "user" || messages[0].ProviderMessageID != "u-1" || !strings.Contains(mustRawJSON(t, messages[0]), "please fix oauth callback") {
		t.Fatalf("unexpected user message %#v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].ProviderMessageID != "a-1" || !strings.Contains(mustRawJSON(t, messages[1]), "patched") {
		t.Fatalf("unexpected assistant message %#v", messages[1])
	}
}

func TestUploadThreadSnapshotShouldCreateCodexThreadForUser(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	payload := thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread: json.RawMessage(`{
			"id":"codex-session-1",
			"title":"Fix Codex upload",
			"model":"gpt-5.5",
			"cwd":"/Users/alice/work/jieli",
			"created_ms":1730000000000,
			"updated_ms":1730000300000,
			"messages":[
				{"role":"user","message_id":"u-1","content":"please fix codex upload"},
				{"role":"assistant","message_id":"tool-1","content":[{"type":"tool_use","name":"apply_patch","input":{"patch_text":"*** Begin Patch\n*** Add File: docs/test.md\n+alpha\n+beta\n*** End Patch\n"}}]},
				{"role":"tool","message_id":"tool-1","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"ok"}]}
			]
		}`),
		RepoURL:   "git@home.pika12.com:guoyb/jieli.git",
		Branch:    "feature/codex",
		SourceURL: "https://jieli.test/threads/T-codex-session-1",
		Labels:    []string{"codex"},
	}
	stored, err := thread.UploadThreadSnapshot(user, payload)
	if err != nil {
		t.Fatalf("upload codex snapshot: %v", err)
	}

	if stored.Provider != model.ProviderCodex || stored.ProviderThreadID != "T-codex-session-1" || stored.UserID != user.ID {
		t.Fatalf("unexpected identity fields %#v", stored)
	}
	if stored.Title != "Fix Codex upload" || stored.Repo != "guoyb/jieli" || stored.RepoURL != "https://home.pika12.com/guoyb/jieli" || stored.Branch != "feature/codex" || stored.MessageCount != 3 {
		t.Fatalf("unexpected stored thread %#v", stored)
	}
	if stored.ProviderCreatedMillis != 1730000000000 || stored.UserLastInteractedAtMS != 1730000300000 {
		t.Fatalf("expected Codex timestamps to be stored, got %#v", stored)
	}
	if stored.LinesAdded != 2 || stored.LinesRemoved != 0 || stored.FilesChanged != 1 {
		t.Fatalf("expected Codex change stats +2 -0 1 file, got +%d -%d files=%d summary=%s", stored.LinesAdded, stored.LinesRemoved, stored.FilesChanged, stored.SummaryStatsJSON)
	}
	detail, err := thread.GetThreadDetail(user, stored.ID)
	if err != nil {
		t.Fatalf("get codex detail: %v", err)
	}
	if detail.Model != "gpt-5.5" {
		t.Fatalf("expected Codex model in detail, got %q", detail.Model)
	}
	if !strings.Contains(stored.SearchText, "please fix codex upload") || !strings.Contains(stored.SearchText, "codex") {
		t.Fatalf("expected search text to include Codex messages and labels, got %q", stored.SearchText)
	}
	messages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Order("position ASC").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected three messages, got %#v", messages)
	}
	if messages[0].Role != "user" || messages[0].ProviderMessageID != "u-1" || !strings.Contains(mustRawJSON(t, messages[0]), "please fix codex upload") {
		t.Fatalf("unexpected user message %#v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].ProviderMessageID != "tool-1" || !strings.Contains(mustRawJSON(t, messages[1]), "apply_patch") {
		t.Fatalf("unexpected assistant message %#v", messages[1])
	}
	if messages[2].Role != "tool" || messages[2].ProviderMessageID != "tool-1" || !strings.Contains(mustRawJSON(t, messages[2]), "tool_result") {
		t.Fatalf("unexpected tool message %#v", messages[2])
	}
}

func TestUploadThreadSnapshotShouldUseCodexSummaryStats(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread: json.RawMessage(`{
			"id":"codex-summary-stats",
			"title":"Codex summary stats",
			"summaryStats":{
				"diffStats":{"added":77,"deleted":0,"changed":0},
				"filesChanged":3,
				"messageCount":2,
				"source":"git_tree_diff",
				"complete":true
			},
			"messages":[
				{"role":"user","message_id":"u-1","content":"please change files"},
				{"role":"assistant","message_id":"a-1","content":[{"type":"tool_use","name":"apply_patch","input":{"patch_text":"*** Begin Patch\n*** Add File: docs/inferred.md\n+one\n*** End Patch\n"}}]}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("upload codex snapshot: %v", err)
	}
	if stored.LinesAdded != 77 || stored.LinesRemoved != 0 || stored.FilesChanged != 3 {
		t.Fatalf("expected Codex summary stats +77 -0 3 files, got +%d -%d files=%d summary=%s", stored.LinesAdded, stored.LinesRemoved, stored.FilesChanged, stored.SummaryStatsJSON)
	}
	if !strings.Contains(stored.SummaryStatsJSON, `"source":"git_tree_diff"`) || !strings.Contains(stored.SummaryStatsJSON, `"complete":true`) {
		t.Fatalf("expected raw summary metadata to be preserved, got %s", stored.SummaryStatsJSON)
	}
	detail, err := thread.GetThreadDetail(user, stored.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if detail.SummaryStats["source"] != "git_tree_diff" || detail.SummaryStats["complete"] != true {
		t.Fatalf("expected detail summary metadata, got %#v", detail.SummaryStats)
	}
}

func TestUploadThreadSnapshotShouldUseClaudeCodeSummaryStats(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderClaudeCode,
		Thread: json.RawMessage(`{
			"id":"claude-summary-stats",
			"title":"Claude summary stats",
			"summaryStats":{
				"diffStats":{"added":77,"deleted":0,"changed":0},
				"filesChanged":3,
				"messageCount":2,
				"source":"git_tree_diff",
				"complete":true
			},
			"messages":[
				{"role":"user","message_id":"u-1","content":"please change files"},
				{"role":"assistant","message_id":"a-1","content":[{"type":"tool_use","name":"Write","input":{"file_path":"docs/inferred.md","content":"one\n"}}]}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("upload claude code snapshot: %v", err)
	}
	if stored.LinesAdded != 77 || stored.LinesRemoved != 0 || stored.FilesChanged != 3 {
		t.Fatalf("expected Claude Code summary stats +77 -0 3 files, got +%d -%d files=%d summary=%s", stored.LinesAdded, stored.LinesRemoved, stored.FilesChanged, stored.SummaryStatsJSON)
	}
	if !strings.Contains(stored.SummaryStatsJSON, `"source":"git_tree_diff"`) || !strings.Contains(stored.SummaryStatsJSON, `"complete":true`) {
		t.Fatalf("expected raw summary metadata to be preserved, got %s", stored.SummaryStatsJSON)
	}
}

func TestUploadThreadSnapshotShouldNotDeriveClaudeCodeRepoFromCWD(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderClaudeCode,
		Thread:   json.RawMessage(`{"id":"cc-no-repo","title":"Local folder","cwd":"/Users/alice/work/github/plugins","messages":[{"role":"user","message_id":"u-1","content":"sync local folder"}]}`),
	})
	if err != nil {
		t.Fatalf("upload claude code snapshot: %v", err)
	}

	if stored.Repo != "" {
		t.Fatalf("expected empty repo when Claude Code payload does not provide one, got %q", stored.Repo)
	}
}

func TestUploadThreadSnapshotShouldDeriveClaudeCodeChangeStatsFromWriteAndEdit(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	payload := thread.UploadPayload{
		Provider: model.ProviderClaudeCode,
		Thread: json.RawMessage(`{
			"id":"576ce500-9dba-4e6b-b384-26799622421d",
			"title":"Claude file changes",
			"messages":[
				{"role":"assistant","message_id":"write-1","content":[{"type":"tool_use","name":"Write","input":{"file_path":"/repo/docs/test.md","content":"alpha\nbeta\n"}}]},
				{"role":"assistant","message_id":"edit-1","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/repo/docs/test.md","old_string":"alpha\nbeta\n","new_string":"alpha\nbeta changed\ngamma\n","replace_all":false}}]}
			]
		}`),
	}
	stored, err := thread.UploadThreadSnapshot(user, payload)
	if err != nil {
		t.Fatalf("upload claude code snapshot: %v", err)
	}
	if stored.ProviderThreadID != "T-576ce500-9dba-4e6b-b384-26799622421d" {
		t.Fatalf("expected Claude Code thread id to be stored with T- prefix, got %q", stored.ProviderThreadID)
	}
	if stored.LinesAdded != 3 || stored.LinesRemoved != 0 || stored.FilesChanged != 1 {
		t.Fatalf("expected stored stats +3 -0 1 file, got +%d -%d files=%d summary=%s", stored.LinesAdded, stored.LinesRemoved, stored.FilesChanged, stored.SummaryStatsJSON)
	}
	stored.LinesAdded = 0
	stored.LinesRemoved = 0
	stored.FilesChanged = 0
	stored.SummaryStatsJSON = `{"diffStats":{"added":0,"changed":0,"deleted":0},"filesChanged":0,"messageCount":2}`
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("clear stored stats: %v", err)
	}
	repaired, err := thread.UploadThreadSnapshot(user, payload)
	if err != nil {
		t.Fatalf("reupload matching claude code snapshot: %v", err)
	}
	if repaired.LinesAdded != 3 || repaired.LinesRemoved != 0 || repaired.FilesChanged != 1 || !strings.Contains(repaired.SummaryStatsJSON, `"changed":1`) {
		t.Fatalf("expected idempotent reupload to repair stats, got +%d -%d files=%d summary=%s", repaired.LinesAdded, repaired.LinesRemoved, repaired.FilesChanged, repaired.SummaryStatsJSON)
	}
	detail, err := thread.GetThreadDetail(user, stored.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if detail.LinesAdded != 3 || detail.LinesRemoved != 0 || detail.LinesChanged != 1 || detail.FilesChanged != 1 {
		t.Fatalf("expected detail stats +3 -0 ~1 1 file, got +%d -%d ~%d files=%d", detail.LinesAdded, detail.LinesRemoved, detail.LinesChanged, detail.FilesChanged)
	}
}

func TestUploadThreadSnapshotShouldRejectWhenMessageQuotaExceededAndCleanupDisabled(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix", PlanLevel: model.PlanLevelFree, AutoCleanupEnabled: false}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	existing := &model.Thread{
		UserID:           user.ID,
		Provider:         model.ProviderCodex,
		ProviderThreadID: "T-existing",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     model.FreeMessageQuota - 1,
	}
	if err := model.ThreadDB.Save(existing); err != nil {
		t.Fatalf("seed existing usage: %v", err)
	}

	_, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-over-quota","title":"Over quota","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`),
	})
	if err == nil {
		t.Fatalf("expected upload to fail when projected message usage exceeds quota")
	}
	total, err := model.SumThreadMessageCountByUser(user.ID)
	if err != nil {
		t.Fatalf("sum current messages: %v", err)
	}
	if total != model.FreeMessageQuota-1 {
		t.Fatalf("expected rejected upload not to increase current messages, got %d", total)
	}
	storedUser, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatalf("load user after rejected upload: %v", err)
	}
	if strings.TrimSpace(storedUser.UsageInfoJSON) == "" {
		t.Fatalf("expected rejected upload to record usage_info")
	}
	var usageInfo struct {
		LastUploadError struct {
			Code                   string `json:"code"`
			Provider               string `json:"provider"`
			ProviderThreadID       string `json:"provider_thread_id"`
			CurrentMessages        int    `json:"current_messages"`
			ExistingThreadMessages int    `json:"existing_thread_messages"`
			IncomingMessages       int    `json:"incoming_messages"`
			ProjectedMessages      int    `json:"projected_messages"`
			Quota                  int    `json:"quota"`
		} `json:"last_upload_error"`
	}
	if err := json.Unmarshal([]byte(storedUser.UsageInfoJSON), &usageInfo); err != nil {
		t.Fatalf("decode usage_info: %v", err)
	}
	if usageInfo.LastUploadError.Code != "message_quota_exceeded" ||
		usageInfo.LastUploadError.Provider != model.ProviderCodex ||
		usageInfo.LastUploadError.ProviderThreadID != "T-over-quota" ||
		usageInfo.LastUploadError.CurrentMessages != model.FreeMessageQuota-1 ||
		usageInfo.LastUploadError.ExistingThreadMessages != 0 ||
		usageInfo.LastUploadError.IncomingMessages != 2 ||
		usageInfo.LastUploadError.ProjectedMessages != model.FreeMessageQuota+1 ||
		usageInfo.LastUploadError.Quota != model.FreeMessageQuota {
		t.Fatalf("expected quota error details in usage_info, got %#v", usageInfo.LastUploadError)
	}
}

func TestUploadThreadSnapshotShouldUseDeltaForQuotaAndClearUploadErrorOnSuccess(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{
		Username:           "alice",
		GitHubID:           "github-alice",
		APIKeyHash:         "hash",
		APIKeyPrefix:       "prefix",
		PlanLevel:          model.PlanLevelFree,
		AutoCleanupEnabled: false,
	}
	if err := user.SetUsageInfo(model.UserUsageInfo{LastUploadError: &model.UserUsageUploadError{Code: "message_quota_exceeded"}}); err != nil {
		t.Fatalf("set usage info: %v", err)
	}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	otherUsage := &model.Thread{
		UserID:           user.ID,
		Provider:         model.ProviderCodex,
		ProviderThreadID: "T-other-usage",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     model.FreeMessageQuota - 30,
	}
	if err := model.ThreadDB.Save(otherUsage); err != nil {
		t.Fatalf("seed other usage: %v", err)
	}
	existing := &model.Thread{
		UserID:           user.ID,
		Provider:         model.ProviderCodex,
		ProviderThreadID: "T-existing-delta",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     20,
	}
	if err := model.ThreadDB.Save(existing); err != nil {
		t.Fatalf("seed existing thread: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   jsonPayload("T-existing-delta", 25),
	})
	if err != nil {
		t.Fatalf("expected delta-based upload to fit quota: %v", err)
	}
	if stored.ID != existing.ID || stored.MessageCount != 25 {
		t.Fatalf("expected existing thread updated to 25 messages, got id=%d count=%d", stored.ID, stored.MessageCount)
	}
	latestUser, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatalf("load user after successful upload: %v", err)
	}
	if latestUser.UsageInfo().LastUploadError != nil {
		t.Fatalf("expected successful upload to clear last upload error, got %s", latestUser.UsageInfoJSON)
	}
}

func TestUploadThreadSnapshotShouldEnforceMessageQuotaForPluginProviders(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix", PlanLevel: model.PlanLevelFree, AutoCleanupEnabled: false}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	existingUsage := &model.Thread{
		UserID:           user.ID,
		Provider:         model.ProviderAmp,
		ProviderThreadID: "T-existing-usage",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     model.FreeMessageQuota - 1,
	}
	if err := model.ThreadDB.Save(existingUsage); err != nil {
		t.Fatalf("seed existing usage: %v", err)
	}

	_, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"codex-over-quota","title":"Codex over quota","messages":[{"role":"user","message_id":"u-1","content":"one"},{"role":"assistant","message_id":"a-1","content":"two"}]}`),
	})
	if err == nil {
		t.Fatalf("expected Codex upload to fail when projected message usage exceeds quota")
	}
	latestUser, err := model.UserDB.ByID(user.ID)
	if err != nil {
		t.Fatalf("load user after rejected Codex upload: %v", err)
	}
	lastError := latestUser.UsageInfo().LastUploadError
	if lastError == nil || lastError.Provider != model.ProviderCodex || lastError.ProviderThreadID != "T-codex-over-quota" {
		t.Fatalf("expected Codex quota error details in usage_info, got %#v", lastError)
	}
}

func TestUploadThreadSnapshotShouldUpdateExistingThreadAndReplaceLabels(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	first, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider:  model.ProviderCodex,
		Thread:    json.RawMessage(`{"id":"T-service-one","title":"Old title","messages":[{"role":"user","content":"old content"}]}`),
		Repo:      "owner/repo",
		Branch:    "main",
		SourceURL: "https://ampcode.com/threads/T-service-one",
		Labels:    []string{"old", "bug"},
	})
	if err != nil {
		t.Fatalf("initial upload: %v", err)
	}

	updated, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider:  model.ProviderCodex,
		Thread:    json.RawMessage(`{"id":"T-service-one","title":"New title","messages":[{"role":"user","content":"new content"},{"role":"assistant","content":"fixed"}]}`),
		Repo:      "owner/repo",
		Branch:    "feature/oauth",
		SourceURL: "https://ampcode.com/threads/T-service-one",
		Labels:    []string{"feature"},
	})
	if err != nil {
		t.Fatalf("update upload: %v", err)
	}
	if updated.ID != first.ID {
		t.Fatalf("expected upload to update existing thread %d, got %d", first.ID, updated.ID)
	}
	if updated.Title != "New title" || updated.Branch != "feature/oauth" || updated.MessageCount != 2 || !strings.Contains(updated.SearchText, "new content") {
		t.Fatalf("expected updated snapshot data, got %#v", updated)
	}
	threads, err := model.ThreadDB.Where("user_id = ? AND provider = ? AND provider_thread_id = ?", user.ID, model.ProviderCodex, "T-service-one").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch threads: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("expected one upserted thread, got %d", len(threads))
	}
	messages, err := model.ThreadMessageDB.Where("thread_id = ?", updated.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}
	if len(messages) != 2 || messages[0].Position != 0 || !strings.Contains(mustRawJSON(t, messages[0]), "new content") || messages[1].Position != 1 || !strings.Contains(mustRawJSON(t, messages[1]), "fixed") {
		t.Fatalf("expected replacement messages in upload order, got %#v", messages)
	}
	firstMessageID := messages[0].ID
	for _, message := range messages {
		if strings.Contains(mustRawJSON(t, message), "old content") {
			t.Fatalf("expected old message content to be replaced, got %#v", messages)
		}
	}

	rewound, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider:  model.ProviderCodex,
		Thread:    json.RawMessage(`{"id":"T-service-one","title":"Rewound title","messages":[{"role":"user","content":"rewound content"}]}`),
		Repo:      "owner/repo",
		Branch:    "feature/oauth",
		SourceURL: "https://ampcode.com/threads/T-service-one",
		Labels:    []string{"feature"},
	})
	if err != nil {
		t.Fatalf("rewind upload: %v", err)
	}
	if rewound.ID != first.ID {
		t.Fatalf("expected rewind upload to preserve thread id %d, got %d", first.ID, rewound.ID)
	}
	messages, err = model.ThreadMessageDB.Where("thread_id = ?", updated.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch rewound messages: %v", err)
	}
	if len(messages) != 1 || messages[0].ID != firstMessageID || messages[0].Position != 0 || !strings.Contains(mustRawJSON(t, messages[0]), "rewound content") {
		t.Fatalf("expected rewind to update first message in place and delete tail, got %#v", messages)
	}
	labels, err := model.ThreadLabelDB.Where("thread_id = ?", updated.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch labels: %v", err)
	}
	if len(labels) != 1 || labels[0].Label != "feature" {
		t.Fatalf("expected replacement labels [feature], got %#v", labels)
	}
}

func TestUploadThreadSnapshotShouldAssociateReferencedAttachments(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	attachment := &model.Attachment{
		UserID:        user.ID,
		PublicID:      "att-cdn",
		MediaType:     "image/png",
		Extension:     "png",
		StorageDriver: "r2",
		StorageKey:    "att-cdn.png",
		SizeBytes:     12,
	}
	if err := model.AttachmentDB.Save(attachment); err != nil {
		t.Fatalf("save attachment: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-attachment-assoc","title":"Attachment assoc","messages":[{"role":"user","content":[{"type":"image","source":{"url":"https://cdn.jieli.app/att-cdn.png","type":"url"}}]}]}`),
	})
	if err != nil {
		t.Fatalf("upload thread: %v", err)
	}

	var count int
	err = model.ThreadDB.DB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM thread_attachments WHERE thread_id = ? AND attachment_id = ?", stored.ID, attachment.ID).Scan(&count)
	if err != nil {
		t.Fatalf("count thread attachments: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one thread attachment association, got %d", count)
	}
}

func TestUploadThreadSnapshotShouldAssociateLegacyAttachmentURLs(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	attachment := &model.Attachment{
		UserID:        user.ID,
		PublicID:      "att-legacy",
		MediaType:     "image/png",
		Extension:     "png",
		StorageDriver: "local",
		StorageKey:    "att-legacy.png",
		SizeBytes:     12,
	}
	if err := model.AttachmentDB.Save(attachment); err != nil {
		t.Fatalf("save attachment: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-attachment-legacy","title":"Attachment legacy","messages":[{"role":"user","content":[{"type":"image","source":{"url":"/attachments/att-legacy.png","type":"url"}}]}]}`),
	})
	if err != nil {
		t.Fatalf("upload thread: %v", err)
	}

	links, err := model.ThreadAttachmentDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch thread attachments: %v", err)
	}
	if len(links) != 1 || links[0].AttachmentID != attachment.ID {
		t.Fatalf("expected legacy URL attachment association, got %#v", links)
	}
}

func TestUploadThreadSnapshotShouldOnlyAssociateNewMessages(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	payload := thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-attachment-incremental","title":"Attachment incremental","messages":[{"role":"user","content":[{"type":"image","source":{"url":"https://cdn.jieli.app/att-old.png","type":"url"}}]}]}`),
	}
	stored, err := thread.UploadThreadSnapshot(user, payload)
	if err != nil {
		t.Fatalf("upload initial thread: %v", err)
	}
	attachment := &model.Attachment{
		UserID:        user.ID,
		PublicID:      "att-old",
		MediaType:     "image/png",
		Extension:     "png",
		StorageDriver: "r2",
		StorageKey:    "att-old.png",
		SizeBytes:     12,
	}
	if err := model.AttachmentDB.Save(attachment); err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	payload.Thread = json.RawMessage(`{"id":"T-attachment-incremental","title":"Attachment incremental","messages":[{"role":"user","content":[{"type":"image","source":{"url":"https://cdn.jieli.app/att-old.png","type":"url"}}]},{"role":"assistant","content":"new message without attachment"}]}`)
	if _, err := thread.UploadThreadSnapshot(user, payload); err != nil {
		t.Fatalf("upload appended thread: %v", err)
	}

	links, err := model.ThreadAttachmentDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch thread attachments: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected old message attachment URL to be skipped on appended upload, got %#v", links)
	}
}

func TestUploadThreadSnapshotShouldRestoreSoftDeletedReferencedAttachments(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	attachment := &model.Attachment{
		UserID:        user.ID,
		PublicID:      "att-restore",
		MediaType:     "image/png",
		Extension:     "png",
		StorageDriver: "r2",
		StorageKey:    "att-restore.png",
		SizeBytes:     12,
	}
	if err := model.AttachmentDB.Save(attachment); err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	if err := model.AttachmentDB.SoftDelete(attachment); err != nil {
		t.Fatalf("soft delete attachment: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-attachment-restore","title":"Attachment restore","messages":[{"role":"user","content":[{"type":"image","source":{"url":"https://cdn.jieli.app/att-restore.png","type":"url"}}]}]}`),
	})
	if err != nil {
		t.Fatalf("upload thread: %v", err)
	}

	restored, err := model.AttachmentDB.ByID(attachment.ID)
	if err != nil {
		t.Fatalf("expected attachment to be restored: %v", err)
	}
	if restored.Deleted {
		t.Fatalf("expected attachment deleted flag to be false")
	}
	links, err := model.ThreadAttachmentDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch thread attachments: %v", err)
	}
	if len(links) != 1 || links[0].AttachmentID != attachment.ID {
		t.Fatalf("expected restored attachment association, got %#v", links)
	}
}

func TestUploadThreadSnapshotShouldUpdateDailyActivity(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	_, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-service-one","title":"Today","summaryStats":{"diffStats":{"added":116533,"deleted":14001,"changed":8335}},"messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`),
	})
	if err != nil {
		t.Fatalf("upload codex snapshot: %v", err)
	}
	activities, err := model.DailyActivityDB.Where("user_id = ? AND date = ?", user.ID, time.Now().Format("2006-01-02")).Fetch(0, 1)
	if err != nil {
		t.Fatalf("fetch daily activity: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected one daily activity row, got %d", len(activities))
	}
	if activities[0].ThreadCount != 1 || activities[0].MessageCount != 2 {
		t.Fatalf("expected daily activity thread_count=1 message_count=2, got %#v", activities[0])
	}
	if activities[0].LinesAdded != 116533 || activities[0].LinesRemoved != 14001 || activities[0].LinesChanged != 8335 {
		t.Fatalf("expected daily activity diff stats added=116533 removed=14001 changed=8335, got %#v", activities[0])
	}
}

func TestUploadThreadSnapshotShouldUpdateDailyActivityWithPositiveDeltas(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	upload := func(added int, changed int, messages string) {
		t.Helper()
		_, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
			Provider: model.ProviderCodex,
			Thread: json.RawMessage(`{"id":"T-daily-delta","title":"Daily delta","summaryStats":{"diffStats":{"added":` +
				strconv.Itoa(added) + `,"deleted":0,"changed":` + strconv.Itoa(changed) +
				`},"filesChanged":2},"messages":` + messages + `}`),
		})
		if err != nil {
			t.Fatalf("upload codex snapshot added=%d changed=%d: %v", added, changed, err)
		}
	}
	activity := func() *model.DailyActivity {
		t.Helper()
		activities, err := model.DailyActivityDB.Where("user_id = ? AND date = ?", user.ID, time.Now().Format("2006-01-02")).Fetch(0, 1)
		if err != nil {
			t.Fatalf("fetch daily activity: %v", err)
		}
		if len(activities) != 1 {
			t.Fatalf("expected one daily activity row, got %d", len(activities))
		}
		return activities[0]
	}

	upload(10, 4, `[{"role":"user","content":"one"}]`)
	first := activity()
	if first.ThreadCount != 1 || first.MessageCount != 1 || first.LinesAdded != 10 || first.LinesChanged != 4 || first.FilesChanged != 2 {
		t.Fatalf("expected initial daily activity, got %#v", first)
	}

	upload(15, 6, `[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]`)
	increased := activity()
	if increased.ThreadCount != 1 || increased.MessageCount != 2 || increased.LinesAdded != 15 || increased.LinesChanged != 6 || increased.FilesChanged != 2 {
		t.Fatalf("expected positive deltas only after increase, got %#v", increased)
	}

	upload(15, 6, `[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]`)
	repeated := activity()
	if repeated.ThreadCount != 1 || repeated.MessageCount != 2 || repeated.LinesAdded != 15 || repeated.LinesChanged != 6 || repeated.FilesChanged != 2 {
		t.Fatalf("expected repeated upload not to inflate daily activity, got %#v", repeated)
	}

	upload(8, 3, `[{"role":"user","content":"one"}]`)
	decreased := activity()
	if decreased.ThreadCount != 1 || decreased.MessageCount != 2 || decreased.LinesAdded != 15 || decreased.LinesChanged != 6 || decreased.FilesChanged != 2 {
		t.Fatalf("expected decreased stats not to subtract daily activity, got %#v", decreased)
	}
	stored, err := thread.GetThreadDetailByProviderID(user, "T-daily-delta")
	if err != nil {
		t.Fatalf("get thread detail: %v", err)
	}
	if stored.LinesAdded != 8 || stored.LinesChanged != 3 {
		t.Fatalf("expected latest thread stats to decrease while activity stays cumulative, got +%d ~%d", stored.LinesAdded, stored.LinesChanged)
	}
}

func TestUploadThreadSnapshotShouldIgnoreIdenticalSnapshotWithoutRewritingLabels(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	payload := thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-duplicate","title":"Duplicate","messages":[{"role":"user","content":"same content"}]}`),
		Repo:     "owner/repo",
		Labels:   []string{"sync"},
	}

	first, err := thread.UploadThreadSnapshot(user, payload)
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}
	labels, err := model.ThreadLabelDB.Where("thread_id = ?", first.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch labels after first upload: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("expected one label after first upload, got %d", len(labels))
	}
	firstLabelID := labels[0].ID

	second, err := thread.UploadThreadSnapshot(user, payload)
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected duplicate upload to return existing thread %d, got %d", first.ID, second.ID)
	}
	labels, err = model.ThreadLabelDB.Where("thread_id = ?", first.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch labels after duplicate upload: %v", err)
	}
	if len(labels) != 1 || labels[0].ID != firstLabelID || labels[0].Label != "sync" {
		t.Fatalf("expected duplicate upload to preserve existing label row, got %#v", labels)
	}
}

func TestUploadThreadSnapshotShouldOnlyWriteChangedMessagesAndLabels(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	firstPayload := thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-append","title":"Append","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`),
		Labels:   []string{"sync", "keep"},
	}
	stored, err := thread.UploadThreadSnapshot(user, firstPayload)
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}
	firstMessages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Order("position ASC").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch first messages: %v", err)
	}
	if len(firstMessages) != 2 {
		t.Fatalf("expected two initial messages, got %#v", firstMessages)
	}
	firstLabels, err := model.ThreadLabelDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch first labels: %v", err)
	}
	labelIDs := map[string]int64{}
	for _, label := range firstLabels {
		labelIDs[label.Label] = label.ID
	}

	secondPayload := thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-append","title":"Append updated","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"user","content":"three"}]}`),
		Labels:   []string{"sync", "keep"},
	}
	stored, err = thread.UploadThreadSnapshot(user, secondPayload)
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	secondMessages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Order("position ASC").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch second messages: %v", err)
	}
	if len(secondMessages) != 3 {
		t.Fatalf("expected appended third message, got %#v", secondMessages)
	}
	if secondMessages[0].ID != firstMessages[0].ID || secondMessages[1].ID != firstMessages[1].ID {
		t.Fatalf("expected unchanged existing message rows to be preserved, before=%#v after=%#v", firstMessages, secondMessages)
	}
	secondLabels, err := model.ThreadLabelDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch second labels: %v", err)
	}
	if len(secondLabels) != 2 {
		t.Fatalf("expected two labels after second upload, got %#v", secondLabels)
	}
	for _, label := range secondLabels {
		if labelIDs[label.Label] != label.ID {
			t.Fatalf("expected unchanged label row %q to be preserved, before=%#v after=%#v", label.Label, labelIDs, secondLabels)
		}
	}
}

func TestUploadThreadSnapshotShouldRewriteChangedMessageAndDeleteTrailingRows(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	firstPayload := thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-shrink","title":"Shrink","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"user","content":"three"}]}`),
	}
	stored, err := thread.UploadThreadSnapshot(user, firstPayload)
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}
	firstMessages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Order("position ASC").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch first messages: %v", err)
	}
	if len(firstMessages) != 3 {
		t.Fatalf("expected three initial messages, got %#v", firstMessages)
	}

	// Position 1 content changes in place; position 2 is dropped (tail deletion).
	secondPayload := thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-shrink","title":"Shrink","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two-edited"}]}`),
	}
	stored, err = thread.UploadThreadSnapshot(user, secondPayload)
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	secondMessages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Order("position ASC").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch second messages: %v", err)
	}
	if len(secondMessages) != 2 {
		t.Fatalf("expected two messages after shrink, got %#v", secondMessages)
	}
	if secondMessages[0].ID != firstMessages[0].ID {
		t.Fatalf("expected unchanged head row to be preserved, before=%d after=%d", firstMessages[0].ID, secondMessages[0].ID)
	}
	if secondMessages[1].ID != firstMessages[1].ID {
		t.Fatalf("expected changed row to be updated in place, before=%d after=%d", firstMessages[1].ID, secondMessages[1].ID)
	}
	raw, err := secondMessages[1].RawJSON()
	if err != nil {
		t.Fatalf("decode changed message: %v", err)
	}
	if !strings.Contains(raw, "two-edited") {
		t.Fatalf("expected changed row content to be rewritten, got %q", raw)
	}
	if _, err := model.ThreadMessageDB.ByID(firstMessages[2].ID); err == nil {
		t.Fatalf("expected trailing message row %d to be deleted", firstMessages[2].ID)
	}
}

func TestListThreadsShouldReturnOwnedThreadsWithLabelsAndSearch(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix","messages":[{"role":"user","content":"please fix oauth"}]}`), Repo: "owner/repo", Labels: []string{"auth"}}); err != nil {
		t.Fatalf("upload alice alpha: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-beta","title":"Billing cleanup","messages":[{"role":"user","content":"billing"}]}`), Repo: "owner/repo", Labels: []string{"billing"}}); err != nil {
		t.Fatalf("upload alice beta: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(bob, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"Bob oauth","messages":[{"role":"user","content":"please fix oauth"}]}`), Repo: "other/repo", Labels: []string{"auth"}}); err != nil {
		t.Fatalf("upload bob alpha: %v", err)
	}

	result, err := thread.ListThreads(alice, thread.ListOptions{Search: "oauth", Repo: "owner/repo", Label: "auth", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if result.Total != 1 || len(result.Threads) != 1 {
		t.Fatalf("expected one owned matching thread, got total=%d threads=%#v", result.Total, result.Threads)
	}
	if result.Threads[0].ProviderThreadID != "T-alpha" || result.Threads[0].Preview == "" {
		t.Fatalf("expected alice T-alpha summary with preview, got %#v", result.Threads[0])
	}
	if result.Threads[0].Preview != "please fix oauth" {
		t.Fatalf("expected preview from first message only, got %q", result.Threads[0].Preview)
	}
	if len(result.Threads[0].Labels) != 1 || result.Threads[0].Labels[0] != "auth" {
		t.Fatalf("expected auth label, got %#v", result.Threads[0].Labels)
	}
	result, err = thread.ListThreads(alice, thread.ListOptions{Repo: "owner/repo", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list all owner repo threads: %v", err)
	}
	if result.Total != 2 || len(result.Threads) != 2 || result.Threads[0].ProviderThreadID != "T-beta" || result.Threads[1].ProviderThreadID != "T-alpha" {
		t.Fatalf("expected threads sorted by created time descending, got %#v", result.Threads)
	}

	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix updated","messages":[{"role":"user","content":"please fix oauth again"}]}`), Repo: "owner/repo", Labels: []string{"auth"}}); err != nil {
		t.Fatalf("update alice alpha: %v", err)
	}
	result, err = thread.ListThreads(alice, thread.ListOptions{Repo: "owner/repo", Sort: "updated", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list updated owner repo threads: %v", err)
	}
	if result.Total != 2 || len(result.Threads) != 2 || result.Threads[0].ProviderThreadID != "T-alpha" || result.Threads[1].ProviderThreadID != "T-beta" {
		t.Fatalf("expected threads sorted by updated time descending, got %#v", result.Threads)
	}
}

func TestListThreadsShouldPreviewMessageTextWithoutSearchMetadata(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	if _, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider:  model.ProviderClaudeCode,
		Repo:      "github/jieli",
		Branch:    "feat/backend",
		SourceURL: "http://127.0.0.1:8080/threads/T-576ce500-9dba-4e6b-b384-26799622421d",
		Labels:    []string{"claude-code"},
		Thread: json.RawMessage(`{
			"id":"T-576ce500-9dba-4e6b-b384-26799622421d",
			"title":"hello 哈哈",
			"model":"gpt-5.4-mini-2026-03-17",
			"cwd":"/workspace/github/jieli",
			"messages":[
				{"role":"user","content":"hello 哈哈","message_id":"u-1"},
				{"role":"assistant","content":"收到，我来看看","message_id":"a-1"}
			]
		}`),
	}); err != nil {
		t.Fatalf("upload claude code thread: %v", err)
	}

	result, err := thread.ListThreads(user, thread.ListOptions{Search: "gpt-5.4", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if result.Total != 1 || len(result.Threads) != 1 {
		t.Fatalf("expected one matching thread, got total=%d threads=%#v", result.Total, result.Threads)
	}
	if result.Threads[0].Preview != "hello 哈哈 收到，我来看看" {
		t.Fatalf("expected preview from message text only, got %q", result.Threads[0].Preview)
	}
	storedThreads, err := model.ThreadDB.Where("user_id = ? AND provider_thread_id = ?", user.ID, "T-576ce500-9dba-4e6b-b384-26799622421d").Fetch(0, 1)
	if err != nil {
		t.Fatalf("fetch stored thread: %v", err)
	}
	if len(storedThreads) != 1 || storedThreads[0].Preview != "hello 哈哈 收到，我来看看" {
		t.Fatalf("expected stored preview, got %#v", storedThreads)
	}
	metadata := []string{"github/jieli", "feat/backend", "127.0.0.1", "/workspace/github/jieli", "gpt-5.4", "claude-code"}
	for _, value := range metadata {
		if strings.Contains(result.Threads[0].Preview, value) {
			t.Fatalf("expected preview without metadata %q, got %q", value, result.Threads[0].Preview)
		}
	}
}

func TestListThreadsShouldUseStoredPreviewWithoutReadingMessages(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	stored := &model.Thread{
		UserID:           user.ID,
		Provider:         model.ProviderClaudeCode,
		ProviderThreadID: "T-preview",
		Title:            "Preview fallback title",
		Visibility:       model.ThreadVisibilityPrivate,
		Preview:          "stored preview",
		MessageCount:     1,
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("save thread: %v", err)
	}
	if err := model.ThreadMessageDB.Save(&model.ThreadMessage{
		ThreadID: stored.ID,
		Position: 0,
		Role:     "user",
		RawZstd:  model.CompressRawJSON(`{"role":"user","content":"expensive raw message preview"}`),
	}); err != nil {
		t.Fatalf("save message: %v", err)
	}

	result, err := thread.ListThreads(user, thread.ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if len(result.Threads) != 1 || result.Threads[0].Preview != "stored preview" {
		t.Fatalf("expected stored preview without message-derived preview, got %#v", result.Threads)
	}

	stored.Preview = ""
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("clear preview: %v", err)
	}
	result, err = thread.ListThreads(user, thread.ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list threads after clearing preview: %v", err)
	}
	if len(result.Threads) != 1 || result.Threads[0].Preview != "Preview fallback title" {
		t.Fatalf("expected title fallback without message scan, got %#v", result.Threads)
	}
}

func TestListThreadsShouldPageBeyondThreeHundredThreads(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	for i := 0; i < 305; i++ {
		if err := model.ThreadDB.Save(&model.Thread{
			UserID:           user.ID,
			Provider:         model.ProviderClaudeCode,
			ProviderThreadID: fmt.Sprintf("T-page-%03d", i),
			Title:            fmt.Sprintf("Thread %03d", i),
			Visibility:       model.ThreadVisibilityPrivate,
			Preview:          fmt.Sprintf("Preview %03d", i),
		}); err != nil {
			t.Fatalf("save thread %d: %v", i, err)
		}
	}

	result, err := thread.ListThreads(user, thread.ListOptions{Page: 16, PageSize: 20})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if result.Total != 305 || len(result.Threads) != 5 {
		t.Fatalf("expected total 305 with 5 rows on page 16, got total=%d rows=%d", result.Total, len(result.Threads))
	}
}

func TestUploadThreadSnapshotShouldDeriveChangeStatsFromEditFileWhenSummaryStatsMissing(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread:   json.RawMessage(`{"id":"T-edit","title":"Edit file","messages":[{"role":"assistant","content":[{"type":"tool_use","name":"edit_file","input":{"path":"/repo/a.go","old_str":"one\ntwo\nthree","new_str":"one\ntwo changed\nthree\nfour"}}]}]}`),
	})
	if err != nil {
		t.Fatalf("upload edit snapshot: %v", err)
	}
	if stored.LinesAdded != 1 || stored.LinesRemoved != 0 || stored.FilesChanged != 1 {
		t.Fatalf("expected stored stats +1 -0 1 file, got +%d -%d files=%d summary=%s", stored.LinesAdded, stored.LinesRemoved, stored.FilesChanged, stored.SummaryStatsJSON)
	}
	detail, err := thread.GetThreadDetail(user, stored.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if detail.LinesAdded != 1 || detail.LinesRemoved != 0 || detail.LinesChanged != 1 || detail.FilesChanged != 1 {
		t.Fatalf("expected detail stats +1 -0 ~1 1 file, got +%d -%d ~%d files=%d", detail.LinesAdded, detail.LinesRemoved, detail.LinesChanged, detail.FilesChanged)
	}
}

func TestUploadThreadSnapshotShouldPreserveRawSummaryStatsWhenOnlyFilesChangedIsProvided(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}

	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread: json.RawMessage(`{
			"id":"T-summary-files-only",
			"title":"Summary files only",
			"summaryStats":{
				"diffStats":{"added":0,"deleted":0,"changed":0},
				"filesChanged":9,
				"messageCount":1,
				"source":"provider_metadata",
				"complete":false
			},
			"messages":[
				{"role":"assistant","content":[{"type":"tool_use","name":"edit_file","input":{"path":"/repo/a.go","old_str":"one\ntwo","new_str":"one\ntwo changed\nthree"}}]}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("upload codex snapshot: %v", err)
	}
	if stored.LinesAdded != 1 || stored.LinesRemoved != 0 || stored.FilesChanged != 1 {
		t.Fatalf("expected inferred counters +1 -0 1 file, got +%d -%d files=%d summary=%s", stored.LinesAdded, stored.LinesRemoved, stored.FilesChanged, stored.SummaryStatsJSON)
	}
	if !strings.Contains(stored.SummaryStatsJSON, `"source":"provider_metadata"`) || !strings.Contains(stored.SummaryStatsJSON, `"filesChanged":9`) {
		t.Fatalf("expected raw summary JSON to be preserved, got %s", stored.SummaryStatsJSON)
	}
	detail, err := thread.GetThreadDetail(user, stored.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if detail.LinesAdded != 1 || detail.FilesChanged != 9 || detail.SummaryStats["source"] != "provider_metadata" {
		t.Fatalf("expected detail to expose raw summary metadata with stored line counters, got detail=%#v summary=%#v", detail, detail.SummaryStats)
	}
}

func TestUploadThreadSnapshotShouldDeriveChangeStatsFromCreateFileAndApplyPatch(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	stored, err := thread.UploadThreadSnapshot(user, thread.UploadPayload{
		Provider: model.ProviderCodex,
		Thread: json.RawMessage(`{
			"id":"T-create-patch",
			"title":"Create and patch",
			"messages":[
				{"role":"assistant","content":[
					{"type":"tool_use","name":"create_file","input":{"path":"/repo/index.go","content":"package thing\n\nconst Name = \"jieli\"\n"}},
					{"type":"tool_use","name":"apply_patch","input":{"patchtext":"*** Begin Patch\n*** Update File: /repo/internal/schema/metadata.go\n@@\n type IndexInfo struct {\n-    Unique bool\n+    Unique bool // Is unique index\n+    Where string\n }\n*** End Patch"}}
				]}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("upload create/patch snapshot: %v", err)
	}
	if stored.LinesAdded != 4 || stored.LinesRemoved != 0 || stored.FilesChanged != 2 {
		t.Fatalf("expected stored stats +4 -0 2 files, got +%d -%d files=%d summary=%s", stored.LinesAdded, stored.LinesRemoved, stored.FilesChanged, stored.SummaryStatsJSON)
	}
	detail, err := thread.GetThreadDetail(user, stored.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if detail.LinesAdded != 4 || detail.LinesRemoved != 0 || detail.LinesChanged != 1 || detail.FilesChanged != 2 {
		t.Fatalf("expected detail stats +4 -0 ~1 2 files, got +%d -%d ~%d files=%d", detail.LinesAdded, detail.LinesRemoved, detail.LinesChanged, detail.FilesChanged)
	}
}

func TestGetThreadDetailShouldReturnOwnedSnapshotWithLabels(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice A.", Email: "alice@example.test", AvatarURL: "https://example.test/alice.png", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	// A persisted pre-retirement Amp record, independent of any upload implementation.
	aliceThread := &model.Thread{UserID: alice.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-alpha", Title: "OAuth fix", Visibility: model.ThreadVisibilityPrivate, Repo: "owner/repo", AgentMode: "deep", CreatorUserID: "amp-alice", ReasoningEffort: "high", MessageCount: 2, SearchText: "OAuth fix please fix oauth done", EnvJSON: `{"initial":{"workspaceRoot":"file:///Users/alice/work/repo","workingDirectory":"file:///Users/alice/work/repo/subdir"}}`, SummaryStatsJSON: `{"diffStats":{"added":12,"deleted":3,"changed":4},"messageCount":2}`}
	if err := model.ThreadDB.Save(aliceThread); err != nil {
		t.Fatal(err)
	}
	for position, raw := range []string{`{"role":"user","content":"please fix oauth"}`, `{"role":"assistant","content":"done","usage":{"totalInputTokens":37035,"maxInputTokens":200000}}`} {
		message := &model.ThreadMessage{ThreadID: aliceThread.ID, Position: position}
		message.SetRawJSON(raw)
		if err := model.ThreadMessageDB.Save(message); err != nil {
			t.Fatal(err)
		}
	}
	if err := model.ThreadLabelDB.Save(&model.ThreadLabel{UserID: alice.ID, ThreadID: aliceThread.ID, Label: "auth"}); err != nil {
		t.Fatal(err)
	}
	bobThread, err := thread.UploadThreadSnapshot(bob, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"Bob oauth","messages":[{"role":"user","content":"please fix oauth"}]}`), Repo: "other/repo", Labels: []string{"auth"}})
	if err != nil {
		t.Fatalf("upload bob thread: %v", err)
	}

	detail, err := thread.GetThreadDetail(alice, aliceThread.ID)
	if err != nil {
		t.Fatalf("get thread detail: %v", err)
	}
	if detail.ProviderThreadID != "T-alpha" || detail.Title != "OAuth fix" {
		t.Fatalf("expected alice thread detail, got %#v", detail)
	}
	var messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(detail.Messages, &messages); err != nil {
		t.Fatalf("decode detail messages: %v", err)
	}
	if len(messages) != 2 || messages[0].Content != "please fix oauth" || messages[1].Role != "assistant" || messages[1].Content != "done" {
		t.Fatalf("expected detail messages restored in upload order, got %#v", messages)
	}
	if len(detail.Labels) != 1 || detail.Labels[0] != "auth" {
		t.Fatalf("expected auth label, got %#v", detail.Labels)
	}
	if detail.Owner == nil || detail.Owner.Username != "alice" || detail.Owner.DisplayName != "Alice A." || detail.Owner.AvatarURL != "https://example.test/alice.png" {
		t.Fatalf("expected public owner fields, got %#v", detail.Owner)
	}
	if detail.CreatorUserID != "amp-alice" || detail.ReasoningEffort != "high" || detail.Mode != "deep" {
		t.Fatalf("expected provider metadata, got creator=%q reasoning=%q mode=%q", detail.CreatorUserID, detail.ReasoningEffort, detail.Mode)
	}
	if detail.WorkspaceRoot != "file:///Users/alice/work/repo" || detail.WorkingDirectory != "file:///Users/alice/work/repo/subdir" {
		t.Fatalf("expected workspace metadata, got root=%q working=%q", detail.WorkspaceRoot, detail.WorkingDirectory)
	}
	if detail.SummaryStats == nil || detail.SummaryStats["messageCount"] == nil {
		t.Fatalf("expected structured summary stats, got %#v", detail.SummaryStats)
	}
	if detail.ContextPercent == nil || *detail.ContextPercent != 18.5 || detail.ContextWindow == nil || *detail.ContextWindow != 200000 {
		t.Fatalf("expected context usage from latest message usage, got percent=%v window=%v", detail.ContextPercent, detail.ContextWindow)
	}
	if _, err := thread.GetThreadDetail(alice, bobThread.ID); err == nil {
		t.Fatalf("expected alice not to read bob's thread")
	}
	found, err := thread.ListThreads(alice, thread.ListOptions{Search: "oauth"})
	if err != nil || len(found.Threads) != 1 {
		t.Fatalf("historical search: %+v %v", found, err)
	}
	shared, err := thread.UpdateThreadMeta(alice, aliceThread.ID, thread.MetaUpdate{Visibility: model.ThreadVisibilityUnlisted})
	if err != nil {
		t.Fatal(err)
	}
	public, err := thread.GetSharedThread(shared.ShareSlug)
	if err != nil || !strings.Contains(string(public.Messages), "please fix oauth") {
		t.Fatalf("historical share: %+v %v", public, err)
	}
	if exported := thread.ExportMarkdown(public); !strings.Contains(exported, "please fix oauth") || !strings.Contains(exported, "done") {
		t.Fatalf("historical export: %s", exported)
	}

}

func TestGetThreadDetailShouldReturnAllStoredMessages(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	stored := &model.Thread{
		UserID:           alice.ID,
		Provider:         model.ProviderAmp,
		ProviderThreadID: "T-many-messages",
		Title:            "Many messages",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     1001,
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("save thread: %v", err)
	}
	for i := range 1001 {
		if err := model.ThreadMessageDB.Save(&model.ThreadMessage{
			ThreadID:          stored.ID,
			Position:          i,
			ProviderMessageID: fmt.Sprintf("m-%d", i),
			Role:              "user",
			RawZstd:           model.CompressRawJSON(fmt.Sprintf(`{"role":"user","content":"message-%d"}`, i)),
		}); err != nil {
			t.Fatalf("save message %d: %v", i, err)
		}
	}

	detail, err := thread.GetThreadDetail(alice, stored.ID)
	if err != nil {
		t.Fatalf("get thread detail: %v", err)
	}
	var messages []struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(detail.Messages, &messages); err != nil {
		t.Fatalf("decode detail messages: %v", err)
	}
	if len(messages) != 1001 {
		t.Fatalf("expected all 1001 messages, got %d", len(messages))
	}
	if messages[0].Content != "message-0" || messages[1000].Content != "message-1000" {
		t.Fatalf("expected messages in ascending position order, first=%q last=%q", messages[0].Content, messages[1000].Content)
	}
}

func TestUpdateThreadMetaShouldUpdateOwnedTitleVisibilityAndLabels(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	aliceThread, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix","messages":[{"role":"user","content":"please fix oauth"}]}`), Labels: []string{"auth"}})
	if err != nil {
		t.Fatalf("upload alice thread: %v", err)
	}
	bobThread, err := thread.UploadThreadSnapshot(bob, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"Bob oauth","messages":[{"role":"user","content":"please fix oauth"}]}`), Labels: []string{"auth"}})
	if err != nil {
		t.Fatalf("upload bob thread: %v", err)
	}

	updated, err := thread.UpdateThreadMeta(alice, aliceThread.ID, thread.MetaUpdate{Title: "Public OAuth fix", Visibility: model.ThreadVisibilityUnlisted, Labels: []string{"oauth", "shared"}})
	if err != nil {
		t.Fatalf("update thread meta: %v", err)
	}
	if updated.Title != "Public OAuth fix" || updated.Visibility != model.ThreadVisibilityUnlisted || updated.ShareSlug == "" {
		t.Fatalf("expected title, unlisted visibility, and share slug, got %#v", updated)
	}
	detail, err := thread.GetThreadDetail(alice, aliceThread.ID)
	if err != nil {
		t.Fatalf("get updated detail: %v", err)
	}
	if len(detail.Labels) != 2 || detail.Labels[0] != "oauth" || detail.Labels[1] != "shared" {
		t.Fatalf("expected replacement labels, got %#v", detail.Labels)
	}
	if _, err := thread.UpdateThreadMeta(alice, bobThread.ID, thread.MetaUpdate{Title: "stolen"}); err == nil {
		t.Fatalf("expected alice not to update bob's thread")
	}
}

func TestGetSharedThreadShouldReturnOnlyUnlistedThread(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice A.", Email: "alice@example.test", AvatarURL: "https://example.test/alice.png", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	publicThread, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-public","title":"Public OAuth fix","messages":[{"role":"user","content":"please fix oauth"}]}`), Labels: []string{"oauth"}})
	if err != nil {
		t.Fatalf("upload public thread: %v", err)
	}
	privateThread, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-private","title":"Private incident","messages":[{"role":"user","content":"secret"}]}`)})
	if err != nil {
		t.Fatalf("upload private thread: %v", err)
	}
	publicThread, err = thread.UpdateThreadMeta(alice, publicThread.ID, thread.MetaUpdate{Visibility: model.ThreadVisibilityUnlisted})
	if err != nil {
		t.Fatalf("make thread unlisted: %v", err)
	}
	detail, err := thread.GetSharedThread(publicThread.ShareSlug)
	if err != nil {
		t.Fatalf("get shared thread: %v", err)
	}
	if detail.ProviderThreadID != "T-public" || detail.Title != "Public OAuth fix" || detail.Visibility != model.ThreadVisibilityUnlisted {
		t.Fatalf("expected unlisted shared detail, got %#v", detail)
	}
	if len(detail.Labels) != 1 || detail.Labels[0] != "oauth" {
		t.Fatalf("expected oauth label, got %#v", detail.Labels)
	}
	if detail.Owner == nil || detail.Owner.Username != "alice" || detail.Owner.DisplayName != "Alice A." || detail.Owner.AvatarURL != "https://example.test/alice.png" {
		t.Fatalf("expected shared detail to include public owner fields, got %#v", detail.Owner)
	}
	if _, err := thread.GetSharedThread(privateThread.ShareSlug); err == nil {
		t.Fatalf("expected private thread not to be shared")
	}
	if err := thread.DeleteThread(alice, publicThread.ID); err != nil {
		t.Fatalf("delete public thread: %v", err)
	}
	if _, err := thread.GetSharedThread(publicThread.ShareSlug); err == nil {
		t.Fatalf("expected deleted shared thread not to be readable")
	}
	if _, err := thread.GetSharedThread("missing-slug"); err == nil {
		t.Fatalf("expected missing shared slug not to be readable")
	}
}

func TestArchiveThreadShouldToggleOwnedThreadArchivedState(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	aliceThread, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix","messages":[{"role":"user","content":"please fix oauth"}]}`)})
	if err != nil {
		t.Fatalf("upload alice thread: %v", err)
	}
	bobThread, err := thread.UploadThreadSnapshot(bob, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"Bob oauth","messages":[{"role":"user","content":"please fix oauth"}]}`)})
	if err != nil {
		t.Fatalf("upload bob thread: %v", err)
	}

	archived, err := thread.SetThreadArchived(alice, aliceThread.ID, true)
	if err != nil {
		t.Fatalf("archive thread: %v", err)
	}
	if !archived.Archived {
		t.Fatalf("expected archived thread, got %#v", archived)
	}
	unarchived, err := thread.SetThreadArchived(alice, aliceThread.ID, false)
	if err != nil {
		t.Fatalf("unarchive thread: %v", err)
	}
	if unarchived.Archived {
		t.Fatalf("expected unarchived thread, got %#v", unarchived)
	}
	if _, err := thread.SetThreadArchived(alice, bobThread.ID, true); err == nil {
		t.Fatalf("expected alice not to archive bob's thread")
	}
}

func TestDeleteThreadShouldHardDeleteOwnedThreadAndLabels(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	aliceThread, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix","messages":[{"role":"user","content":"please fix oauth"}]}`), Labels: []string{"auth"}})
	if err != nil {
		t.Fatalf("upload alice thread: %v", err)
	}
	bobThread, err := thread.UploadThreadSnapshot(bob, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"Bob oauth","messages":[{"role":"user","content":"please fix oauth"}]}`), Labels: []string{"auth"}})
	if err != nil {
		t.Fatalf("upload bob thread: %v", err)
	}

	if err := thread.DeleteThread(alice, aliceThread.ID); err != nil {
		t.Fatalf("delete alice thread: %v", err)
	}
	if _, err := thread.GetThreadDetail(alice, aliceThread.ID); err == nil {
		t.Fatalf("expected deleted thread not to be readable")
	}
	labels, err := model.ThreadLabelDB.Where("thread_id = ?", aliceThread.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch labels: %v", err)
	}
	if len(labels) != 0 {
		t.Fatalf("expected labels to be hard-deleted with thread, got %#v", labels)
	}
	if err := thread.DeleteThread(alice, bobThread.ID); err == nil {
		t.Fatalf("expected alice not to delete bob's thread")
	}
}

func closeModelDB(t *testing.T) func() {
	t.Helper()
	return func() {
		if err := model.CloseDB(); err != nil {
			t.Fatalf("close db: %v", err)
		}
	}
}

func TestThreadFacetsShouldReturnDistinctSortedReposAndLabels(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-a","title":"A","messages":[{"role":"user","content":"x"}]}`), Repo: "owner/web", Labels: []string{"bug", "auth"}}); err != nil {
		t.Fatalf("upload alice a: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-b","title":"B","messages":[{"role":"user","content":"y"}]}`), Repo: "owner/api", Labels: []string{"bug"}}); err != nil {
		t.Fatalf("upload alice b: %v", err)
	}
	// Bob's repos/labels must not leak into alice's facets.
	if _, err := thread.UploadThreadSnapshot(bob, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-c","title":"C","messages":[{"role":"user","content":"z"}]}`), Repo: "other/secret", Labels: []string{"private"}}); err != nil {
		t.Fatalf("upload bob c: %v", err)
	}

	facets, err := thread.ThreadFacets(alice)
	if err != nil {
		t.Fatalf("thread facets: %v", err)
	}
	if len(facets.Repos) != 2 || facets.Repos[0] != "owner/api" || facets.Repos[1] != "owner/web" {
		t.Fatalf("expected distinct sorted repos [owner/api owner/web], got %#v", facets.Repos)
	}
	if len(facets.Labels) != 2 || facets.Labels[0] != "auth" || facets.Labels[1] != "bug" {
		t.Fatalf("expected distinct sorted labels [auth bug], got %#v", facets.Labels)
	}
	if len(facets.Providers) != 1 ||
		facets.Providers[0].ID != model.ProviderCodex || facets.Providers[0].Label != "Codex" {
		t.Fatalf("expected used provider Codex in facets, got %#v", facets.Providers)
	}
	labelRows, err := model.ThreadLabelDB.Where("user_id = ?", alice.ID).Fetch(0, 100)
	if err != nil {
		t.Fatalf("fetch alice labels: %v", err)
	}
	if len(labelRows) != 3 {
		t.Fatalf("expected alice label rows to carry user_id, got %#v", labelRows)
	}
}

func TestThreadFacetsShouldReturnOnlyUsedProvidersInRegistryOrder(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	if err := user.SetUsedAgents([]string{model.ProviderAmp, model.ProviderQwenCode, model.ProviderCodex}); err != nil {
		t.Fatalf("set used agents: %v", err)
	}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save used agents: %v", err)
	}
	user.UsedAgentsJSON = ""

	facets, err := thread.ThreadFacets(user)
	if err != nil {
		t.Fatalf("thread facets: %v", err)
	}
	if len(facets.Providers) != 3 ||
		facets.Providers[0].ID != model.ProviderCodex ||
		facets.Providers[1].ID != model.ProviderQwenCode ||
		facets.Providers[2].ID != model.ProviderAmp {
		t.Fatalf("expected used providers in registry order [codex qwen_code amp], got %#v", facets.Providers)
	}
}

func TestRetiredAmpWritesAreRejectedByService(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "retired.db")); err != nil {
		t.Fatal(err)
	}
	user := &model.User{Username: "alice", APIKeyHash: "retired-test"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatal(err)
	}
	payload := thread.UploadPayload{Provider: model.ProviderAmp, Thread: json.RawMessage(`{"id":"T-retired","messages":[{"role":"user","content":"must not write"}]}`)}
	if _, _, err := thread.UploadIdentity(payload); err == nil || err.Error() != "unsupported provider" {
		t.Fatalf("identity rejection: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(user, payload); err == nil || err.Error() != "unsupported provider" {
		t.Fatalf("upload rejection: %v", err)
	}
}
