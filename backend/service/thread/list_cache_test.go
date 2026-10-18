package thread

import (
	"database/sql"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
	"github.com/burugo/thing"
)

func TestThreadMessagePayloadCacheEvictsLeastRecentlyUsedEntryWithinByteBudget(t *testing.T) {
	db := &sql.DB{}
	cache := newThreadMessagePayloadCache(8)
	first := &model.Thread{BaseModel: thing.BaseModel{ID: 1}, MessageCount: 201, ThreadV: 1}
	second := &model.Thread{BaseModel: thing.BaseModel{ID: 2}, MessageCount: 201, ThreadV: 1}
	third := &model.Thread{BaseModel: thing.BaseModel{ID: 3}, MessageCount: 201, ThreadV: 1}

	cache.store(db, first, threadMessagePayload{Messages: []byte("1111")})
	cache.store(db, second, threadMessagePayload{Messages: []byte("2222")})
	if _, ok := cache.load(db, first); !ok {
		t.Fatal("expected first payload to be cached")
	}
	cache.store(db, third, threadMessagePayload{Messages: []byte("3333")})

	if _, ok := cache.load(db, first); !ok {
		t.Fatal("expected recently used first payload to remain cached")
	}
	if _, ok := cache.load(db, second); ok {
		t.Fatal("expected least recently used second payload to be evicted")
	}
	if _, ok := cache.load(db, third); !ok {
		t.Fatal("expected third payload to be cached")
	}
	if cache.usedBytes > 8 {
		t.Fatalf("expected cache to stay within 8-byte budget, got %d", cache.usedBytes)
	}
}

func TestThreadMessagePayloadCacheSkipsPayloadLargerThanByteBudget(t *testing.T) {
	db := &sql.DB{}
	cache := newThreadMessagePayloadCache(4)
	stored := &model.Thread{BaseModel: thing.BaseModel{ID: 1}, MessageCount: 201, ThreadV: 1}

	cache.store(db, stored, threadMessagePayload{Messages: []byte("12345")})

	if _, ok := cache.load(db, stored); ok {
		t.Fatal("expected payload larger than the complete budget not to be cached")
	}
	if cache.usedBytes != 0 {
		t.Fatalf("expected oversized payload to retain zero bytes, got %d", cache.usedBytes)
	}
}
