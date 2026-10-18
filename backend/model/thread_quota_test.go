package model_test

import (
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
)

func TestSumThreadMessageCountByUserShouldCountBeyondLegacyFetchCap(t *testing.T) {
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
	const aliceThreads = 1100
	for i := 0; i < aliceThreads; i++ {
		stored := &model.Thread{
			UserID:           alice.ID,
			Provider:         model.ProviderAmp,
			ProviderThreadID: providerThreadIDForIndex(i),
			Visibility:       model.ThreadVisibilityPrivate,
			MessageCount:     2,
		}
		if err := model.ThreadDB.Save(stored); err != nil {
			t.Fatalf("save alice thread %d: %v", i, err)
		}
	}
	// Bob's threads must not leak into alice's total.
	if err := model.ThreadDB.Save(&model.Thread{UserID: bob.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-bob", Visibility: model.ThreadVisibilityPrivate, MessageCount: 999}); err != nil {
		t.Fatalf("save bob thread: %v", err)
	}

	total, err := model.SumThreadMessageCountByUser(alice.ID)
	if err != nil {
		t.Fatalf("sum thread message count: %v", err)
	}
	if total != aliceThreads*2 {
		t.Fatalf("expected total %d across all threads, got %d", aliceThreads*2, total)
	}
}

func TestSumThreadMessageCountByUserShouldExcludeDeletedThreads(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	alice := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "alice-hash", APIKeyPrefix: "alice-prefix"}
	if err := model.UserDB.Save(alice); err != nil {
		t.Fatalf("save alice: %v", err)
	}
	keep := &model.Thread{UserID: alice.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-keep", Visibility: model.ThreadVisibilityPrivate, MessageCount: 5}
	drop := &model.Thread{UserID: alice.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-drop", Visibility: model.ThreadVisibilityPrivate, MessageCount: 7}
	if err := model.ThreadDB.Save(keep); err != nil {
		t.Fatalf("save keep: %v", err)
	}
	if err := model.ThreadDB.Save(drop); err != nil {
		t.Fatalf("save drop: %v", err)
	}
	if err := model.ThreadDB.Delete(drop); err != nil {
		t.Fatalf("delete drop: %v", err)
	}

	total, err := model.SumThreadMessageCountByUser(alice.ID)
	if err != nil {
		t.Fatalf("sum thread message count: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected only surviving thread counted (5), got %d", total)
	}
}

func providerThreadIDForIndex(i int) string {
	return "T-alice-" + itoa(i)
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
