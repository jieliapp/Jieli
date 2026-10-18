/** Shared API types derived from the backend spec. */

export type Provider =
  | 'amp'
  | 'claude_code'
  | 'codex'
  | 'opencode'
  | 'pi'
  | 'gemini'
  | 'github_copilot'
  | 'antigravity'
  | 'qwen_code'
  | 'cursor'
  | 'windsurf'
  | 'droid';
export type Visibility = 'private' | 'unlisted';

export interface ProviderInfo {
  id: Provider;
  label: string;
}

export interface User {
  inactive?: boolean;
  is_admin?: boolean;
  must_change_password?: boolean;
  id: number;
  username: string;
  display_name?: string;
  email?: string;
  avatar_url?: string;
  github_id: string;
  preferred_locale?: string;
}

export interface ThreadLabel {
  label: string;
}

export interface ThreadSummary {
  id: number;
  provider: Provider;
  provider_thread_id: string;
  title: string;
  visibility: Visibility;
  share_slug?: string;
  repo?: string;
  repo_url?: string;
  branch?: string;
  source_url?: string;
  preview?: string;
  working_directory?: string;
  message_count: number;
  lines_added: number;
  lines_removed: number;
  lines_changed?: number;
  files_changed: number;
  labels: string[];
  archived: boolean;
  created_at: string;
  updated_at: string;
}

export interface TextBlock {
  type: 'text';
  text: string;
}

export interface ThinkingBlock {
  type: 'thinking';
  thinking?: string;
  text?: string;
}

export interface ToolUseBlock {
  type: 'tool_use';
  id: string;
  name?: string;
  input?: unknown;
  complete?: boolean;
}

export interface ToolResultBlock {
  type: 'tool_result';
  toolUseID?: string;
  tool_use_id?: string;
  content?: string | ContentBlock[];
  is_error?: boolean;
  run?: {
    result?: string | ContentBlock[] | { output?: string; exitCode?: number; content?: unknown };
    status?: string;
  };
}

export interface ImageBlock {
  type: 'image';
  source?: { url?: string; type?: string };
  sourcePath?: string;
}

export type ContentBlock =
  | TextBlock
  | ThinkingBlock
  | ToolUseBlock
  | ToolResultBlock
  | ImageBlock
  | { type: string; [key: string]: unknown };

export interface ThreadMessage {
  role: 'user' | 'assistant' | 'tool' | string;
  content: string | ContentBlock[];
  [key: string]: unknown;
}

export interface ThreadOwner {
  username: string;
  display_name?: string;
  avatar_url?: string;
}

export interface ThreadDetail extends ThreadSummary {
  data_json?: unknown;
  owner?: ThreadOwner;
  creator_user_id?: string;
  reasoning_effort?: string;
  summary_stats?: Record<string, unknown>;
  messages: ThreadMessage[];
  mode?: string;
  model?: string;
  workspace_root?: string;
  working_directory?: string;
  context_percent?: number;
  context_window_tokens?: number;
  cost_display?: string;
}

export interface ThreadListResult {
  threads: ThreadSummary[];
  total: number;
  page: number;
  page_size: number;
}

export interface ThreadListFilters {
  search?: string;
  provider?: string;
  repo?: string;
  label?: string;
  visibility?: string;
  archived?: boolean;
  include_archived?: boolean;
  sort?: 'created' | 'updated';
  page?: number;
  page_size?: number;
}

export interface ThreadFacets {
  providers: ProviderInfo[];
  repos: string[];
  labels: string[];
}

export interface ProfileStats {
  total_messages: number;
  total_threads: number;
  lines_added: number;
  lines_removed: number;
  files_changed: number;
}

export interface LastUploadError {
  code: string;
  at?: string;
  provider?: string;
  provider_thread_id?: string;
  current_messages?: number;
  existing_thread_messages?: number;
  incoming_messages?: number;
  projected_messages?: number;
  quota?: number;
}

export interface UsageInfo {
  last_upload_error?: LastUploadError | null;
}

export interface SubscriptionSummary {
  status: 'none' | 'active' | 'expired' | 'cancelled' | string;
  plan_level: 'free' | 'pro' | 'ultra' | string;
  starts_at?: string;
  expires_at?: string;
  source?: string;
}

export interface Profile {
  user: User;
  stats: ProfileStats;
  plan_level: 'free' | 'pro' | 'ultra' | string;
  subscription?: SubscriptionSummary;
  message_quota: number;
  current_messages: number;
  auto_cleanup_enabled: boolean;
  usage_info?: UsageInfo;
}

export interface SubscriptionSyncResult {
  subscription: SubscriptionSummary;
  profile: Profile;
}

export interface PublicUser {
  username: string;
  display_name?: string;
  avatar_url?: string;
}

export interface PublicProfile {
  user: PublicUser;
  stats: ProfileStats;
}

export interface ProfileUpdatePayload {
  username: string;
  display_name: string;
}

export interface AvatarUploadPayload {
  data: string;
  mediaType: string;
}

export interface ActivityDay {
  date: string;
  thread_count: number;
  message_count: number;
  lines_added: number;
  lines_removed: number;
  lines_changed: number;
}

export interface ApiKeyInfo {
  prefix: string;
  created_at?: string;
  has_key: boolean;
  api_key?: string;
}

export interface ApiKeyRotateResult {
  api_key: string;
  prefix: string;
}
