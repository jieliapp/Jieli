package model

import (
	"context"

	"github.com/burugo/thing"
)

const (
	ProviderAmp              = "amp"
	ProviderClaudeCode       = "claude_code"
	ProviderCodex            = "codex"
	ProviderOpenCode         = "opencode"
	ProviderPi               = "pi"
	ProviderGemini           = "gemini"
	ProviderGitHubCopilot    = "github_copilot"
	ProviderAntigravity      = "antigravity"
	ProviderQwenCode         = "qwen_code"
	ProviderCursor           = "cursor"
	ProviderWindsurf         = "windsurf"
	ProviderDroid            = "droid"
	ThreadVisibilityPrivate  = "private"
	ThreadVisibilityUnlisted = "unlisted"
)

type ProviderInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func SupportedProviders() []ProviderInfo {
	return []ProviderInfo{
		{ID: ProviderClaudeCode, Label: "Claude Code"},
		{ID: ProviderCodex, Label: "Codex"},
		{ID: ProviderOpenCode, Label: "OpenCode"},
		{ID: ProviderPi, Label: "Pi"},
		{ID: ProviderGemini, Label: "Gemini CLI"},
		{ID: ProviderGitHubCopilot, Label: "GitHub Copilot"},
		{ID: ProviderAntigravity, Label: "Antigravity"},
		{ID: ProviderQwenCode, Label: "Qwen Code"},
		{ID: ProviderCursor, Label: "Cursor"},
		{ID: ProviderWindsurf, Label: "Windsurf"},
		{ID: ProviderDroid, Label: "Droid"},
		{ID: ProviderAmp, Label: "Amp"},
	}
}

type Thread struct {
	thing.BaseModel
	UserID                 int64  `json:"user_id" db:"user_id,index,unique:uniq_threads_user_provider_thread"`
	Provider               string `json:"provider" db:"provider,index,unique:uniq_threads_user_provider_thread"`
	ProviderThreadID       string `json:"provider_thread_id" db:"provider_thread_id,unique:uniq_threads_user_provider_thread"`
	Title                  string `json:"title" db:"title"`
	Visibility             string `json:"visibility" db:"visibility,index,default:'private'"`
	ShareSlug              string `json:"share_slug" db:"share_slug,index"`
	Repo                   string `json:"repo" db:"repo,index"`
	RepoURL                string `json:"repo_url" db:"repo_url"`
	Branch                 string `json:"branch" db:"branch"`
	SourceURL              string `json:"source_url" db:"source_url"`
	Preview                string `json:"preview" db:"preview,default:''"`
	SearchText             string `json:"search_text" db:"search_text,default:''"`
	MessageCount           int    `json:"message_count" db:"message_count,default:0"`
	LinesAdded             int    `json:"lines_added" db:"lines_added,default:0"`
	LinesRemoved           int    `json:"lines_removed" db:"lines_removed,default:0"`
	FilesChanged           int    `json:"files_changed" db:"files_changed,default:0"`
	Archived               bool   `json:"archived" db:"archived,index,default:false"`
	ThreadV                int64  `json:"thread_v" db:"thread_v,default:0"`
	ProviderCreatedMillis  int64  `json:"provider_created_ms" db:"provider_created_ms,default:0"`
	UserLastInteractedAtMS int64  `json:"user_last_interacted_at_ms" db:"user_last_interacted_at_ms,default:0"`
	ProviderUpdatedAt      string `json:"provider_updated_at" db:"provider_updated_at"`
	CreatorUserID          string `json:"creator_user_id" db:"creator_user_id"`
	AgentMode              string `json:"agent_mode" db:"agent_mode"`
	UsesDtw                bool   `json:"uses_dtw" db:"uses_dtw,default:false"`
	UsesThreadActors       bool   `json:"uses_thread_actors" db:"uses_thread_actors,default:false"`
	EnvJSON                string `json:"env_json" db:"env_json"`
	MetaJSON               string `json:"meta_json" db:"meta_json"`
	RelationshipsJSON      string `json:"relationships_json" db:"relationships_json"`
	SummaryStatsJSON       string `json:"summary_stats_json" db:"summary_stats_json"`
	ReasoningEffort        string `json:"reasoning_effort" db:"reasoning_effort"`
	UploadFingerprint      string `json:"-" db:"upload_fingerprint"`
}

var ThreadDB *thing.Thing[*Thread]

func (Thread) TableName() string {
	return "threads"
}

type ThreadMessage struct {
	thing.BaseModel
	ThreadID          int64  `json:"thread_id" db:"thread_id,index,unique:uniq_thread_messages_thread_position"`
	Position          int    `json:"position" db:"position,unique:uniq_thread_messages_thread_position"`
	ProviderMessageID string `json:"provider_message_id" db:"provider_message_id,index"`
	ProtocolMessageID string `json:"protocol_message_id" db:"protocol_message_id"`
	Role              string `json:"role" db:"role"`
	RawZstd           []byte `json:"-" db:"raw_zstd"`     // zstd-compressed raw_json payload (BLOB)
	ContentHash       string `json:"-" db:"content_hash"` // SHA-256 of raw_json; lets the upload diff skip decompression
}

var ThreadMessageDB *thing.Thing[*ThreadMessage]

func (ThreadMessage) TableName() string {
	return "thread_messages"
}

// FreeMessageQuota is the maximum number of stored messages a free user retains.
// It is the single source of truth for the quota ceiling; future plan tiers
// should branch inside MessageQuotaForUser rather than reading this directly.
const FreeMessageQuota = 30000

// MessageQuotaForUser returns the message quota ceiling for the given user.
// A negative value means unlimited, so Pro/Ultra users bypass quota checks.
func MessageQuotaForUser(user *User) int {
	if user != nil && user.UnlimitedStorage {
		return -1
	}
	if PlanLevelForUser(user) != PlanLevelFree {
		return -1
	}
	return FreeMessageQuota
}

// SumThreadMessageCountByUser returns the total stored message_count across all
// of a user's threads. It uses a single aggregate SQL query (filtered by the
// indexed user_id column) so it stays correct beyond any ORM fetch page cap and
// never scans the large thread_messages table. The aggregate is intentionally
// raw SQL because SUM() is outside Thing's cache-invalidation grammar.
func SumThreadMessageCountByUser(userID int64) (int, error) {
	var total *int64
	row := ThreadDB.DB().QueryRowContext(
		context.Background(),
		"SELECT SUM(message_count) FROM threads WHERE user_id = ? AND deleted = ?",
		userID, false,
	)
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	if total == nil {
		return 0, nil
	}
	return int(*total), nil
}
