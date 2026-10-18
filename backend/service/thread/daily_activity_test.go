package thread_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/thread"
)

func dailyActivityForUser(t *testing.T, userID int64) *model.DailyActivity {
	t.Helper()
	rows, err := model.DailyActivityDB.Where("user_id = ?", userID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch daily activity: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly one daily activity row for today, got %d", len(rows))
	}
	return rows[0]
}

func TestUploadShouldAccumulateDailyActivityAcrossThreads(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-a","title":"A","messages":[{"role":"user","content":"one"}]}`)}); err != nil {
		t.Fatalf("upload a: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-b","title":"B","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`)}); err != nil {
		t.Fatalf("upload b: %v", err)
	}

	activity := dailyActivityForUser(t, alice.ID)
	if activity.ThreadCount != 2 {
		t.Fatalf("expected thread_count to accumulate to 2, got %d", activity.ThreadCount)
	}
	if activity.MessageCount != 3 {
		t.Fatalf("expected message_count to accumulate to 3 (1+2), got %d", activity.MessageCount)
	}
}

func TestUploadShouldAccumulateOnlyNetNewMessagesOnResend(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	// First upload: 2 messages.
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-a","title":"A","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`)}); err != nil {
		t.Fatalf("upload first: %v", err)
	}
	// Resend same thread grown to 3 messages: only +1 net-new should accumulate.
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-a","title":"A","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"user","content":"three"}]}`)}); err != nil {
		t.Fatalf("upload resend: %v", err)
	}

	activity := dailyActivityForUser(t, alice.ID)
	if activity.MessageCount != 3 {
		t.Fatalf("expected message_count 3 (2 + 1 net-new), got %d", activity.MessageCount)
	}
	if activity.ThreadCount != 1 {
		t.Fatalf("expected thread_count 1 (same thread updated), got %d", activity.ThreadCount)
	}
}

func TestUploadEmptyPlaceholderShouldNotCountAndReactivationShouldCount(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	// Upload with zero messages (StartSession hook, no API key quota).
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-empty","title":"Empty","messages":[]}`)}); err != nil {
		t.Fatalf("upload empty: %v", err)
	}

	// No daily activity row should exist yet.
	rows, err := model.DailyActivityDB.Where("user_id = ?", alice.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch daily activity: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no daily activity for empty placeholder, got %d rows (thread_count=%d)", len(rows), rows[0].ThreadCount)
	}

	// Thread should be soft-deleted and not appear in listing.
	listed, err := thread.ListThreads(alice, thread.ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if listed.Total != 0 {
		t.Fatalf("expected 0 listed threads after empty upload, got %d", listed.Total)
	}

	// Re-upload same thread with messages (user actually started chatting).
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-empty","title":"Now Active","messages":[{"role":"user","content":"hello"},{"role":"assistant","content":"hi"}]}`)}); err != nil {
		t.Fatalf("upload with messages: %v", err)
	}

	// Daily activity should now show thread_count=1, message_count=2.
	activity := dailyActivityForUser(t, alice.ID)
	if activity.ThreadCount != 1 {
		t.Fatalf("expected thread_count 1 after reactivation, got %d", activity.ThreadCount)
	}
	if activity.MessageCount != 2 {
		t.Fatalf("expected message_count 2 after reactivation, got %d", activity.MessageCount)
	}

	// Thread should now be visible in listing.
	listed, err = thread.ListThreads(alice, thread.ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list threads after reactivation: %v", err)
	}
	if listed.Total != 1 {
		t.Fatalf("expected 1 listed thread after reactivation, got %d", listed.Total)
	}
}

func TestUploadSoftDeletedThreadShouldReactivateWithoutDuplicatingDailyActivity(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-reactivate","title":"A","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`)}); err != nil {
		t.Fatalf("upload first: %v", err)
	}
	activity := dailyActivityForUser(t, alice.ID)
	if activity.ThreadCount != 1 || activity.MessageCount != 2 {
		t.Fatalf("expected initial activity thread_count=1 message_count=2, got thread_count=%d message_count=%d", activity.ThreadCount, activity.MessageCount)
	}

	listed, err := thread.ListThreads(alice, thread.ListOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list threads: %v", err)
	}
	if listed.Total != 1 {
		t.Fatalf("expected thread to be listed before delete, got %d", listed.Total)
	}
	if err := thread.DeleteThread(alice, listed.Threads[0].ID); err != nil {
		t.Fatalf("delete thread: %v", err)
	}
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-reactivate","title":"A","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"}]}`)}); err != nil {
		t.Fatalf("upload reactivated: %v", err)
	}

	activity = dailyActivityForUser(t, alice.ID)
	if activity.ThreadCount != 1 {
		t.Fatalf("expected reactivation not to increment thread_count, got %d", activity.ThreadCount)
	}
	if activity.MessageCount != 2 {
		t.Fatalf("expected reactivation not to duplicate message_count, got %d", activity.MessageCount)
	}
}

func TestUploadShouldNotReduceDailyActivityOnRewind(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}

	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-a","title":"A","messages":[{"role":"user","content":"one"},{"role":"assistant","content":"two"},{"role":"user","content":"three"}]}`)}); err != nil {
		t.Fatalf("upload first: %v", err)
	}
	// Rewind to 1 message: accumulated total must not drop.
	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: json.RawMessage(`{"id":"T-a","title":"A","messages":[{"role":"user","content":"one"}]}`)}); err != nil {
		t.Fatalf("upload rewind: %v", err)
	}

	activity := dailyActivityForUser(t, alice.ID)
	if activity.MessageCount != 3 {
		t.Fatalf("expected message_count to stay at peak 3 after rewind, got %d", activity.MessageCount)
	}
}
