package user_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/thread"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

func TestGetProfileShouldReturnSafeUserFieldsAndStats(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", Email: "alice@example.com", AvatarURL: "https://example.com/avatar.png", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix", PreferredLocale: "zh"}
	bob := &model.User{Username: "bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix","messages":[{"role":"user","content":"one"}]}`)}); err != nil {
		t.Fatalf("upload alice alpha: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-beta","title":"Billing","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`)}); err != nil {
		t.Fatalf("upload alice beta: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(bob, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"Bob oauth","messages":[{"role":"user","content":"secret"}]}`)}); err != nil {
		t.Fatalf("upload bob alpha: %v", err)
	}

	profile, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.User.ID != alice.ID || profile.User.Username != "alice" || profile.User.DisplayName != "Alice Doe" || profile.User.Email != "alice@example.com" || profile.User.AvatarURL != "https://example.com/avatar.png" || profile.User.PreferredLocale != "zh" {
		t.Fatalf("expected safe user fields, got %#v", profile)
	}
	if profile.Stats.TotalThreads != 2 || profile.Stats.TotalMessages != 3 {
		t.Fatalf("expected alice-only stats total_threads=2 total_messages=3, got %#v", profile)
	}
}

func TestGetProfileShouldReturnCurrentSubscription(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour).Truncate(time.Millisecond)
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", PlanLevel: model.PlanLevelPro}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.SubscriptionDB.Save(&model.Subscription{
		UserID:                 alice.ID,
		PlanLevel:              model.PlanLevelPro,
		Status:                 model.SubscriptionStatusActive,
		StartsAtMS:             now.Add(-time.Hour).UnixMilli(),
		EndsAtMS:               expiresAt.UnixMilli(),
		Source:                 model.SubscriptionSourceNeo,
		ExternalSubscriptionID: "501",
		ExternalGroupID:        10,
	}); err != nil {
		t.Fatalf("save subscription: %v", err)
	}

	profile, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.Subscription.Status != model.SubscriptionStatusActive || profile.Subscription.PlanLevel != model.PlanLevelPro {
		t.Fatalf("expected active pro subscription, got %#v", profile.Subscription)
	}
	if profile.Subscription.ExpiresAt == nil || !profile.Subscription.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("expected subscription expiry %s, got %#v", expiresAt, profile.Subscription.ExpiresAt)
	}
}

func TestGetProfileShouldUseActiveSubscriptionForEffectivePlan(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	now := time.Now().UTC()
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", PlanLevel: model.PlanLevelFree}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.SubscriptionDB.Save(&model.Subscription{
		UserID:                 alice.ID,
		PlanLevel:              model.PlanLevelPro,
		Status:                 model.SubscriptionStatusActive,
		StartsAtMS:             now.Add(-time.Hour).UnixMilli(),
		EndsAtMS:               now.Add(time.Hour).UnixMilli(),
		Source:                 model.SubscriptionSourceNeo,
		ExternalSubscriptionID: "501",
		ExternalGroupID:        10,
	}); err != nil {
		t.Fatalf("save subscription: %v", err)
	}

	profile, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.PlanLevel != model.PlanLevelPro {
		t.Fatalf("expected active subscription to make profile pro, got %q", profile.PlanLevel)
	}
	if profile.MessageQuota != -1 {
		t.Fatalf("expected active subscription to use unlimited quota, got %d", profile.MessageQuota)
	}
}

func TestGetPublicProfileShouldReturnPublicSafeFieldsAndStats(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", Email: "alice@example.com", AvatarURL: "https://example.com/avatar.png", GitHubID: "github-alice", APIKeyHash: "alice-hash"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-alpha","title":"OAuth fix","messages":[{"role":"user","content":"one"}]}`)}); err != nil {
		t.Fatalf("upload alice alpha: %v", err)
	}

	profile, err := userservice.GetPublicProfile("alice")
	if err != nil {
		t.Fatalf("get public profile: %v", err)
	}
	if profile.User.Username != "alice" || profile.User.DisplayName != "Alice Doe" || profile.User.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("expected public user fields, got %#v", profile.User)
	}
	if profile.Stats.TotalThreads != 1 || profile.Stats.TotalMessages != 1 {
		t.Fatalf("expected public stats, got %#v", profile.Stats)
	}
}

func TestListPublicActivityShouldReturnNamedUsersDailyActivity(t *testing.T) {
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
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: alice.ID, Date: "2026-05-29", ThreadCount: 2, MessageCount: 5}); err != nil {
		t.Fatalf("save alice activity: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: bob.ID, Date: "2026-05-30", ThreadCount: 9, MessageCount: 99}); err != nil {
		t.Fatalf("save bob activity: %v", err)
	}

	activity, err := userservice.ListPublicActivity("alice")
	if err != nil {
		t.Fatalf("list public activity: %v", err)
	}
	if len(activity) != 1 || activity[0].Date != "2026-05-29" || activity[0].MessageCount != 5 {
		t.Fatalf("expected alice activity only, got %#v", activity)
	}
}

func TestListActivityShouldReturnCurrentUsersDailyActivity(t *testing.T) {
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
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: alice.ID, Date: "2026-05-29", ThreadCount: 2, MessageCount: 5, LinesAdded: 100, LinesRemoved: 20, LinesChanged: 7}); err != nil {
		t.Fatalf("save alice first activity: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: alice.ID, Date: "2026-05-30", ThreadCount: 1, MessageCount: 3, LinesAdded: 200, LinesRemoved: 30, LinesChanged: 9}); err != nil {
		t.Fatalf("save alice second activity: %v", err)
	}
	if err := model.DailyActivityDB.Save(&model.DailyActivity{UserID: bob.ID, Date: "2026-05-30", ThreadCount: 9, MessageCount: 99, LinesAdded: 900, LinesRemoved: 90, LinesChanged: 99}); err != nil {
		t.Fatalf("save bob activity: %v", err)
	}

	activity, err := userservice.ListActivity(alice)
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}
	if len(activity) != 2 {
		t.Fatalf("expected two alice activity rows, got %#v", activity)
	}
	if activity[0].Date != "2026-05-29" || activity[0].ThreadCount != 2 || activity[0].MessageCount != 5 || activity[0].LinesAdded != 100 || activity[0].LinesRemoved != 20 || activity[0].LinesChanged != 7 {
		t.Fatalf("expected first alice activity row, got %#v", activity[0])
	}
	if activity[1].Date != "2026-05-30" || activity[1].ThreadCount != 1 || activity[1].MessageCount != 3 || activity[1].LinesAdded != 200 || activity[1].LinesRemoved != 30 || activity[1].LinesChanged != 9 {
		t.Fatalf("expected second alice activity row, got %#v", activity[1])
	}
}

func TestUpdateLocaleShouldPersistSupportedLocaleOnly(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix", PreferredLocale: "en"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	updated, err := userservice.UpdateLocale(alice, "zh")
	if err != nil {
		t.Fatalf("update locale: %v", err)
	}
	if updated.PreferredLocale != "zh" {
		t.Fatalf("expected zh locale, got %#v", updated)
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.PreferredLocale != "zh" {
		t.Fatalf("expected stored zh locale, got %q", stored.PreferredLocale)
	}
	if _, err := userservice.UpdateLocale(alice, "fr"); err == nil {
		t.Fatalf("expected unsupported locale to fail")
	}
}

func TestUpdateProfileShouldLowercaseUsername(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice Doe", GitHubID: "github-alice"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	updated, err := userservice.UpdateProfile(alice, userservice.ProfileUpdate{Username: "Alice2", DisplayName: "Alice Dev"})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.Username != "alice2" || updated.DisplayName != "Alice Dev" {
		t.Fatalf("expected updated safe user fields, got %#v", updated)
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if stored.Username != "alice2" || stored.DisplayName != "Alice Dev" {
		t.Fatalf("expected stored profile fields, got %#v", stored)
	}
}

func TestUpdateProfileShouldRejectTakenUsername(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", DisplayName: "Alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	bob := &model.User{Username: "bob", DisplayName: "Bob", GitHubID: "github-bob", APIKeyHash: "bob-hash", APIKeyPrefix: "bob-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(bob); err != nil {
		t.Fatalf("save bob: %v", err)
	}

	_, err := userservice.UpdateProfile(bob, userservice.ProfileUpdate{Username: "alice", DisplayName: "Bob Updated"})
	if !errors.Is(err, userservice.ErrUsernameTaken) {
		t.Fatalf("expected username taken error, got %v", err)
	}
	stored, err := model.UserDB.ByID(bob.ID)
	if err != nil {
		t.Fatalf("load stored bob: %v", err)
	}
	if stored.Username != "bob" || stored.DisplayName != "Bob" {
		t.Fatalf("expected bob profile unchanged, got %#v", stored)
	}
}

func TestUpdateProfileShouldRejectLowercasedTakenUsername(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	tom := &model.User{Username: "tom", GitHubID: "github-tom", APIKeyHash: "tom-hash", APIKeyPrefix: "tom-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	if err := model.UserDB.Save(tom); err != nil {
		t.Fatalf("save tom: %v", err)
	}

	_, err := userservice.UpdateProfile(alice, userservice.ProfileUpdate{Username: "Tom", DisplayName: "Alice"})
	if !errors.Is(err, userservice.ErrUsernameTaken) {
		t.Fatalf("expected username taken error for lowercased collision, got %v", err)
	}
}

func TestUpdateProfileShouldRejectInvalidUsername(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	_, err := userservice.UpdateProfile(alice, userservice.ProfileUpdate{Username: "bad name", DisplayName: "Alice"})
	if !errors.Is(err, userservice.ErrInvalidUsername) {
		t.Fatalf("expected invalid username error, got %v", err)
	}
}

func TestUpdateProfileShouldRejectShortUsername(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	_, err := userservice.UpdateProfile(alice, userservice.ProfileUpdate{Username: "ab", DisplayName: "Alice"})
	if !errors.Is(err, userservice.ErrInvalidUsername) {
		t.Fatalf("expected invalid username error, got %v", err)
	}
}

func TestUpdateProfileShouldTreatReservedUsernameAsTaken(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	_, err := userservice.UpdateProfile(alice, userservice.ProfileUpdate{Username: "cli", DisplayName: "Alice"})
	if !errors.Is(err, userservice.ErrUsernameTaken) {
		t.Fatalf("expected username taken error, got %v", err)
	}

	_, err = userservice.UpdateProfile(alice, userservice.ProfileUpdate{Username: "burugo", DisplayName: "Alice"})
	if !errors.Is(err, userservice.ErrUsernameTaken) {
		t.Fatalf("expected username taken error, got %v", err)
	}
}

func TestUpdateProfileShouldTreatEnvironmentReservedUsernameAsTaken(t *testing.T) {
	t.Setenv("USERNAME_RESERVED_WORDS", "internal, Launch")
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	_, err := userservice.UpdateProfile(alice, userservice.ProfileUpdate{Username: "launch", DisplayName: "Alice"})
	if !errors.Is(err, userservice.ErrUsernameTaken) {
		t.Fatalf("expected username taken error, got %v", err)
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
