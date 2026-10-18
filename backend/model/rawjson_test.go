package model_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jieliapp/jieli/backend/model"
)

func TestRawJSONShouldRoundTripByteForByte(t *testing.T) {
	cases := map[string]string{
		"empty":   "",
		"simple":  `{"role":"user","content":"hello"}`,
		"unicode": `{"role":"user","content":"héllo 世界 🚀 \"quoted\""}`,
		"large":   `{"role":"assistant","content":"` + strings.Repeat("a", 200000) + `"}`,
		"tool":    `{"role":"tool","content":[{"type":"tool_result","tool_use_id":"t-1","content":"ok"}],"usage":{"totalInputTokens":42}}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			message := &model.ThreadMessage{}
			message.SetRawJSON(payload)
			if payload != "" && len(message.RawZstd) == 0 {
				t.Fatalf("expected non-empty blob for payload %q", name)
			}
			got, err := message.RawJSON()
			if err != nil {
				t.Fatalf("decode raw json: %v", err)
			}
			if got != payload {
				t.Fatalf("round-trip mismatch for %q", name)
			}
		})
	}
}

func TestSetRawJSONShouldRecordContentHash(t *testing.T) {
	t.Run("populates hash alongside blob", func(t *testing.T) {
		payload := `{"role":"user","content":"hello"}`
		message := &model.ThreadMessage{}
		message.SetRawJSON(payload)
		if len(message.RawZstd) == 0 {
			t.Fatalf("expected non-empty blob")
		}
		if message.ContentHash != model.HashRawJSON(payload) {
			t.Fatalf("expected content_hash %q, got %q", model.HashRawJSON(payload), message.ContentHash)
		}
	})
	t.Run("empty payload still yields a non-empty hash", func(t *testing.T) {
		message := &model.ThreadMessage{}
		message.SetRawJSON("")
		if len(message.RawZstd) != 0 {
			t.Fatalf("expected empty blob for empty payload")
		}
		if message.ContentHash == "" {
			t.Fatalf("expected non-empty content_hash for empty payload")
		}
	})
	t.Run("different payloads produce different hashes", func(t *testing.T) {
		a := &model.ThreadMessage{}
		a.SetRawJSON(`{"a":1}`)
		b := &model.ThreadMessage{}
		b.SetRawJSON(`{"a":2}`)
		if a.ContentHash == b.ContentHash {
			t.Fatalf("expected distinct content hashes for distinct payloads")
		}
	})
}

func TestRawJSONShouldRoundTripThroughDatabaseAndCache(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jieli.db")
	t.Cleanup(closeModelDB(t))
	if err := model.InitSQLite(dbPath); err != nil {
		t.Fatalf("init sqlite: %v", err)
	}

	payload := `{"role":"assistant","content":"persisted 世界 🚀"}`
	message := &model.ThreadMessage{ThreadID: 1, Position: 0, Role: "assistant"}
	message.SetRawJSON(payload)
	if err := model.ThreadMessageDB.Save(message); err != nil {
		t.Fatalf("save message: %v", err)
	}

	// First ByID read (DB) and a second ByID read (cache-hit path) must both
	// decompress to the original payload.
	for i := 0; i < 2; i++ {
		loaded, err := model.ThreadMessageDB.ByID(message.ID)
		if err != nil {
			t.Fatalf("load message (pass %d): %v", i, err)
		}
		got, err := loaded.RawJSON()
		if err != nil {
			t.Fatalf("decode raw json (pass %d): %v", i, err)
		}
		if got != payload {
			t.Fatalf("expected payload round-trip (pass %d), got %q", i, got)
		}
	}
}
