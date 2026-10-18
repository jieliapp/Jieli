package thread

import (
	"encoding/json"
	"testing"
)

func TestEditFileInputsShouldParseSnakeCaseInput(t *testing.T) {
	edits := editFileInputs(json.RawMessage(`[{"type":"tool_use","name":"edit_file","input":{"path":"/repo/a.go","old_str":"one\ntwo\nthree","new_str":"one\ntwo changed\nthree\nfour"}}]`))
	if len(edits) != 1 {
		t.Fatalf("expected one edit, got %#v", edits)
	}
	if edits[0].Path != "/repo/a.go" || edits[0].OldStr == "" || edits[0].NewStr == "" {
		t.Fatalf("expected edit_file path and old/new strings, got %#v", edits[0])
	}
}

func TestLineDiffStatsShouldSeparateAddedDeletedAndChangedLines(t *testing.T) {
	added, deleted, changed := lineDiffStats("one\ntwo\nthree", "one\ntwo changed\nthree\nfour")
	if added != 1 || deleted != 0 || changed != 1 {
		t.Fatalf("expected +1 -0 ~1, got +%d -%d ~%d", added, deleted, changed)
	}
}
