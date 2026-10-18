package thread_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/thread"
)

func seedThread(t *testing.T, userID int64, providerThreadID string, messageCount int, lastInteractedMS int64) *model.Thread {
	t.Helper()
	stored := &model.Thread{
		UserID:                 userID,
		Provider:               model.ProviderAmp,
		ProviderThreadID:       providerThreadID,
		Visibility:             model.ThreadVisibilityPrivate,
		MessageCount:           messageCount,
		UserLastInteractedAtMS: lastInteractedMS,
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("seed thread %s: %v", providerThreadID, err)
	}
	for i := 0; i < messageCount; i++ {
		msg := &model.ThreadMessage{ThreadID: stored.ID, Position: i, Role: "user"}
		msg.SetRawJSON("{}")
		if err := model.ThreadMessageDB.Save(msg); err != nil {
			t.Fatalf("seed message %d for %s: %v", i, providerThreadID, err)
		}
	}
	return stored
}

func threadExists(t *testing.T, id int64) bool {
	t.Helper()
	rows, err := model.ThreadDB.Where("id = ?", id).Fetch(0, 1)
	if err != nil {
		t.Fatalf("fetch active thread %d: %v", id, err)
	}
	return len(rows) == 1
}

func TestMaybeAutoCleanupShouldDeleteOldestThreadsDownToBuffer(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix", PlanLevel: model.PlanLevelFree, AutoCleanupEnabled: true}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	// Quota 30000, 90% = 27000. Seed 28000 across 28 threads of 1000 each,
	// oldest first by UserLastInteractedAtMS. Target delete = 30% of 30000 = 9000,
	// so 9 oldest threads (9000 messages) get removed, leaving 19000 (~63%).
	threads := make([]*model.Thread, 0, 28)
	for i := 0; i < 28; i++ {
		threads = append(threads, seedThread(t, alice.ID, providerID(i), 1000, int64(i+1)*1000))
	}

	if err := thread.MaybeAutoCleanup(alice); err != nil {
		t.Fatalf("maybe auto cleanup: %v", err)
	}

	total, err := model.SumThreadMessageCountByUser(alice.ID)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if total != 19000 {
		t.Fatalf("expected 19000 messages remaining after deleting oldest 9000, got %d", total)
	}
	// The 9 oldest threads must be gone; the 10th oldest and newer must survive.
	for i := 0; i < 9; i++ {
		if threadExists(t, threads[i].ID) {
			t.Fatalf("expected oldest thread index %d (last_interacted=%d) deleted", i, threads[i].UserLastInteractedAtMS)
		}
	}
	for i := 9; i < 28; i++ {
		if !threadExists(t, threads[i].ID) {
			t.Fatalf("expected thread index %d to survive", i)
		}
	}
}

func TestMaybeAutoCleanupShouldNotDeleteWhenDisabled(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix", PlanLevel: model.PlanLevelFree, AutoCleanupEnabled: false}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	for i := 0; i < 28; i++ {
		seedThread(t, alice.ID, providerID(i), 1000, int64(i+1)*1000)
	}

	if err := thread.MaybeAutoCleanup(alice); err != nil {
		t.Fatalf("maybe auto cleanup: %v", err)
	}

	total, err := model.SumThreadMessageCountByUser(alice.ID)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if total != 28000 {
		t.Fatalf("expected no deletion when disabled, got %d", total)
	}
}

func TestUploadShouldTriggerAutoCleanupWhenOverThreshold(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix", PlanLevel: model.PlanLevelFree, AutoCleanupEnabled: true}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	// Seed 27000 messages (exactly 90%) across 27 oldest threads, then upload
	// one more thread via the public path. finishProviderUpload should trigger
	// cleanup and drop usage toward the 70% buffer.
	for i := 0; i < 27; i++ {
		seedThread(t, alice.ID, providerID(i), 1000, int64(i+1)*1000)
	}

	if _, err := thread.UploadThreadSnapshot(alice, thread.UploadPayload{Provider: model.ProviderCodex, Thread: jsonPayload("T-new", 1)}); err != nil {
		t.Fatalf("upload new: %v", err)
	}

	total, err := model.SumThreadMessageCountByUser(alice.ID)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if total >= 27000 {
		t.Fatalf("expected auto-cleanup to reduce usage below 27000, got %d", total)
	}
}

func TestMaybeAutoCleanupShouldNotDeleteBelowThreshold(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix", PlanLevel: model.PlanLevelFree, AutoCleanupEnabled: true}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	// 26000 messages = 86.7%, below the 90% trigger.
	for i := 0; i < 26; i++ {
		seedThread(t, alice.ID, providerID(i), 1000, int64(i+1)*1000)
	}

	if err := thread.MaybeAutoCleanup(alice); err != nil {
		t.Fatalf("maybe auto cleanup: %v", err)
	}

	total, err := model.SumThreadMessageCountByUser(alice.ID)
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if total != 26000 {
		t.Fatalf("expected no deletion below threshold, got %d", total)
	}
}

func providerID(i int) string {
	return "T-" + itoa(i)
}

// jsonPayload builds a minimal amp thread snapshot with messageCount messages.
func jsonPayload(id string, messageCount int) json.RawMessage {
	msgs := make([]string, 0, messageCount)
	for i := 0; i < messageCount; i++ {
		msgs = append(msgs, `{"role":"user","content":"x"}`)
	}
	return json.RawMessage(`{"id":"` + id + `","title":"t","messages":[` + strings.Join(msgs, ",") + `]}`)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
