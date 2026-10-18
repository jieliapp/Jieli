package frontend

import (
	"io/fs"
	"strings"
	"testing"
)

func TestDistShouldIncludeSvelteKitAppAssets(t *testing.T) {
	dist := Dist()
	foundAppAsset := false
	err := fs.WalkDir(dist, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasPrefix(path, "_app/immutable/entry/") && strings.HasSuffix(path, ".js") {
			foundAppAsset = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded dist: %v", err)
	}
	if !foundAppAsset {
		t.Fatalf("expected embedded dist to include SvelteKit _app entry assets")
	}
}
