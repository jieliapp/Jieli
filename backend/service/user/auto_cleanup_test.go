package user_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/thread"
	userservice "github.com/jieliapp/jieli/backend/service/user"
)

func TestSetAutoCleanupShouldPersistPreference(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	updated, err := userservice.SetAutoCleanup(alice, true)
	if err != nil {
		t.Fatalf("set auto cleanup: %v", err)
	}
	if !updated.AutoCleanupEnabled {
		t.Fatalf("expected returned user to have auto cleanup enabled, got %#v", updated)
	}
	stored, err := model.UserDB.ByID(alice.ID)
	if err != nil {
		t.Fatalf("load stored user: %v", err)
	}
	if !stored.AutoCleanupEnabled {
		t.Fatalf("expected stored auto cleanup enabled")
	}

	turnedOff, err := userservice.SetAutoCleanup(stored, false)
	if err != nil {
		t.Fatalf("disable auto cleanup: %v", err)
	}
	if turnedOff.AutoCleanupEnabled {
		t.Fatalf("expected auto cleanup disabled after toggle off")
	}
}

func TestGetProfileShouldIncludeQuotaAndAutoCleanupFields(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix", PlanLevel: model.PlanLevelFree, AutoCleanupEnabled: true}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	profile, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.MessageQuota != userservice.MessageQuotaForUser(alice) {
		t.Fatalf("expected message_quota to match quota function, got %d", profile.MessageQuota)
	}
	if profile.MessageQuota <= 0 {
		t.Fatalf("expected positive message quota, got %d", profile.MessageQuota)
	}
	if profile.PlanLevel != model.PlanLevelFree {
		t.Fatalf("expected free plan level, got %q", profile.PlanLevel)
	}
	if !profile.AutoCleanupEnabled {
		t.Fatalf("expected auto_cleanup_enabled true in profile")
	}
}

func TestGetProfileShouldDefaultToFreePlan(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	profile, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.PlanLevel != model.PlanLevelFree {
		t.Fatalf("expected default free plan level, got %q", profile.PlanLevel)
	}
	if profile.MessageQuota != model.FreeMessageQuota {
		t.Fatalf("expected free message quota %d, got %d", model.FreeMessageQuota, profile.MessageQuota)
	}
}

func TestGetProfileShouldKeepExplicitProUnlimitedPlan(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix", PlanLevel: model.PlanLevelPro}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	profile, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.PlanLevel != model.PlanLevelPro {
		t.Fatalf("expected explicit pro plan level, got %q", profile.PlanLevel)
	}
	if profile.MessageQuota != -1 {
		t.Fatalf("expected pro message quota to use unlimited sentinel -1, got %d", profile.MessageQuota)
	}
}

func TestGetProfileStatsShouldDropWhenThreadHardDeleted(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	first, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-a","title":"A","summaryStats":{"diffStats":{"added":10,"deleted":2,"changed":1},"filesChanged":3},"messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`)})
	if err != nil {
		t.Fatalf("upload a: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-b","title":"B","messages":[{"role":"user","content":"one"}]}`)}); err != nil {
		t.Fatalf("upload b: %v", err)
	}

	before, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile before: %v", err)
	}
	if before.Stats.TotalMessages != 3 {
		t.Fatalf("expected lifetime total_messages 3, got %d", before.Stats.TotalMessages)
	}
	if before.Stats.TotalThreads != 2 || before.Stats.LinesAdded != 10 || before.Stats.LinesRemoved != 2 || before.Stats.FilesChanged != 3 {
		t.Fatalf("expected profile stats before delete, got %#v", before.Stats)
	}
	if before.CurrentMessages != 3 {
		t.Fatalf("expected current_messages 3, got %d", before.CurrentMessages)
	}

	if err := thread.DeleteThread(alice, first.ID); err != nil {
		t.Fatalf("delete thread a: %v", err)
	}

	after, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile after: %v", err)
	}
	if after.Stats.TotalMessages != 1 {
		t.Fatalf("expected total_messages to drop to 1 after hard delete, got %d", after.Stats.TotalMessages)
	}
	if after.Stats.TotalThreads != 1 || after.Stats.LinesAdded != 0 || after.Stats.LinesRemoved != 0 || after.Stats.FilesChanged != 0 {
		t.Fatalf("expected profile stats to subtract hard-deleted thread, got %#v", after.Stats)
	}
	if after.CurrentMessages != 1 {
		t.Fatalf("expected current_messages to drop to 1 after delete, got %d", after.CurrentMessages)
	}
}

func TestGetProfileStatsShouldDropHardDeletedThreadAcrossActivityDays(t *testing.T) {
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
		ProviderThreadID: "T-cross-day",
		Title:            "Cross day",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     3,
		LinesAdded:       15,
		LinesRemoved:     3,
		FilesChanged:     2,
		SummaryStatsJSON: `{"diffStats":{"added":15,"deleted":3,"changed":2},"filesChanged":2,"messageCount":3}`,
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("save thread: %v", err)
	}
	for i := 0; i < stored.MessageCount; i++ {
		msg := &model.ThreadMessage{ThreadID: stored.ID, Position: i, Role: "user"}
		msg.SetRawJSON("{}")
		if err := model.ThreadMessageDB.Save(msg); err != nil {
			t.Fatalf("save message %d: %v", i, err)
		}
	}
	rows := []*model.DailyActivity{
		{UserID: alice.ID, Date: "2026-06-01", ThreadCount: 1, MessageCount: 1, LinesAdded: 5, LinesRemoved: 1, LinesChanged: 1, FilesChanged: 1},
		{UserID: alice.ID, Date: "2026-06-02", ThreadCount: 0, MessageCount: 2, LinesAdded: 10, LinesRemoved: 2, LinesChanged: 1, FilesChanged: 1},
	}
	for _, row := range rows {
		if err := model.DailyActivityDB.Save(row); err != nil {
			t.Fatalf("save activity %s: %v", row.Date, err)
		}
	}

	if err := thread.DeleteThread(alice, stored.ID); err != nil {
		t.Fatalf("delete thread: %v", err)
	}
	profile, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.Stats.TotalThreads != 0 || profile.Stats.TotalMessages != 0 || profile.Stats.LinesAdded != 0 || profile.Stats.LinesRemoved != 0 || profile.Stats.FilesChanged != 0 {
		t.Fatalf("expected hard delete to subtract stats across days, got %#v", profile.Stats)
	}
}

func TestGetProfileStatsShouldNotDropWhenThreadSoftDeleted(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	stored, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-a","title":"A","summaryStats":{"diffStats":{"added":10,"deleted":2,"changed":1},"filesChanged":3},"messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`)})
	if err != nil {
		t.Fatalf("upload a: %v", err)
	}
	if err := thread.SoftDeleteThread(alice, stored.ID); err != nil {
		t.Fatalf("soft delete thread a: %v", err)
	}

	profile, err := userservice.GetProfile(alice)
	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if profile.Stats.TotalThreads != 1 || profile.Stats.TotalMessages != 2 || profile.Stats.LinesAdded != 10 || profile.Stats.LinesRemoved != 2 || profile.Stats.FilesChanged != 3 {
		t.Fatalf("expected soft delete to keep profile stats, got %#v", profile.Stats)
	}
	if profile.CurrentMessages != 0 {
		t.Fatalf("expected current_messages to exclude soft-deleted thread, got %d", profile.CurrentMessages)
	}
}
