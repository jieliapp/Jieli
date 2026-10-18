package thread

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestThreadListWhereSearchShouldAvoidSQLFunctions(t *testing.T) {
	where, args, err := threadListWhere(42, ListOptions{Search: "Task"})
	if err != nil {
		t.Fatalf("build thread list where: %v", err)
	}
	if strings.Contains(where, "LOWER(") {
		t.Fatalf("expected search where without SQL functions, got %q", where)
	}
	if !strings.Contains(where, "search_text LIKE ?") {
		t.Fatalf("expected search_text LIKE filter, got %q", where)
	}
	if len(args) != 2 || args[1] != "%task%" {
		t.Fatalf("expected lowercase search pattern arg, got %#v", args)
	}
}

func TestPreviewShouldNotSplitUTF8Runes(t *testing.T) {
	text := strings.Repeat("a", 159) + "当后面还有内容"
	result := preview(text)
	if !utf8.ValidString(result) {
		t.Fatalf("expected valid utf8 preview, got %q", result)
	}
	if strings.ContainsRune(result, utf8.RuneError) {
		t.Fatalf("expected preview without replacement rune, got %q", result)
	}
	if got := []rune(result); len(got) != 160 || got[159] != '当' {
		t.Fatalf("expected preview to keep the full 160th rune, got %q", result)
	}
}
