package podkop

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var compatMark = regexp.MustCompile(`//\s*compat:\s*([a-z0-9-]+)`)

// Every item kept for podkop 0.7 is marked where it is done, and every mark names an item
// of the list: removing one is a search for its name.
func TestCompatMarks(t *testing.T) {
	marked := make(map[string][]string)
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == "compat.go" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, found := range compatMark.FindAllStringSubmatch(string(content), -1) {
			marked[found[1]] = append(marked[found[1]], path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	listed := make(map[string]bool)
	for _, item := range compat {
		listed[item.Name] = true
		if len(marked[item.Name]) == 0 {
			t.Errorf("%s is listed and marked nowhere", item.Name)
		}
		if item.Why == "" || item.Until == "" {
			t.Errorf("%s does not say why or until when", item.Name)
		}
	}
	for name, paths := range marked {
		if !listed[name] {
			t.Errorf("%s is marked in %v and not listed in compat.go", name, paths)
		}
	}
}
