package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestOverTheCorpus runs the verb against real documents and says what it
// would drop. Skipped unless a corpus is named: no scan of anybody's document
// enters the repository, and the test suite must pass on a machine that has
// none.
func TestOverTheCorpus(t *testing.T) {
	dir := os.Getenv("BLANKCORPUS")
	if dir == "" {
		t.Skip("no BLANKCORPUS")
	}
	docs, pages, dropped, allBlank := 0, 0, 0, 0
	filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || filepath.Ext(path) != ".pdf" || docs >= 200 {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		h := &fakeHost{name: filepath.Base(path), file: raw}
		s := newState(surfaceW, surfaceH, h)
		s.open()
		if s.doc == nil {
			return nil
		}
		docs++
		before := s.doc.PageCount()
		pages += before
		s.dropBlank()
		after := s.doc.PageCount()
		dropped += before - after
		if before == after && strings.Contains(s.note, "every page of this document is blank") {
			allBlank++
			t.Logf("  tout blanc: %s (%d pages)", filepath.Base(path), before)
		}
		return nil
	})
	t.Logf("%d documents, %d pages, %d dropped, %d refused as all-blank",
		docs, pages, dropped, allBlank)
}
