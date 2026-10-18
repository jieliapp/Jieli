package model

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/burugo/thing"
)

type User struct {
	thing.BaseModel
	LoginEmail         string `json:"-" db:"login_email,index,default:''"`
	PasswordHash       string `json:"-" db:"password_hash,default:''"`
	IsAdmin            bool   `json:"is_admin" db:"is_admin,default:false"`
	MustChangePassword bool   `json:"must_change_password" db:"must_change_password,default:false"`
	UnlimitedStorage   bool   `json:"-" db:"-"`
	Username           string `json:"username" db:"username,index"`
	DisplayName        string `json:"display_name" db:"display_name"`
	Email              string `json:"email" db:"email,index"`
	EmailManaged       bool   `json:"-" db:"email_managed,default:false"`
	AvatarURL          string `json:"avatar_url" db:"avatar_url"`
	GitHubID           string `json:"github_id" db:"github_id"`
	GoogleID           string `json:"google_id" db:"google_id"`
	APIKeyHash         string `json:"-" db:"api_key_hash,unique"`
	APIKeyPrefix       string `json:"api_key_prefix" db:"api_key_prefix,index"`
	APIKeyEncrypted    string `json:"-" db:"api_key_encrypted,default:''"`
	PreferredLocale    string `json:"preferred_locale" db:"preferred_locale"`
	PlanLevel          string `json:"plan_level" db:"plan_level,index,default:'free'"`
	UsedAgentsJSON     string `json:"-" db:"used_agents,default:''"`
	UsageInfoJSON      string `json:"-" db:"usage_info,default:''"`
	AutoCleanupEnabled bool   `json:"auto_cleanup_enabled" db:"auto_cleanup_enabled,default:false"`
}

var UserDB *thing.Thing[*User]

const (
	PlanLevelFree  = "free"
	PlanLevelPro   = "pro"
	PlanLevelUltra = "ultra"
)

func PlanLevelForUser(user *User) string {
	if user == nil {
		return PlanLevelFree
	}
	switch strings.TrimSpace(strings.ToLower(user.PlanLevel)) {
	case PlanLevelPro:
		return PlanLevelPro
	case PlanLevelUltra:
		return PlanLevelUltra
	default:
		return PlanLevelFree
	}
}

type UserUsageInfo struct {
	LastUploadError *UserUsageUploadError `json:"last_upload_error,omitempty"`
}

type UserUsageUploadError struct {
	Code                   string `json:"code"`
	At                     string `json:"at"`
	Provider               string `json:"provider"`
	ProviderThreadID       string `json:"provider_thread_id"`
	CurrentMessages        int    `json:"current_messages"`
	ExistingThreadMessages int    `json:"existing_thread_messages"`
	IncomingMessages       int    `json:"incoming_messages"`
	ProjectedMessages      int    `json:"projected_messages"`
	Quota                  int    `json:"quota"`
}

func (User) TableName() string {
	return "users"
}

func (u *User) UsageInfo() UserUsageInfo {
	if u == nil || strings.TrimSpace(u.UsageInfoJSON) == "" {
		return UserUsageInfo{}
	}
	var info UserUsageInfo
	if err := json.Unmarshal([]byte(u.UsageInfoJSON), &info); err != nil {
		return UserUsageInfo{}
	}
	return info
}

func (u *User) SetUsageInfo(info UserUsageInfo) error {
	if u == nil {
		return errors.New("user is required")
	}
	if info.LastUploadError == nil {
		u.UsageInfoJSON = ""
		return nil
	}
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}
	u.UsageInfoJSON = string(raw)
	return nil
}

func IsSupportedProvider(provider string) bool {
	for _, info := range SupportedProviders() {
		if info.ID == provider {
			return true
		}
	}
	return false
}

func (u *User) UsedAgents() []string {
	if u == nil || strings.TrimSpace(u.UsedAgentsJSON) == "" {
		return nil
	}
	var agents []string
	if err := json.Unmarshal([]byte(u.UsedAgentsJSON), &agents); err != nil {
		return nil
	}
	return orderedSupportedProviders(agents)
}

func (u *User) SetUsedAgents(agents []string) error {
	ordered := orderedSupportedProviders(agents)
	if len(ordered) == 0 {
		u.UsedAgentsJSON = ""
		return nil
	}
	raw, err := json.Marshal(ordered)
	if err != nil {
		return err
	}
	u.UsedAgentsJSON = string(raw)
	return nil
}

// UpdateUser changes only fields owned by update, using a fresh transactional
// record so a stale request cannot restore credentials or other account state.
// The callback must not open another transaction.
func UpdateUser(ctx context.Context, id int64, update func(*User) error) (*User, error) {
	var user *User
	err := thing.Transaction(ctx, func(tx *thing.TxSession) error {
		users, err := thing.UseTx[*User](tx)
		if err != nil {
			return err
		}
		user, err = users.ByID(id)
		if err != nil {
			return err
		}
		if err = update(user); err != nil {
			return err
		}
		return users.Save(user)
	})
	return user, err
}

func RecordUsedAgent(user *User, provider string) (bool, error) {
	if user == nil || user.ID == 0 {
		return false, errors.New("user is required")
	}
	if !IsSupportedProvider(provider) {
		return false, errors.New("unsupported provider")
	}
	added := false
	latest, err := UpdateUser(context.Background(), user.ID, func(current *User) error {
		agents := current.UsedAgents()
		for _, agent := range agents {
			if agent == provider {
				return nil
			}
		}
		added = true
		return current.SetUsedAgents(append(agents, provider))
	})
	if err != nil {
		return false, err
	}
	user.UsedAgentsJSON = latest.UsedAgentsJSON
	return added, nil
}

func UsedProviderInfos(user *User) []ProviderInfo {
	agents := user.UsedAgents()
	if len(agents) == 0 {
		return nil
	}
	agentSet := make(map[string]struct{}, len(agents))
	for _, agent := range agents {
		agentSet[agent] = struct{}{}
	}
	infos := make([]ProviderInfo, 0, len(agents))
	for _, info := range SupportedProviders() {
		if _, ok := agentSet[info.ID]; ok {
			infos = append(infos, info)
		}
	}
	return infos
}

func orderedSupportedProviders(providers []string) []string {
	if len(providers) == 0 {
		return nil
	}
	providerSet := make(map[string]struct{}, len(providers))
	for _, provider := range providers {
		provider = strings.TrimSpace(provider)
		if provider != "" {
			providerSet[provider] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(providerSet))
	for _, info := range SupportedProviders() {
		if _, ok := providerSet[info.ID]; ok {
			ordered = append(ordered, info.ID)
		}
	}
	return ordered
}
