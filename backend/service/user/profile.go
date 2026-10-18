package user

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jieliapp/jieli/backend/model"
	subscriptionservice "github.com/jieliapp/jieli/backend/service/subscription"
)

var ErrInvalidUsername = errors.New("invalid username")
var ErrUsernameTaken = errors.New("username taken")

const (
	usernameMinLength = 3
	usernameMaxLength = 32
	usernameSuffixLen = 6
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9]+$`)

var defaultReservedUsernames = []string{
	"jieli",
	"burugo",
	"admin",
	"root",
	"system",
	"support",
	"api",
	"web",
	"cli",
	"auth",
	"login",
	"logout",
	"settings",
	"profile",
	"account",
	"users",
	"me",
	"threads",
	"share",
	"attachments",
	"static",
	"assets",
	"docs",
	"status",
	"help",
	"test",
	"demo",
}

type ProfileUser struct {
	Inactive           bool   `json:"inactive"`
	IsAdmin            bool   `json:"is_admin"`
	MustChangePassword bool   `json:"must_change_password"`
	ID                 int64  `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"display_name"`
	Email              string `json:"email"`
	AvatarURL          string `json:"avatar_url"`
	GitHubID           string `json:"github_id"`
	PreferredLocale    string `json:"preferred_locale"`
}

type ProfileStats struct {
	TotalMessages int `json:"total_messages"`
	TotalThreads  int `json:"total_threads"`
	LinesAdded    int `json:"lines_added"`
	LinesRemoved  int `json:"lines_removed"`
	FilesChanged  int `json:"files_changed"`
}

type Profile struct {
	User               ProfileUser         `json:"user"`
	Stats              ProfileStats        `json:"stats"`
	PlanLevel          string              `json:"plan_level"`
	Subscription       ProfileSubscription `json:"subscription"`
	MessageQuota       int                 `json:"message_quota"`
	CurrentMessages    int                 `json:"current_messages"`
	AutoCleanupEnabled bool                `json:"auto_cleanup_enabled"`
	UsageInfo          model.UserUsageInfo `json:"usage_info"`
}

type ProfileSubscription = subscriptionservice.Summary

type PublicProfileUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

type PublicProfile struct {
	User  PublicProfileUser `json:"user"`
	Stats ProfileStats      `json:"stats"`
}

type ProfileUpdate struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type Activity struct {
	Date         string `json:"date"`
	ThreadCount  int    `json:"thread_count"`
	MessageCount int    `json:"message_count"`
	LinesAdded   int    `json:"lines_added"`
	LinesRemoved int    `json:"lines_removed"`
	LinesChanged int    `json:"lines_changed"`
}

func GetProfile(user *model.User) (Profile, error) {
	if user == nil || user.ID == 0 {
		return Profile{}, errors.New("user is required")
	}
	stats, err := profileStatsForUser(user)
	if err != nil {
		return Profile{}, err
	}
	current, err := model.SumThreadMessageCountByUser(user.ID)
	if err != nil {
		return Profile{}, err
	}
	subscription, err := subscriptionservice.Current(user.ID, time.Now().UTC())
	if err != nil {
		return Profile{}, err
	}
	planLevel := effectiveProfilePlanLevel(user, subscription)
	quotaUser := *user
	quotaUser.PlanLevel = planLevel
	return Profile{
		User:               SafeUser(user),
		Stats:              stats,
		PlanLevel:          planLevel,
		Subscription:       subscription,
		MessageQuota:       MessageQuotaForUser(&quotaUser),
		CurrentMessages:    current,
		AutoCleanupEnabled: user.AutoCleanupEnabled,
		UsageInfo:          user.UsageInfo(),
	}, nil
}

func effectiveProfilePlanLevel(user *model.User, subscription ProfileSubscription) string {
	if subscription.Status == model.SubscriptionStatusActive {
		switch strings.TrimSpace(strings.ToLower(subscription.PlanLevel)) {
		case model.PlanLevelPro:
			return model.PlanLevelPro
		case model.PlanLevelUltra:
			return model.PlanLevelUltra
		}
	}
	return model.PlanLevelForUser(user)
}

func SafeUser(user *model.User) ProfileUser {
	if user == nil {
		return ProfileUser{}
	}
	return ProfileUser{
		Inactive:           user.Deleted,
		IsAdmin:            user.IsAdmin,
		MustChangePassword: user.MustChangePassword,
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		Email:              user.Email,
		AvatarURL:          user.AvatarURL,
		GitHubID:           user.GitHubID,
		PreferredLocale:    user.PreferredLocale,
	}
}

func PublicSafeUser(user *model.User) PublicProfileUser {
	if user == nil {
		return PublicProfileUser{}
	}
	return PublicProfileUser{
		Username:    user.Username,
		DisplayName: user.DisplayName,
		AvatarURL:   user.AvatarURL,
	}
}

func GetPublicProfile(username string) (PublicProfile, error) {
	user, err := findUserByUsername(username)
	if err != nil {
		return PublicProfile{}, err
	}
	stats, err := profileStatsForUser(user)
	if err != nil {
		return PublicProfile{}, err
	}
	return PublicProfile{User: PublicSafeUser(user), Stats: stats}, nil
}

func ListPublicActivity(username string) ([]Activity, error) {
	user, err := findUserByUsername(username)
	if err != nil {
		return nil, err
	}
	return ListActivity(user)
}

// ValidateUsername canonicalizes a new username and applies profile naming rules.
// Availability must be checked by the caller in its write transaction.
func ValidateUsername(raw string) (string, error) {
	username := normalizeUsername(raw)
	if reservedUsername(username) {
		return "", ErrUsernameTaken
	}
	if !validUsername(username) {
		return "", ErrInvalidUsername
	}
	return username, nil
}

func UpdateProfile(user *model.User, update ProfileUpdate) (ProfileUser, error) {
	if user == nil || user.ID == 0 {
		return ProfileUser{}, errors.New("user is required")
	}
	username, err := ValidateUsername(update.Username)
	if err != nil {
		return ProfileUser{}, err
	}
	if err := ensureUsernameAvailable(user.ID, username); err != nil {
		return ProfileUser{}, err
	}
	updated, err := model.UpdateUser(context.Background(), user.ID, func(current *model.User) error {
		current.Username = username
		current.DisplayName = strings.TrimSpace(update.DisplayName)
		return nil
	})
	if err != nil {
		return ProfileUser{}, err
	}
	return SafeUser(updated), nil
}

func UpdateAvatarURL(user *model.User, avatarURL string) (ProfileUser, error) {
	if user == nil || user.ID == 0 {
		return ProfileUser{}, errors.New("user is required")
	}
	updated, err := model.UpdateUser(context.Background(), user.ID, func(current *model.User) error {
		current.AvatarURL = strings.TrimSpace(avatarURL)
		return nil
	})
	if err != nil {
		return ProfileUser{}, err
	}
	return SafeUser(updated), nil
}

func ListActivity(user *model.User) ([]Activity, error) {
	if user == nil || user.ID == 0 {
		return nil, errors.New("user is required")
	}
	rows, err := model.DailyActivityDB.Where("user_id = ?", user.ID).Order("date ASC").Fetch(0, 1000)
	if err != nil {
		return nil, err
	}
	activity := make([]Activity, 0, len(rows))
	for _, row := range rows {
		activity = append(activity, Activity{
			Date:         row.Date,
			ThreadCount:  row.ThreadCount,
			MessageCount: row.MessageCount,
			LinesAdded:   row.LinesAdded,
			LinesRemoved: row.LinesRemoved,
			LinesChanged: row.LinesChanged,
		})
	}
	return activity, nil
}

func UpdateLocale(user *model.User, locale string) (*model.User, error) {
	if user == nil || user.ID == 0 {
		return nil, errors.New("user is required")
	}
	if locale != "en" && locale != "zh" {
		return nil, errors.New("unsupported locale")
	}
	return model.UpdateUser(context.Background(), user.ID, func(current *model.User) error {
		current.PreferredLocale = locale
		return nil
	})
}

func profileStatsForUser(user *model.User) (ProfileStats, error) {
	if user == nil || user.ID == 0 {
		return ProfileStats{}, errors.New("user is required")
	}
	// Profile counters come from the daily_activity ledger. Auto-cleanup only
	// hides threads and must not reduce these totals; explicit user deletion
	// adjusts the ledger before hard-deleting the thread.
	rows, err := model.DailyActivityDB.Where("user_id = ?", user.ID).All()
	if err != nil {
		return ProfileStats{}, err
	}
	stats := ProfileStats{}
	for _, row := range rows {
		stats.TotalThreads += row.ThreadCount
		stats.TotalMessages += row.MessageCount
		stats.LinesAdded += row.LinesAdded
		stats.LinesRemoved += row.LinesRemoved
		stats.FilesChanged += row.FilesChanged
	}
	return stats, nil
}

func findUserByUsername(username string) (*model.User, error) {
	username = strings.TrimSpace(username)
	users, err := model.UserDB.Where("username = ?", username).Fetch(0, 1)
	if err != nil {
		return nil, err
	}
	if len(users) != 1 {
		return nil, errors.New("user not found")
	}
	return users[0], nil
}

func ensureUsernameAvailable(currentUserID int64, username string) error {
	users, err := model.UserDB.Where("username = ?", normalizeUsername(username)).WithDeleted().Fetch(0, 2)
	if err != nil {
		return err
	}
	for _, existing := range users {
		if existing.ID != currentUserID {
			return ErrUsernameTaken
		}
	}
	return nil
}

func GenerateAvailableUsername(base string, stableID string) (string, error) {
	base = sanitizeUsernameBase(base)
	if base == "" {
		base = "user"
	}
	if validUsername(base) {
		if err := ensureUsernameAvailable(0, base); err == nil {
			return base, nil
		} else if !errors.Is(err, ErrUsernameTaken) {
			return "", err
		}
	}

	suffix := usernameSuffix(stableID)
	if suffix == "" {
		suffix = usernameSuffix(base)
	}
	prefixLen := usernameMaxLength - len(suffix)
	if prefixLen < usernameMinLength {
		prefixLen = usernameMinLength
	}
	prefix := base
	if len(prefix) > prefixLen {
		prefix = prefix[:prefixLen]
	}
	if prefix == "" {
		prefix = "user"
	}
	candidate := prefix + suffix
	if len(candidate) > usernameMaxLength {
		candidate = candidate[:usernameMaxLength]
	}
	if len(candidate) < usernameMinLength {
		candidate = "user" + suffix
		if len(candidate) > usernameMaxLength {
			candidate = candidate[:usernameMaxLength]
		}
	}
	if validUsername(candidate) {
		if err := ensureUsernameAvailable(0, candidate); err == nil {
			return candidate, nil
		} else if !errors.Is(err, ErrUsernameTaken) {
			return "", err
		}
	}

	for suffixLen := usernameSuffixLen + 2; suffixLen <= 16; suffixLen += 2 {
		suffix = usernameSuffixWithLength(stableID, suffixLen)
		prefixLen = usernameMaxLength - len(suffix)
		prefix = base
		if prefix == "" {
			prefix = "user"
		}
		if len(prefix) > prefixLen {
			prefix = prefix[:prefixLen]
		}
		candidate = prefix + suffix
		if validUsername(candidate) {
			if err := ensureUsernameAvailable(0, candidate); err == nil {
				return candidate, nil
			} else if !errors.Is(err, ErrUsernameTaken) {
				return "", err
			}
		}
	}
	return "", ErrUsernameTaken
}

func validUsername(username string) bool {
	return len(username) >= usernameMinLength &&
		len(username) <= usernameMaxLength &&
		usernamePattern.MatchString(username) &&
		!reservedUsername(username)
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func reservedUsername(username string) bool {
	username = normalizeUsername(username)
	for _, reserved := range defaultReservedUsernames {
		if username == reserved {
			return true
		}
	}
	for _, reserved := range strings.Split(strings.ToLower(strings.TrimSpace(os.Getenv("USERNAME_RESERVED_WORDS"))), ",") {
		if username == strings.TrimSpace(reserved) {
			return true
		}
	}
	return false
}

func sanitizeUsernameBase(base string) string {
	base = normalizeUsername(base)
	var builder strings.Builder
	for _, char := range base {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func usernameSuffix(value string) string {
	return usernameSuffixWithLength(value, usernameSuffixLen)
}

func usernameSuffixWithLength(value string, length int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	sum := sha1.Sum([]byte(value))
	encoded := hex.EncodeToString(sum[:])
	if length > len(encoded) {
		length = len(encoded)
	}
	return encoded[:length]
}
