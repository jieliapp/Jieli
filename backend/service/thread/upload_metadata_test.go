package thread

import (
	"path/filepath"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/burugo/thing"
	"github.com/burugo/thing/thingtest"
)

func TestLoadThreadMessageMetadataBypassesThingCacheAndOmitsRawPayload(t *testing.T) {
	t.Cleanup(func() { _ = model.CloseDB() })
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	storedThread := &model.Thread{
		UserID:           1,
		Provider:         model.ProviderAmp,
		ProviderThreadID: "T-metadata-only",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     1,
	}
	if err := model.ThreadDB.Save(storedThread); err != nil {
		t.Fatalf("save thread: %v", err)
	}
	storedMessage := &model.ThreadMessage{
		ThreadID:          storedThread.ID,
		Position:          0,
		ProviderMessageID: "m-1",
		ProtocolMessageID: "p-1",
		Role:              "user",
	}
	storedMessage.SetRawJSON(`{"role":"user","content":"large payload must not be selected"}`)
	if err := model.ThreadMessageDB.Save(storedMessage); err != nil {
		t.Fatalf("save message: %v", err)
	}
	cache, ok := thing.Cache().(*thingtest.MockCache)
	if !ok {
		t.Fatalf("expected mock Thing cache in tests, got %T", thing.Cache())
	}
	cache.ResetCounters()

	metadata, err := loadThreadMessageMetadata(storedThread.ID)
	if err != nil {
		t.Fatalf("load message metadata: %v", err)
	}
	if len(metadata) != 1 {
		t.Fatalf("expected one metadata row, got %#v", metadata)
	}
	row := metadata[0]
	if row.ID != storedMessage.ID || row.Position != 0 || row.ProviderMessageID != "m-1" || row.ProtocolMessageID != "p-1" || row.Role != "user" || row.ContentHash != storedMessage.ContentHash {
		t.Fatalf("unexpected metadata row %#v", row)
	}
	for _, operation := range []string{"GetModel", "SetModel", "GetQueryIDs", "SetQueryIDs"} {
		if count := cache.CounterSnapshot()[operation]; count != 0 {
			t.Fatalf("expected metadata read to bypass Thing cache operation %s, got %d", operation, count)
		}
	}
}

func TestReplaceMessagesDoesNotRetainRawPayloadInThingCache(t *testing.T) {
	t.Cleanup(func() { _ = model.CloseDB() })
	if err := model.InitSQLite(filepath.Join(t.TempDir(), "jieli.db")); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	storedThread := &model.Thread{
		UserID:           1,
		Provider:         model.ProviderAmp,
		ProviderThreadID: "T-raw-cache",
		Visibility:       model.ThreadVisibilityPrivate,
		MessageCount:     1,
	}
	if err := model.ThreadDB.Save(storedThread); err != nil {
		t.Fatalf("save thread: %v", err)
	}

	if err := replaceMessages(1, storedThread.ID, []storedMessage{{
		Role: "user",
		Raw:  []byte(`{"role":"user","content":"do not retain this raw payload"}`),
	}}); err != nil {
		t.Fatalf("replace messages: %v", err)
	}
	var messageID int64
	if err := model.ThreadMessageDB.DB().QueryRow("SELECT id FROM thread_messages WHERE thread_id = ?", storedThread.ID).Scan(&messageID); err != nil {
		t.Fatalf("query stored message id: %v", err)
	}
	cache, ok := thing.Cache().(*thingtest.MockCache)
	if !ok {
		t.Fatalf("expected mock Thing cache in tests, got %T", thing.Cache())
	}
	cacheKey := "thread_messages:" + cache.FormatID(messageID)
	if cache.Exists(cacheKey) {
		t.Fatalf("expected raw message model not to remain in Thing cache key %q", cacheKey)
	}
}
