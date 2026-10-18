package thread_test

import (
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/jieliapp/jieli/backend/service/thread"
)

func TestDeleteThreadShouldHardDeleteThreadAndMessages(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	stored := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-delete", Title: "delete me", Visibility: model.ThreadVisibilityPrivate}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("save thread: %v", err)
	}
	const messageCount = 10005
	for i := 0; i < messageCount; i++ {
		msg := &model.ThreadMessage{ThreadID: stored.ID, Position: i, Role: "user"}
		msg.SetRawJSON("{}")
		if err := model.ThreadMessageDB.Save(msg); err != nil {
			t.Fatalf("save message %d: %v", i, err)
		}
	}

	if err := thread.DeleteThread(user, stored.ID); err != nil {
		t.Fatalf("delete thread: %v", err)
	}

	messages, err := model.ThreadMessageDB.Where("thread_id = ?", stored.ID).Fetch(0, 20000)
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("expected messages to be hard-deleted, got %d", len(messages))
	}
	if _, err := thread.GetThreadDetail(user, stored.ID); err == nil {
		t.Fatalf("expected thread detail to hide deleted thread")
	}
	deleted, err := model.ThreadDB.Where("id = ?", stored.ID).WithDeleted().Fetch(0, 1)
	if err != nil {
		t.Fatalf("fetch deleted thread: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected thread row to be hard-deleted, got %#v", deleted)
	}
}

func TestDeleteThreadShouldInvalidateCachedThreadList(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	stored := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-cache-delete", Title: "cache delete", Visibility: model.ThreadVisibilityPrivate}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("save thread: %v", err)
	}

	warmed, err := model.ThreadDB.Where("user_id = ?", user.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("warm thread list cache: %v", err)
	}
	if len(warmed) != 1 || warmed[0].ID != stored.ID {
		t.Fatalf("expected one warmed thread, got %#v", warmed)
	}

	if err := thread.DeleteThread(user, stored.ID); err != nil {
		t.Fatalf("delete thread: %v", err)
	}

	afterDelete, err := model.ThreadDB.Where("user_id = ?", user.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch thread list after delete: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Fatalf("expected cached thread list to be invalidated, got %#v", afterDelete)
	}
}

func TestDeleteThreadShouldReclaimActivityTotalsFromNewestRows(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	stored := &model.Thread{
		UserID:           user.ID,
		Provider:         model.ProviderAmp,
		ProviderThreadID: "T-activity-reclaim",
		Title:            "activity reclaim",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     8,
		LinesAdded:       12,
	}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("save thread: %v", err)
	}
	for _, row := range []*model.DailyActivity{
		{UserID: user.ID, Date: "2026-06-01", ThreadCount: 1, MessageCount: 5, LinesAdded: 5},
		{UserID: user.ID, Date: "2026-06-02", ThreadCount: 1, MessageCount: 5, LinesAdded: 10},
	} {
		if err := model.DailyActivityDB.Save(row); err != nil {
			t.Fatalf("save activity %s: %v", row.Date, err)
		}
	}

	if err := thread.DeleteThread(user, stored.ID); err != nil {
		t.Fatalf("delete thread: %v", err)
	}

	rows, err := model.DailyActivityDB.Where("user_id = ?", user.ID).Order("date ASC").Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch activity: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected two activity rows, got %#v", rows)
	}
	if rows[0].Date != "2026-06-01" || rows[0].ThreadCount != 1 || rows[0].MessageCount != 2 || rows[0].LinesAdded != 3 {
		t.Fatalf("expected older row to receive only remaining aggregate reclaim, got %#v", rows[0])
	}
	if rows[1].Date != "2026-06-02" || rows[1].ThreadCount != 0 || rows[1].MessageCount != 0 || rows[1].LinesAdded != 0 {
		t.Fatalf("expected newest row to be reclaimed first, got %#v", rows[1])
	}
}

func TestRestoreDeletedThreadsShouldOnlyRestoreSoftDeletedThreads(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	softDeleted := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-soft", Title: "soft", Visibility: model.ThreadVisibilityPrivate, MessageCount: 1}
	hardDeleted := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-hard", Title: "hard", Visibility: model.ThreadVisibilityPrivate, MessageCount: 1}
	emptyDeleted := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-empty", Title: "empty", Visibility: model.ThreadVisibilityPrivate}
	for _, stored := range []*model.Thread{softDeleted, hardDeleted, emptyDeleted} {
		if err := model.ThreadDB.Save(stored); err != nil {
			t.Fatalf("save thread %s: %v", stored.ProviderThreadID, err)
		}
	}
	if err := thread.SoftDeleteThread(user, softDeleted.ID); err != nil {
		t.Fatalf("soft delete thread: %v", err)
	}
	if err := thread.SoftDeleteThread(user, emptyDeleted.ID); err != nil {
		t.Fatalf("soft delete empty thread: %v", err)
	}
	if err := thread.DeleteThread(user, hardDeleted.ID); err != nil {
		t.Fatalf("hard delete thread: %v", err)
	}

	restored, err := thread.RestoreDeletedThreads(user)
	if err != nil {
		t.Fatalf("restore deleted threads: %v", err)
	}
	if restored != 1 {
		t.Fatalf("expected one restored thread, got %d", restored)
	}
	if _, err := model.ThreadDB.ByID(softDeleted.ID); err != nil {
		t.Fatalf("expected soft-deleted thread restored: %v", err)
	}
	if rows, err := model.ThreadDB.Where("id = ?", hardDeleted.ID).WithDeleted().Fetch(0, 1); err != nil || len(rows) != 0 {
		t.Fatalf("expected hard-deleted thread to stay gone, rows=%#v err=%v", rows, err)
	}
	if rows, err := model.ThreadDB.Where("id = ?", emptyDeleted.ID).WithDeleted().Fetch(0, 1); err != nil || len(rows) != 1 || !rows[0].Deleted {
		t.Fatalf("expected empty deleted placeholder to stay hidden, rows=%#v err=%v", rows, err)
	}
}

func TestDeleteThreadShouldRemoveAttachmentLinksButKeepAttachmentObject(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	stored := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-delete-attachment", Title: "delete attachment", Visibility: model.ThreadVisibilityPrivate}
	if err := model.ThreadDB.Save(stored); err != nil {
		t.Fatalf("save thread: %v", err)
	}
	attachment := &model.Attachment{
		UserID:        user.ID,
		PublicID:      "att-delete",
		MediaType:     "image/png",
		Extension:     "png",
		StorageDriver: "r2",
		StorageKey:    "att-delete.png",
		SizeBytes:     123,
	}
	if err := model.AttachmentDB.Save(attachment); err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	if err := model.ThreadAttachmentDB.Save(&model.ThreadAttachment{
		UserID:        user.ID,
		ThreadID:      stored.ID,
		AttachmentID:  attachment.ID,
		PublicID:      attachment.PublicID,
		StorageDriver: attachment.StorageDriver,
		StorageKey:    attachment.StorageKey,
	}); err != nil {
		t.Fatalf("save thread attachment: %v", err)
	}

	if err := thread.DeleteThread(user, stored.ID); err != nil {
		t.Fatalf("delete thread: %v", err)
	}

	links, err := model.ThreadAttachmentDB.Where("thread_id = ?", stored.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch thread attachments: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected thread attachment links to be deleted, got %#v", links)
	}
	attachments, err := model.AttachmentDB.Where("public_id = ?", attachment.PublicID).WithDeleted().Fetch(0, 1)
	if err != nil {
		t.Fatalf("fetch attachment with deleted: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected attachment row to remain, got %d rows", len(attachments))
	}
	if attachments[0].Deleted {
		t.Fatalf("expected attachment to stay active")
	}
}

func TestDeleteThreadShouldKeepSharedReferencedAttachmentsActive(t *testing.T) {
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	user := &model.User{Username: "alice", GitHubID: "github-alice", APIKeyHash: "hash", APIKeyPrefix: "prefix"}
	if err := model.UserDB.Save(user); err != nil {
		t.Fatalf("save user: %v", err)
	}
	first := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-delete-shared-a", Title: "shared a", Visibility: model.ThreadVisibilityPrivate}
	second := &model.Thread{UserID: user.ID, Provider: model.ProviderAmp, ProviderThreadID: "T-delete-shared-b", Title: "shared b", Visibility: model.ThreadVisibilityPrivate}
	if err := model.ThreadDB.Save(first); err != nil {
		t.Fatalf("save first thread: %v", err)
	}
	if err := model.ThreadDB.Save(second); err != nil {
		t.Fatalf("save second thread: %v", err)
	}
	attachment := &model.Attachment{
		UserID:        user.ID,
		PublicID:      "att-shared",
		MediaType:     "image/png",
		Extension:     "png",
		StorageDriver: "r2",
		StorageKey:    "att-shared.png",
		SizeBytes:     123,
	}
	if err := model.AttachmentDB.Save(attachment); err != nil {
		t.Fatalf("save attachment: %v", err)
	}
	for _, stored := range []*model.Thread{first, second} {
		if err := model.ThreadAttachmentDB.Save(&model.ThreadAttachment{
			UserID:        user.ID,
			ThreadID:      stored.ID,
			AttachmentID:  attachment.ID,
			PublicID:      attachment.PublicID,
			StorageDriver: attachment.StorageDriver,
			StorageKey:    attachment.StorageKey,
		}); err != nil {
			t.Fatalf("save thread attachment: %v", err)
		}
	}

	if err := thread.DeleteThread(user, first.ID); err != nil {
		t.Fatalf("delete first thread: %v", err)
	}

	if _, err := model.AttachmentDB.ByID(attachment.ID); err != nil {
		t.Fatalf("expected shared attachment to remain active: %v", err)
	}
	links, err := model.ThreadAttachmentDB.Where("attachment_id = ?", attachment.ID).Fetch(0, 10)
	if err != nil {
		t.Fatalf("fetch remaining links: %v", err)
	}
	if len(links) != 1 || links[0].ThreadID != second.ID {
		t.Fatalf("expected only surviving thread link to remain, got %#v", links)
	}
}
