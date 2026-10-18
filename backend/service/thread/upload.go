package thread

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/burugo/thing"
	"github.com/jieliapp/jieli/backend/common/redact"
	"github.com/jieliapp/jieli/backend/model"
)

const uploadProcessingVersion = "redaction-v1"

const uploadErrorMessageQuotaExceeded = "message_quota_exceeded"

var attachmentURLPattern = regexp.MustCompile(`(?i)(?:https?://[^\s"'<>)]+|/attachments/[^\s"'<>)]+)`)
var ErrMessageQuotaExceeded = errors.New("message quota exceeded")

type UploadPayload struct {
	Provider  string          `json:"provider"`
	Thread    json.RawMessage `json:"thread"`
	Repo      string          `json:"repo"`
	RepoURL   string          `json:"repo_url"`
	Branch    string          `json:"branch"`
	SourceURL string          `json:"source_url"`
	Labels    []string        `json:"labels"`
}

type storedEnvMetadata struct {
	Initial struct {
		Trees []struct {
			URI        string `json:"uri"`
			Display    string `json:"displayName"`
			Repository struct {
				Ref string `json:"ref"`
				URL string `json:"url"`
			} `json:"repository"`
		} `json:"trees"`
		WorkspaceRoot    string `json:"workspaceRoot"`
		WorkingDirectory string `json:"workingDirectory"`
		Git              struct {
			Branch string `json:"branch"`
		} `json:"git"`
	} `json:"initial"`
}

type diffStats struct {
	Added   int `json:"added"`
	Deleted int `json:"deleted"`
	Changed int `json:"changed"`
}

type threadChangeStats struct {
	Added        int
	Deleted      int
	Changed      int
	FilesChanged int
}

type claudeCodeSnapshot struct {
	ID              string              `json:"id"`
	Title           string              `json:"title"`
	Model           string              `json:"model"`
	ReasoningEffort string              `json:"reasoning_effort"`
	CWD             string              `json:"cwd"`
	CreatedMS       int64               `json:"created_ms"`
	UpdatedMS       int64               `json:"updated_ms"`
	SummaryStats    json.RawMessage     `json:"summaryStats"`
	Messages        []claudeCodeMessage `json:"messages"`
}

type claudeCodeMessage struct {
	Role              string          `json:"role"`
	Content           json.RawMessage `json:"content"`
	MessageID         string          `json:"message_id"`
	ProtocolMessageID string          `json:"protocol_message_id"`
	Raw               json.RawMessage `json:"-"`
}

type codexSnapshot struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Model        string          `json:"model"`
	CWD          string          `json:"cwd"`
	CreatedMS    int64           `json:"created_ms"`
	UpdatedMS    int64           `json:"updated_ms"`
	SummaryStats json.RawMessage `json:"summaryStats"`
	Messages     []codexMessage  `json:"messages"`
}

type codexMessage struct {
	Role              string          `json:"role"`
	Content           json.RawMessage `json:"content"`
	MessageID         string          `json:"message_id"`
	ProtocolMessageID string          `json:"protocol_message_id"`
	Raw               json.RawMessage `json:"-"`
}

type storedMessage struct {
	Role              string
	Content           json.RawMessage
	MessageID         string
	ProtocolMessageID string
	Raw               json.RawMessage
}

func (message *claudeCodeMessage) UnmarshalJSON(raw []byte) error {
	type alias claudeCodeMessage
	var decoded alias
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*message = claudeCodeMessage(decoded)
	message.Raw = append(message.Raw[:0], raw...)
	return nil
}

func (message *codexMessage) UnmarshalJSON(raw []byte) error {
	type alias codexMessage
	var decoded alias
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*message = codexMessage(decoded)
	message.Raw = append(message.Raw[:0], raw...)
	return nil
}

// textFromContent extracts plain text from a message content value, which may
// be a string or an array of typed blocks (text/thinking/tool_use/...).
func textFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var blocks []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking string `json:"thinking"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "thinking":
			if block.Thinking != "" {
				parts = append(parts, block.Thinking)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// uploadRedactionDisabled reports whether server-side upload redaction is
// turned off via DISABLE_UPLOAD_REDACTION. Redaction is enabled by default;
// disabling it is intended for testing client-side plugin redaction.
func uploadRedactionDisabled() bool {
	disabled, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("DISABLE_UPLOAD_REDACTION")))
	return disabled
}

func redactUploadPayload(payload UploadPayload) UploadPayload {
	if uploadRedactionDisabled() {
		return payload
	}
	if len(payload.Thread) > 0 {
		payload.Thread = json.RawMessage(redact.RedactJSON(payload.Thread))
	}
	payload.Repo = redact.RedactText(payload.Repo)
	payload.RepoURL = redact.RedactText(payload.RepoURL)
	payload.Branch = redact.RedactText(payload.Branch)
	payload.SourceURL = redact.RedactText(payload.SourceURL)
	for i, label := range payload.Labels {
		payload.Labels[i] = redact.RedactText(label)
	}
	return payload
}

func UploadThreadSnapshot(user *model.User, payload UploadPayload) (*model.Thread, error) {
	if user == nil || user.ID == 0 {
		return nil, errors.New("user is required")
	}
	payload = redactUploadPayload(payload)
	switch payload.Provider {
	case model.ProviderClaudeCode:
		return uploadClaudeCodeSnapshot(user, payload)
	case model.ProviderCodex:
		return uploadCodexSnapshot(user, payload)
	case model.ProviderOpenCode:
		return uploadCodexLikeSnapshot(user, payload, model.ProviderOpenCode)
	case model.ProviderPi:
		return uploadCodexLikeSnapshot(user, payload, model.ProviderPi)
	default:
		return nil, errors.New("unsupported provider")
	}
}

func finishProviderUpload(user *model.User, provider string, stored *model.Thread, facetsChanged bool) (*model.Thread, error) {
	_, err := model.RecordUsedAgent(user, provider)
	if err != nil {
		return nil, err
	}
	_ = facetsChanged
	if err := clearLastUploadError(user); err != nil {
		return nil, err
	}
	// Auto-cleanup runs after the upload has succeeded and is best-effort:
	// MaybeAutoCleanup swallows its own errors so it can never fail the upload.
	_ = MaybeAutoCleanup(user)
	return stored, nil
}

func enforceMessageQuota(user *model.User, existing *model.Thread, provider string, providerThreadID string, incomingMessageCount int) error {
	if user == nil || user.ID == 0 || user.AutoCleanupEnabled {
		return nil
	}
	currentTotal, err := model.SumThreadMessageCountByUser(user.ID)
	if err != nil {
		return err
	}
	existingMessageCount := 0
	if existing != nil {
		existingMessageCount = existing.MessageCount
	}
	delta := incomingMessageCount - existingMessageCount
	if delta <= 0 {
		return nil
	}
	quota := model.MessageQuotaForUser(user)
	if quota < 0 {
		return nil
	}
	projected := currentTotal + delta
	if projected > quota {
		if err := recordQuotaExceededUploadError(user, model.UserUsageUploadError{
			Code:                   uploadErrorMessageQuotaExceeded,
			At:                     time.Now().UTC().Format(time.RFC3339),
			Provider:               provider,
			ProviderThreadID:       providerThreadID,
			CurrentMessages:        currentTotal,
			ExistingThreadMessages: existingMessageCount,
			IncomingMessages:       incomingMessageCount,
			ProjectedMessages:      projected,
			Quota:                  quota,
		}); err != nil {
			return err
		}
		return ErrMessageQuotaExceeded
	}
	return nil
}

func recordQuotaExceededUploadError(user *model.User, uploadError model.UserUsageUploadError) error {
	latest, err := model.UpdateUser(context.Background(), user.ID, func(current *model.User) error {
		usageInfo := current.UsageInfo()
		usageInfo.LastUploadError = &uploadError
		return current.SetUsageInfo(usageInfo)
	})
	if err != nil {
		return err
	}
	user.UsageInfoJSON = latest.UsageInfoJSON
	return nil
}

func clearLastUploadError(user *model.User) error {
	if user == nil || user.ID == 0 {
		return nil
	}
	latest, err := model.UpdateUser(context.Background(), user.ID, func(current *model.User) error {
		usageInfo := current.UsageInfo()
		usageInfo.LastUploadError = nil
		return current.SetUsageInfo(usageInfo)
	})
	if err != nil {
		return err
	}
	user.UsageInfoJSON = latest.UsageInfoJSON
	return nil
}

func UploadIdentity(payload UploadPayload) (string, string, error) {
	provider := payload.Provider
	var providerThreadID string
	switch payload.Provider {
	case model.ProviderClaudeCode:
		threadID, err := snapshotThreadIdentity(payload.Thread)
		if err != nil {
			return "", "", err
		}
		providerThreadID = claudeProviderThreadID(threadID)
	case model.ProviderCodex, model.ProviderOpenCode, model.ProviderPi:
		threadID, err := snapshotThreadIdentity(payload.Thread)
		if err != nil {
			return "", "", err
		}
		providerThreadID = claudeProviderThreadID(threadID)
	default:
		return "", "", errors.New("unsupported provider")
	}
	if strings.TrimSpace(providerThreadID) == "" || providerThreadID == "T-" {
		return "", "", errors.New("thread id is required")
	}
	return provider, providerThreadID, nil
}

func snapshotThreadIdentity(raw json.RawMessage) (string, error) {
	var snapshot struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return "", err
	}
	return snapshot.ID, nil
}

func uploadClaudeCodeSnapshot(user *model.User, payload UploadPayload) (*model.Thread, error) {
	snapshot, err := decodeClaudeCodeSnapshot(payload.Thread)
	if err != nil {
		return nil, err
	}
	if snapshot.ID == "" {
		return nil, errors.New("thread id is required")
	}
	providerThreadID := claudeProviderThreadID(snapshot.ID)
	payload.Repo, payload.RepoURL = normalizedRepoInfo(payload.Repo, payload.RepoURL, nil)
	title := snapshot.Title
	if title == "" {
		title = titleFromClaudeMessages(snapshot.Messages)
	}
	updatedMS := snapshot.UpdatedMS
	if updatedMS == 0 {
		updatedMS = snapshot.CreatedMS
	}
	existing, err := existingThread(user.ID, model.ProviderClaudeCode, providerThreadID)
	if err != nil {
		return nil, err
	}
	if err := enforceMessageQuota(user, existing, model.ProviderClaudeCode, providerThreadID, len(snapshot.Messages)); err != nil {
		return nil, err
	}
	uploadFingerprint := fingerprintForUpload(model.ProviderClaudeCode, providerThreadID, payload)
	if existing != nil && existing.UploadFingerprint == uploadFingerprint {
		// Fingerprint unchanged: only recompute the expensive change stats when the
		// existing thread is missing them (backfill), not on every no-op re-upload.
		if isZeroChangeStats(threadChangeStats{
			Added:        existing.LinesAdded,
			Deleted:      existing.LinesRemoved,
			FilesChanged: existing.FilesChanged,
		}) {
			changeStats, summaryStatsJSON := summaryStatsForUpload(snapshot.SummaryStats, len(snapshot.Messages), changeStatsForClaudeCodeMessages(snapshot.Messages))
			if !isZeroChangeStats(changeStats) {
				existing.LinesAdded = changeStats.Added
				existing.LinesRemoved = changeStats.Deleted
				existing.FilesChanged = changeStats.FilesChanged
				existing.SummaryStatsJSON = summaryStatsJSON
				if err := model.ThreadDB.Save(existing); err != nil {
					return nil, err
				}
			}
		}
		return finishProviderUpload(user, model.ProviderClaudeCode, existing, false)
	}
	changeStats, summaryStatsJSON := summaryStatsForUpload(snapshot.SummaryStats, len(snapshot.Messages), changeStatsForClaudeCodeMessages(snapshot.Messages))
	envJSON := claudeEnvJSON(snapshot.CWD)
	metaJSON := claudeMetaJSON(snapshot)
	stored := &model.Thread{
		UserID:                 user.ID,
		Provider:               model.ProviderClaudeCode,
		ProviderThreadID:       providerThreadID,
		Title:                  title,
		Visibility:             model.ThreadVisibilityPrivate,
		Repo:                   payload.Repo,
		RepoURL:                payload.RepoURL,
		Branch:                 payload.Branch,
		SourceURL:              payload.SourceURL,
		Preview:                previewFromStoredMessages(storedMessagesFromClaudeCode(snapshot.Messages)),
		SearchText:             searchTextForClaudeCode(snapshot, payload, title),
		MessageCount:           len(snapshot.Messages),
		LinesAdded:             changeStats.Added,
		LinesRemoved:           changeStats.Deleted,
		FilesChanged:           changeStats.FilesChanged,
		ThreadV:                1,
		ProviderCreatedMillis:  snapshot.CreatedMS,
		UserLastInteractedAtMS: updatedMS,
		ProviderUpdatedAt:      providerUpdatedAtFromMillis(updatedMS),
		CreatorUserID:          "local",
		EnvJSON:                envJSON,
		MetaJSON:               metaJSON,
		RelationshipsJSON:      "[]",
		SummaryStatsJSON:       summaryStatsJSON,
		ReasoningEffort:        snapshot.ReasoningEffort,
		UploadFingerprint:      uploadFingerprint,
	}
	if existing != nil {
		stored.ID = existing.ID
		stored.Visibility = existing.Visibility
		stored.ShareSlug = existing.ShareSlug
		stored.Archived = existing.Archived
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		return nil, err
	}
	if err := softDeleteIfEmpty(stored, len(snapshot.Messages)); err != nil {
		return nil, err
	}
	if err := replaceMessages(user.ID, stored.ID, storedMessagesFromClaudeCode(snapshot.Messages)); err != nil {
		return nil, err
	}
	labelsChanged, err := replaceLabels(user.ID, stored.ID, payload.Labels)
	if err != nil {
		return nil, err
	}
	if err := updateDailyActivity(user.ID, existing, len(snapshot.Messages), changeStats); err != nil {
		return nil, err
	}
	return finishProviderUpload(user, model.ProviderClaudeCode, stored, labelsChanged || threadRepoFacetChanged(existing, stored))
}

func uploadCodexSnapshot(user *model.User, payload UploadPayload) (*model.Thread, error) {
	return uploadCodexLikeSnapshot(user, payload, model.ProviderCodex)
}

func uploadCodexLikeSnapshot(user *model.User, payload UploadPayload, provider string) (*model.Thread, error) {
	snapshot, err := decodeCodexSnapshot(payload.Thread)
	if err != nil {
		return nil, err
	}
	if snapshot.ID == "" {
		return nil, errors.New("thread id is required")
	}
	providerThreadID := claudeProviderThreadID(snapshot.ID)
	payload.Repo, payload.RepoURL = normalizedRepoInfo(payload.Repo, payload.RepoURL, nil)
	title := snapshot.Title
	if title == "" {
		title = titleFromCodexMessages(snapshot.Messages)
	}
	updatedMS := snapshot.UpdatedMS
	if updatedMS == 0 {
		updatedMS = snapshot.CreatedMS
	}
	existing, err := existingThread(user.ID, provider, providerThreadID)
	if err != nil {
		return nil, err
	}
	if err := enforceMessageQuota(user, existing, provider, providerThreadID, len(snapshot.Messages)); err != nil {
		return nil, err
	}
	uploadFingerprint := fingerprintForUpload(provider, providerThreadID, payload)
	if existing != nil && existing.UploadFingerprint == uploadFingerprint {
		// Fingerprint unchanged: only recompute the expensive change stats when the
		// existing thread is missing them (backfill), not on every no-op re-upload.
		if isZeroChangeStats(threadChangeStats{
			Added:        existing.LinesAdded,
			Deleted:      existing.LinesRemoved,
			FilesChanged: existing.FilesChanged,
		}) {
			changeStats, summaryStatsJSON := summaryStatsForUpload(snapshot.SummaryStats, len(snapshot.Messages), changeStatsForCodexMessages(snapshot.Messages))
			if !isZeroChangeStats(changeStats) {
				existing.LinesAdded = changeStats.Added
				existing.LinesRemoved = changeStats.Deleted
				existing.FilesChanged = changeStats.FilesChanged
				existing.SummaryStatsJSON = summaryStatsJSON
				if err := model.ThreadDB.Save(existing); err != nil {
					return nil, err
				}
			}
		}
		return finishProviderUpload(user, provider, existing, false)
	}
	changeStats, summaryStatsJSON := summaryStatsForUpload(snapshot.SummaryStats, len(snapshot.Messages), changeStatsForCodexMessages(snapshot.Messages))
	envJSON := codexEnvJSON(snapshot.CWD)
	metaJSON := codexMetaJSON(snapshot)
	stored := &model.Thread{
		UserID:                 user.ID,
		Provider:               provider,
		ProviderThreadID:       providerThreadID,
		Title:                  title,
		Visibility:             model.ThreadVisibilityPrivate,
		Repo:                   payload.Repo,
		RepoURL:                payload.RepoURL,
		Branch:                 payload.Branch,
		SourceURL:              payload.SourceURL,
		Preview:                previewFromStoredMessages(storedMessagesFromCodex(snapshot.Messages)),
		SearchText:             searchTextForCodex(snapshot, payload, title),
		MessageCount:           len(snapshot.Messages),
		LinesAdded:             changeStats.Added,
		LinesRemoved:           changeStats.Deleted,
		FilesChanged:           changeStats.FilesChanged,
		ThreadV:                1,
		ProviderCreatedMillis:  snapshot.CreatedMS,
		UserLastInteractedAtMS: updatedMS,
		ProviderUpdatedAt:      providerUpdatedAtFromMillis(updatedMS),
		CreatorUserID:          "local",
		EnvJSON:                envJSON,
		MetaJSON:               metaJSON,
		RelationshipsJSON:      "[]",
		SummaryStatsJSON:       summaryStatsJSON,
		UploadFingerprint:      uploadFingerprint,
	}
	if existing != nil {
		stored.ID = existing.ID
		stored.Visibility = existing.Visibility
		stored.ShareSlug = existing.ShareSlug
		stored.Archived = existing.Archived
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		return nil, err
	}
	if err := softDeleteIfEmpty(stored, len(snapshot.Messages)); err != nil {
		return nil, err
	}
	if err := replaceMessages(user.ID, stored.ID, storedMessagesFromCodex(snapshot.Messages)); err != nil {
		return nil, err
	}
	labelsChanged, err := replaceLabels(user.ID, stored.ID, payload.Labels)
	if err != nil {
		return nil, err
	}
	if err := updateDailyActivity(user.ID, existing, len(snapshot.Messages), changeStats); err != nil {
		return nil, err
	}
	return finishProviderUpload(user, provider, stored, labelsChanged || threadRepoFacetChanged(existing, stored))
}

// softDeleteIfEmpty marks a newly-created thread as deleted when it carries
// zero messages (e.g. a StartSession hook upload before any conversation).
// If the thread already existed as a soft-deleted placeholder and now has
// messages, the Save above already set Deleted=false (BaseModel zero value),
// so it becomes visible again automatically.
func softDeleteIfEmpty(stored *model.Thread, messageCount int) error {
	if messageCount == 0 {
		return model.ThreadDB.SoftDelete(stored)
	}
	return nil
}

func threadRepoFacetChanged(existing *model.Thread, stored *model.Thread) bool {
	if stored == nil {
		return false
	}
	if existing == nil {
		return stored.Repo != ""
	}
	return existing.Repo != stored.Repo
}

func claudeProviderThreadID(raw string) string {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "T-") {
		return value
	}
	return "T-" + value
}

func fingerprintForUpload(provider string, providerThreadID string, payload UploadPayload) string {
	labels := make([]string, 0, len(payload.Labels))
	for _, label := range payload.Labels {
		label = strings.TrimSpace(label)
		if label != "" {
			labels = append(labels, label)
		}
	}
	sort.Strings(labels)
	// Hash the thread payload bytes directly. Skipping JSON normalization can only
	// ever produce a false "changed" (same content, different byte order -> one
	// extra reprocess), never a false "unchanged" (identical bytes always mean
	// identical content), so it is safe; agent CLIs serialize with stable key
	// order in practice.
	fingerprintPayload := map[string]any{
		"provider":           provider,
		"provider_thread_id": providerThreadID,
		"thread":             json.RawMessage(payload.Thread),
		"repo":               payload.Repo,
		"repo_url":           payload.RepoURL,
		"branch":             payload.Branch,
		"source_url":         payload.SourceURL,
		"labels":             labels,
	}
	fingerprintPayload["processing_version"] = uploadProcessingVersion
	raw, _ := json.Marshal(fingerprintPayload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func decodeClaudeCodeSnapshot(raw json.RawMessage) (claudeCodeSnapshot, error) {
	var snapshot claudeCodeSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return claudeCodeSnapshot{}, err
	}
	return snapshot, nil
}

func decodeCodexSnapshot(raw json.RawMessage) (codexSnapshot, error) {
	var snapshot codexSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return codexSnapshot{}, err
	}
	return snapshot, nil
}

func titleFromClaudeMessages(messages []claudeCodeMessage) string {
	for _, message := range messages {
		if message.Role != "user" {
			continue
		}
		title := truncatedTitle(textFromContent(message.Content), 80)
		if title != "" {
			return title
		}
	}
	return "Claude Code session"
}

func titleFromCodexMessages(messages []codexMessage) string {
	for _, message := range messages {
		if message.Role != "user" {
			continue
		}
		title := truncatedTitle(textFromContent(message.Content), 80)
		if title != "" {
			return title
		}
	}
	return "Codex session"
}

func truncatedTitle(text string, maxRunes int) string {
	title := strings.TrimSpace(text)
	if title == "" {
		return ""
	}
	runes := []rune(title)
	if len(runes) <= maxRunes {
		return title
	}
	return string(runes[:maxRunes])
}

func claudeEnvJSON(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		return ""
	}
	raw, _ := json.Marshal(map[string]any{
		"initial": map[string]any{
			"workspaceRoot":    cwd,
			"workingDirectory": cwd,
		},
	})
	return string(raw)
}

func claudeMetaJSON(snapshot claudeCodeSnapshot) string {
	raw, _ := json.Marshal(map[string]any{
		"cwd":   snapshot.CWD,
		"model": snapshot.Model,
	})
	return string(raw)
}

func codexEnvJSON(cwd string) string {
	return claudeEnvJSON(cwd)
}

func codexMetaJSON(snapshot codexSnapshot) string {
	raw, _ := json.Marshal(map[string]any{
		"cwd":   snapshot.CWD,
		"model": snapshot.Model,
	})
	return string(raw)
}

func providerUpdatedAtFromMillis(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339Nano)
}

func repoFromEnv(raw json.RawMessage) string {
	env := parseStoredEnv(raw)
	for _, tree := range env.Initial.Trees {
		if repo, _ := repoInfoFromRepositoryURL(tree.Repository.URL); repo != "" {
			return repo
		}
	}
	return ""
}

func repoURLFromEnv(raw json.RawMessage) string {
	_, repoURL := repoInfoFromEnv(raw)
	return repoURL
}

func normalizedRepoInfo(repo string, repoURL string, env json.RawMessage) (string, string) {
	repo = strings.TrimSpace(repo)
	repoURL = strings.TrimSpace(repoURL)
	if parsedRepo, parsedRepoURL := repoInfoFromRepositoryURL(repoURL); parsedRepo != "" || parsedRepoURL != "" {
		if parsedRepo != "" {
			repo = parsedRepo
		}
		repoURL = parsedRepoURL
	} else {
		repoURL = cleanRepoURL(repoURL)
	}
	if repo == "" || repoURL == "" {
		envRepo, envRepoURL := repoInfoFromEnv(env)
		if repo == "" {
			repo = envRepo
		}
		if repoURL == "" {
			repoURL = envRepoURL
		}
	}
	return repo, repoURL
}

func repoInfoFromEnv(raw json.RawMessage) (string, string) {
	env := parseStoredEnv(raw)
	for _, tree := range env.Initial.Trees {
		if repo, repoURL := repoInfoFromRepositoryURL(tree.Repository.URL); repo != "" || repoURL != "" {
			return repo, repoURL
		}
	}
	return "", ""
}

func branchFromEnv(raw json.RawMessage) string {
	env := parseStoredEnv(raw)
	for _, tree := range env.Initial.Trees {
		if branch := branchFromRef(tree.Repository.Ref); branch != "" {
			return branch
		}
	}
	return strings.TrimSpace(env.Initial.Git.Branch)
}

func parseStoredEnv(raw json.RawMessage) storedEnvMetadata {
	var env storedEnvMetadata
	if len(raw) == 0 {
		return env
	}
	_ = json.Unmarshal(raw, &env)
	return env
}

func branchFromRef(ref string) string {
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	return ref
}

func repoFromRepositoryURL(raw string) string {
	repo, _ := repoInfoFromRepositoryURL(raw)
	return repo
}

func repoInfoFromRepositoryURL(raw string) (string, string) {
	value := cleanGitRemoteURL(raw)
	if value == "" {
		return "", ""
	}
	if !strings.Contains(value, "://") {
		return repoInfoFromSCPRepositoryURL(value)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "ssh" && parsed.Scheme != "git" {
		return "", ""
	}
	host := parsed.Hostname()
	repo := validRepoPath(parsed.Path)
	if host == "" || repo == "" {
		return "", ""
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return repo, cleanRepoURL(parsed.Scheme + "://" + hostWithPort(parsed) + "/" + repo)
	}
	return repo, "https://" + host + "/" + repo
}

func repoInfoFromSCPRepositoryURL(value string) (string, string) {
	index := strings.Index(value, ":")
	if index <= 0 || strings.Contains(value[:index], "/") {
		return "", ""
	}
	host := value[:index]
	if at := strings.LastIndex(host, "@"); at >= 0 {
		host = host[at+1:]
	}
	repo := validRepoPath(value[index+1:])
	if host == "" || repo == "" {
		return "", ""
	}
	return repo, "https://" + host + "/" + repo
}

func cleanGitRemoteURL(raw string) string {
	value := strings.Trim(strings.TrimSpace(raw), "/")
	return strings.TrimSuffix(value, ".git")
}

func hostWithPort(parsed *url.URL) string {
	if parsed.Port() == "" {
		return parsed.Hostname()
	}
	return parsed.Hostname() + ":" + parsed.Port()
}

func cleanRepoURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return ""
	}
	repo := validRepoPath(parsed.Path)
	if repo == "" {
		return ""
	}
	return parsed.Scheme + "://" + hostWithPort(parsed) + "/" + repo
}

func validRepoPath(raw string) string {
	path := strings.Trim(strings.TrimSpace(raw), "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return ""
	}
	for _, part := range parts {
		if !validRepoPathSegment(part) {
			return ""
		}
	}
	return strings.Join(parts, "/")
}

func validRepoPathSegment(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '.' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func existingThread(userID int64, provider string, providerThreadID string) (*model.Thread, error) {
	threads, err := model.ThreadDB.Where("user_id = ? AND provider = ? AND provider_thread_id = ?", userID, provider, providerThreadID).WithDeleted().Fetch(0, 1)
	if err != nil {
		return nil, err
	}
	if len(threads) == 0 {
		return nil, nil
	}
	return threads[0], nil
}

func storedMessagesFromClaudeCode(messages []claudeCodeMessage) []storedMessage {
	stored := make([]storedMessage, 0, len(messages))
	for _, message := range messages {
		stored = append(stored, storedMessage{
			Role:              message.Role,
			Content:           message.Content,
			MessageID:         message.MessageID,
			ProtocolMessageID: message.ProtocolMessageID,
			Raw:               message.Raw,
		})
	}
	return stored
}

func storedMessagesFromCodex(messages []codexMessage) []storedMessage {
	stored := make([]storedMessage, 0, len(messages))
	for _, message := range messages {
		stored = append(stored, storedMessage{
			Role:              message.Role,
			Content:           message.Content,
			MessageID:         message.MessageID,
			ProtocolMessageID: message.ProtocolMessageID,
			Raw:               message.Raw,
		})
	}
	return stored
}

type threadMessageMetadata struct {
	ID                int64
	Position          int
	ProviderMessageID string
	ProtocolMessageID string
	Role              string
	ContentHash       string
}

func loadThreadMessageMetadata(threadID int64) ([]threadMessageMetadata, error) {
	rows, err := model.ThreadMessageDB.DB().QueryContext(
		context.Background(),
		`SELECT id, position, provider_message_id, protocol_message_id, role, content_hash
		 FROM thread_messages
		 WHERE thread_id = $1
		 ORDER BY position ASC`,
		threadID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metadata := make([]threadMessageMetadata, 0)
	for rows.Next() {
		var row threadMessageMetadata
		if err := rows.Scan(&row.ID, &row.Position, &row.ProviderMessageID, &row.ProtocolMessageID, &row.Role, &row.ContentHash); err != nil {
			return nil, err
		}
		metadata = append(metadata, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return metadata, nil
}

func replaceMessages(userID int64, threadID int64, messages []storedMessage) error {
	existingMessages, err := loadThreadMessageMetadata(threadID)
	if err != nil {
		return err
	}
	existingByPosition := make(map[int]threadMessageMetadata, len(existingMessages))
	for _, message := range existingMessages {
		existingByPosition[message.Position] = message
	}
	changedRows := make([]*model.ThreadMessage, 0, len(messages))
	newMessages := make([]storedMessage, 0)
	for index, message := range messages {
		raw := message.Raw
		if len(raw) == 0 {
			encoded, err := json.Marshal(message)
			if err != nil {
				return err
			}
			raw = encoded
		}
		existing, found := existingByPosition[index]
		stored := &model.ThreadMessage{ThreadID: threadID, Position: index}
		if !found {
			message.Raw = raw
			newMessages = append(newMessages, message)
		} else {
			delete(existingByPosition, index)
			stored.ID = existing.ID
			stored.ProviderMessageID = existing.ProviderMessageID
			stored.ProtocolMessageID = existing.ProtocolMessageID
			stored.Role = existing.Role
			stored.ContentHash = existing.ContentHash
		}
		changed := stored.ID == 0 || stored.ProviderMessageID != message.MessageID || stored.ProtocolMessageID != message.ProtocolMessageID || stored.Role != message.Role
		if !changed {
			// A missing legacy hash is rewritten from the incoming payload; the
			// upload diff never loads or decompresses the stored raw_zstd blob.
			changed = stored.ContentHash == "" || stored.ContentHash != model.HashRawJSON(string(raw))
		}
		if !changed {
			continue
		}
		stored.ProviderMessageID = message.MessageID
		stored.ProtocolMessageID = message.ProtocolMessageID
		stored.Role = message.Role
		stored.SetRawJSON(string(raw))
		changedRows = append(changedRows, stored)
	}
	removedRows := make([]*model.ThreadMessage, 0, len(existingByPosition))
	for _, message := range existingByPosition {
		removed := &model.ThreadMessage{ThreadID: threadID, Position: message.Position}
		removed.ID = message.ID
		removedRows = append(removedRows, removed)
	}
	// One transaction + one table cache invalidation, instead of putting complete
	// raw_zstd models into Thing's item cache. New rows
	// land on positions with no existing row and removed rows are all tail
	// positions (>= len(messages)), so neither batch collides on the
	// (thread_id, position) unique index.
	if err := writeThreadMessageChanges(threadID, changedRows, removedRows); err != nil {
		return err
	}
	if err := addThreadAttachments(userID, threadID, newMessages); err != nil {
		return err
	}
	return nil
}

func writeThreadMessageChanges(threadID int64, changedRows []*model.ThreadMessage, removedRows []*model.ThreadMessage) error {
	if len(changedRows) == 0 && len(removedRows) == 0 {
		return nil
	}
	err := thing.Transaction(context.Background(), func(tx *thing.TxSession) error {
		now := time.Now().UTC()
		for _, row := range changedRows {
			if row.ID == 0 {
				if _, err := tx.Exec(
					`INSERT INTO thread_messages
					 (created_at, updated_at, deleted, thread_id, position, provider_message_id, protocol_message_id, role, raw_zstd, content_hash)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
					now, now, false, threadID, row.Position, row.ProviderMessageID, row.ProtocolMessageID, row.Role, row.RawZstd, row.ContentHash,
				); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.Exec(
				`UPDATE thread_messages
				 SET updated_at = ?, provider_message_id = ?, protocol_message_id = ?, role = ?, raw_zstd = ?, content_hash = ?
				 WHERE id = ? AND thread_id = ?`,
				now, row.ProviderMessageID, row.ProtocolMessageID, row.Role, row.RawZstd, row.ContentHash, row.ID, threadID,
			); err != nil {
				return err
			}
		}
		for _, row := range removedRows {
			if _, err := tx.Exec("DELETE FROM thread_messages WHERE id = ? AND thread_id = ?", row.ID, threadID); err != nil {
				return err
			}
		}
		tx.Touch("thread_messages")
		return nil
	})
	if err != nil {
		return err
	}
	invalidateThreadMessagesCache(threadID)
	return nil
}

func addThreadAttachments(userID int64, threadID int64, messages []storedMessage) error {
	if len(messages) == 0 {
		return nil
	}
	attachments, err := referencedAttachments(userID, messages)
	if err != nil {
		return err
	}
	if len(attachments) == 0 {
		return nil
	}
	existingLinks, err := model.ThreadAttachmentDB.Where("thread_id = ?", threadID).All()
	if err != nil {
		return err
	}
	linkedAttachmentIDs := make(map[int64]struct{}, len(existingLinks))
	for _, link := range existingLinks {
		linkedAttachmentIDs[link.AttachmentID] = struct{}{}
	}
	for _, attachment := range attachments {
		if _, ok := linkedAttachmentIDs[attachment.ID]; ok {
			continue
		}
		if attachment.Deleted {
			attachment.Deleted = false
			if err := model.AttachmentDB.Save(attachment); err != nil {
				return err
			}
		}
		row := &model.ThreadAttachment{
			UserID:        userID,
			ThreadID:      threadID,
			AttachmentID:  attachment.ID,
			PublicID:      attachment.PublicID,
			StorageDriver: attachment.StorageDriver,
			StorageKey:    attachment.StorageKey,
		}
		if err := model.ThreadAttachmentDB.Save(row); err != nil {
			return err
		}
	}
	return nil
}

func referencedAttachments(userID int64, messages []storedMessage) ([]*model.Attachment, error) {
	publicIDs := map[string]struct{}{}
	storageKeys := map[string]struct{}{}
	for _, message := range messages {
		raw := message.Raw
		if len(raw) == 0 {
			encoded, err := json.Marshal(message)
			if err != nil {
				return nil, err
			}
			raw = encoded
		}
		references := attachmentReferencesFromText(string(raw))
		for _, publicID := range references.PublicIDs {
			publicIDs[publicID] = struct{}{}
		}
		for _, storageKey := range references.StorageKeys {
			storageKeys[storageKey] = struct{}{}
		}
	}
	byID := map[int64]*model.Attachment{}
	if len(publicIDs) != 0 {
		ids := make([]string, 0, len(publicIDs))
		for publicID := range publicIDs {
			ids = append(ids, publicID)
		}
		rows, err := model.AttachmentDB.Where("user_id = ? AND public_id IN (?)", userID, ids).WithDeleted().Fetch(0, len(ids))
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			byID[row.ID] = row
		}
	}
	if len(storageKeys) != 0 {
		keys := make([]string, 0, len(storageKeys))
		for storageKey := range storageKeys {
			keys = append(keys, storageKey)
		}
		rows, err := model.AttachmentDB.Where("user_id = ? AND storage_key IN (?)", userID, keys).WithDeleted().Fetch(0, len(keys))
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			byID[row.ID] = row
		}
	}
	attachments := make([]*model.Attachment, 0, len(byID))
	for _, attachment := range byID {
		attachments = append(attachments, attachment)
	}
	sort.Slice(attachments, func(i, j int) bool {
		return attachments[i].ID < attachments[j].ID
	})
	return attachments, nil
}

type attachmentReferences struct {
	PublicIDs   []string
	StorageKeys []string
}

func attachmentReferencesFromText(text string) attachmentReferences {
	matches := attachmentURLPattern.FindAllString(text, -1)
	references := attachmentReferences{
		PublicIDs:   make([]string, 0, len(matches)),
		StorageKeys: make([]string, 0, len(matches)),
	}
	for _, match := range matches {
		parsedPath := match
		lower := strings.ToLower(match)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			parsed, err := url.Parse(match)
			if err != nil {
				continue
			}
			parsedPath = parsed.EscapedPath()
		}
		publicID, storageKey := attachmentReferenceFromPath(parsedPath)
		if publicID != "" {
			references.PublicIDs = append(references.PublicIDs, publicID)
		}
		if storageKey != "" {
			references.StorageKeys = append(references.StorageKeys, storageKey)
		}
	}
	return references
}

func attachmentReferenceFromPath(rawPath string) (string, string) {
	value := rawPath
	if parts := strings.Split(rawPath, "/attachments/"); len(parts) >= 2 {
		value = parts[len(parts)-1]
	}
	if value == "" || strings.Contains(value, "/") {
		value = path.Base(rawPath)
	}
	value = strings.Trim(value, " ")
	if unescaped, err := url.PathUnescape(value); err == nil {
		value = unescaped
	}
	if value == "" || value == "." || strings.Contains(value, "/") {
		return "", ""
	}
	storageKey := value
	if index := strings.LastIndex(value, "."); index > 0 {
		value = value[:index]
	}
	return value, storageKey
}

func replaceLabels(userID int64, threadID int64, labels []string) (bool, error) {
	existingLabels, err := model.ThreadLabelDB.Where("thread_id = ?", threadID).Fetch(0, 1000)
	if err != nil {
		return false, err
	}
	changed := false
	existingByLabel := make(map[string]*model.ThreadLabel, len(existingLabels))
	for _, label := range existingLabels {
		existingByLabel[label.Label] = label
	}
	desiredLabels := make([]string, 0, len(labels))
	desiredSet := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if _, exists := desiredSet[label]; exists {
			continue
		}
		desiredSet[label] = struct{}{}
		desiredLabels = append(desiredLabels, label)
	}
	for _, existing := range existingLabels {
		if _, keep := desiredSet[existing.Label]; keep {
			continue
		}
		if err := model.ThreadLabelDB.Delete(existing); err != nil {
			return false, err
		}
		changed = true
	}
	for _, label := range desiredLabels {
		if _, exists := existingByLabel[label]; exists {
			continue
		}
		if err := model.ThreadLabelDB.Save(&model.ThreadLabel{UserID: userID, ThreadID: threadID, Label: label}); err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}

func updateDailyActivity(userID int64, prev *model.Thread, messageCount int, diff threadChangeStats) error {
	if messageCount == 0 {
		return nil
	}
	date := time.Now().Format("2006-01-02")
	activities, err := model.DailyActivityDB.Where("user_id = ? AND date = ?", userID, date).Fetch(0, 1)
	if err != nil {
		return err
	}
	activity := &model.DailyActivity{UserID: userID, Date: date}
	if len(activities) != 0 {
		activity = activities[0]
	}
	// Accumulate only net-new contribution so resending the same thread does not
	// inflate the daily total and a rewind (fewer messages) never reduces it.
	var oldMessageCount, oldAdded, oldRemoved, oldChanged, oldFiles int
	threadDelta := 1
	if prev != nil && prev.MessageCount > 0 {
		threadDelta = 0
		oldMessageCount = prev.MessageCount
		oldAdded = prev.LinesAdded
		oldRemoved = prev.LinesRemoved
		oldChanged = parseSummaryStats(json.RawMessage(prev.SummaryStatsJSON)).Changed
		oldFiles = prev.FilesChanged
	}
	messageDelta := maxInt(0, messageCount-oldMessageCount)
	addedDelta := maxInt(0, diff.Added-oldAdded)
	removedDelta := maxInt(0, diff.Deleted-oldRemoved)
	changedDelta := maxInt(0, diff.Changed-oldChanged)
	filesDelta := maxInt(0, diff.FilesChanged-oldFiles)
	if threadDelta == 0 && messageDelta == 0 && addedDelta == 0 && removedDelta == 0 && changedDelta == 0 && filesDelta == 0 {
		return nil
	}
	activity.ThreadCount += threadDelta
	activity.MessageCount += messageDelta
	activity.LinesAdded += addedDelta
	activity.LinesRemoved += removedDelta
	activity.LinesChanged += changedDelta
	activity.FilesChanged += filesDelta
	return model.DailyActivityDB.Save(activity)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func summaryStatsForUpload(rawSummaryStats json.RawMessage, messageCount int, inferredStats threadChangeStats) (threadChangeStats, string) {
	stats := parseSummaryStats(rawSummaryStats)
	summaryStatsJSON := rawOrEmpty(rawSummaryStats)
	if !hasLineChangeStats(stats) && !isZeroChangeStats(inferredStats) {
		if summaryStatsJSON != "" && !isZeroChangeStats(stats) {
			return inferredStats, summaryStatsJSON
		}
		return inferredStats, summaryStatsJSONForChanges(messageCount, inferredStats)
	}
	if stats.FilesChanged == 0 {
		stats.FilesChanged = inferredStats.FilesChanged
	}
	if summaryStatsJSON == "" {
		summaryStatsJSON = summaryStatsJSONForChanges(messageCount, stats)
	}
	return stats, summaryStatsJSON
}

func isZeroChangeStats(stats threadChangeStats) bool {
	return stats.Added == 0 && stats.Deleted == 0 && stats.Changed == 0 && stats.FilesChanged == 0
}

func hasLineChangeStats(stats threadChangeStats) bool {
	return stats.Added != 0 || stats.Deleted != 0 || stats.Changed != 0
}

func parseSummaryStats(raw json.RawMessage) threadChangeStats {
	var summary struct {
		DiffStats    diffStats `json:"diffStats"`
		FilesChanged int       `json:"filesChanged"`
	}
	if err := json.Unmarshal(raw, &summary); err != nil {
		return threadChangeStats{}
	}
	return threadChangeStats{
		Added:        summary.DiffStats.Added,
		Deleted:      summary.DiffStats.Deleted,
		Changed:      summary.DiffStats.Changed,
		FilesChanged: summary.FilesChanged,
	}
}

func changeStatsForClaudeCodeMessages(messages []claudeCodeMessage) threadChangeStats {
	stats := threadChangeStats{}
	files := make(map[string]struct{})
	for _, message := range messages {
		for _, edit := range editFileInputs(message.Content) {
			files[edit.Path] = struct{}{}
			added, deleted, changed := lineDiffStats(edit.OldStr, edit.NewStr)
			stats.Added += added
			stats.Deleted += deleted
			stats.Changed += changed
		}
	}
	stats.FilesChanged = len(files)
	return stats
}

func changeStatsForCodexMessages(messages []codexMessage) threadChangeStats {
	stats := threadChangeStats{}
	files := make(map[string]struct{})
	for _, message := range messages {
		for _, edit := range editFileInputs(message.Content) {
			files[edit.Path] = struct{}{}
			added, deleted, changed := lineDiffStats(edit.OldStr, edit.NewStr)
			stats.Added += added
			stats.Deleted += deleted
			stats.Changed += changed
		}
	}
	stats.FilesChanged = len(files)
	return stats
}

type editFileInput struct {
	Path   string
	OldStr string
	NewStr string
}

func editFileInputs(raw json.RawMessage) []editFileInput {
	var blocks []struct {
		Type  string                     `json:"type"`
		Name  string                     `json:"name"`
		Input map[string]json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil
	}
	edits := make([]editFileInput, 0)
	for _, block := range blocks {
		if block.Type != "tool_use" {
			continue
		}
		edits = append(edits, editFileInputsForTool(block.Name, block.Input)...)
	}
	return edits
}

func editFileInputsForTool(name string, input map[string]json.RawMessage) []editFileInput {
	toolName := strings.ToLower(strings.TrimSpace(name))
	if toolName == "apply_patch" {
		patch, ok := stringFieldOK(input, "patchtext", "patch_text", "patchText")
		if !ok {
			return nil
		}
		return patchFileInputs(patch)
	}

	path := stringField(input, "path", "file_path", "filePath")
	if path == "" {
		return nil
	}

	switch toolName {
	case "edit_file", "edit":
		return []editFileInput{{
			Path:   path,
			OldStr: stringField(input, "old_str", "oldStr", "old_string", "oldString"),
			NewStr: stringField(input, "new_str", "newStr", "new_string", "newString"),
		}}
	case "write", "write_file", "create_file":
		content, ok := stringFieldOK(input, "content")
		if !ok {
			return nil
		}
		return []editFileInput{{Path: path, NewStr: content}}
	default:
		return nil
	}
}

func patchFileInputs(patch string) []editFileInput {
	var edits []editFileInput
	var current *editFileInput
	flush := func() {
		if current == nil {
			return
		}
		if current.OldStr != "" || current.NewStr != "" {
			edits = append(edits, *current)
		}
		current = nil
	}

	for _, line := range strings.Split(strings.ReplaceAll(patch, "\r\n", "\n"), "\n") {
		if path := patchFilePath(line); path != "" {
			flush()
			current = &editFileInput{Path: path}
			continue
		}
		if current == nil || strings.HasPrefix(line, "@@") || strings.HasPrefix(line, "*** ") {
			continue
		}
		if strings.HasPrefix(line, "+") {
			current.NewStr += strings.TrimPrefix(line, "+") + "\n"
		} else if strings.HasPrefix(line, "-") {
			current.OldStr += strings.TrimPrefix(line, "-") + "\n"
		}
	}
	flush()
	return edits
}

func patchFilePath(line string) string {
	for _, prefix := range []string{"*** Add File:", "*** Update File:", "*** Delete File:"} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func stringField(fields map[string]json.RawMessage, names ...string) string {
	value, _ := stringFieldOK(fields, names...)
	return value
}

func stringFieldOK(fields map[string]json.RawMessage, names ...string) (string, bool) {
	for _, name := range names {
		raw, ok := fields[name]
		if !ok {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			return value, true
		}
	}
	return "", false
}

func lineDiffStats(oldText string, newText string) (added int, deleted int, changed int) {
	oldLines := splitComparableLines(oldText)
	newLines := splitComparableLines(newText)
	common := commonLineCount(oldLines, newLines)
	removed := len(oldLines) - common
	inserted := len(newLines) - common
	if removed < inserted {
		changed = removed
	} else {
		changed = inserted
	}
	added = inserted - changed
	deleted = removed - changed
	return added, deleted, changed
}

func commonLineCount(a []string, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if len(b) > len(a) {
		a, b = b, a
	}
	row := make([]int, len(b)+1)
	for _, left := range a {
		previous := 0
		for j, right := range b {
			current := row[j+1]
			if left == right {
				row[j+1] = previous + 1
			} else if row[j] > row[j+1] {
				row[j+1] = row[j]
			}
			previous = current
		}
	}
	return row[len(b)]
}

func splitComparableLines(text string) []string {
	if text == "" {
		return []string{}
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

func searchTextForClaudeCode(snapshot claudeCodeSnapshot, payload UploadPayload, title string) string {
	parts := []string{title, payload.Repo, payload.Branch, payload.SourceURL, snapshot.CWD, snapshot.Model}
	parts = append(parts, payload.Labels...)
	for _, message := range snapshot.Messages {
		parts = append(parts, message.Role, textFromContent(message.Content))
	}
	return strings.Join(parts, "\n")
}

func searchTextForCodex(snapshot codexSnapshot, payload UploadPayload, title string) string {
	parts := []string{title, payload.Repo, payload.Branch, payload.SourceURL, snapshot.CWD, snapshot.Model}
	parts = append(parts, payload.Labels...)
	for _, message := range snapshot.Messages {
		parts = append(parts, message.Role, textFromContent(message.Content))
	}
	return strings.Join(parts, "\n")
}

func rawOrEmpty(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	return string(raw)
}

func summaryStatsJSONForChanges(messageCount int, stats threadChangeStats) string {
	raw, _ := json.Marshal(map[string]any{
		"diffStats":    map[string]int{"added": stats.Added, "changed": stats.Changed, "deleted": stats.Deleted},
		"filesChanged": stats.FilesChanged,
		"messageCount": messageCount,
	})
	return string(raw)
}
