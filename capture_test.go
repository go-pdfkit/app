package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureDir is where a rendering of the workbench is written so a person can
// look at it.
//
// ⛔ It walks up to the filesystem root looking for a .git and FAILS if it
// finds one. A picture of a user interface is an artefact, not a fixture: one
// written inside a work tree is one `git add -A` from a public repository, and
// a .gitignore is a safety net rather than a barrier -- `git add -f`, a fresh
// clone, or any tool that does not read it publishes the file anyway.
//
// Durable rather than t.TempDir(): a capture removed when the test ends is one
// nobody can open.
func captureDir(t *testing.T) string {
	t.Helper()
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("no durable place to write a capture: %v", err)
	}
	if v := os.Getenv("PDFKIT_CAPTURE_DIR"); v != "" {
		base = v
	}
	dir, err := captureTarget(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("making %s: %v", dir, err)
	}
	return dir
}

// captureTarget is the guard itself, kept pure so it can be tested in both
// directions: a function that only ever fatals can be asserted to fail but
// never asserted to succeed, and a guard that passes for the wrong reason is
// the whole hazard.
func captureTarget(base string) (string, error) {
	abs, err := filepath.Abs(filepath.Join(base, "go-pdfkit", "captures"))
	if err != nil {
		return "", fmt.Errorf("resolving a place to write a capture: %w", err)
	}
	for at := abs; ; {
		if _, err := os.Stat(filepath.Join(at, ".git")); err == nil {
			return "", fmt.Errorf("refusing to write a capture into the work tree at %s: "+
				"a picture of the interface must live outside every repository", at)
		}
		up := filepath.Dir(at)
		if up == at {
			return abs, nil
		}
		at = up
	}
}

func TestCaptureRefusesAWorkTreeAndAcceptsWhatIsOutsideOne(t *testing.T) {
	root := t.TempDir()

	// Inside a work tree: refused, naming the tree.
	tree := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(tree, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := captureTarget(tree)
	if err == nil {
		t.Error("a directory under a .git was accepted")
	} else if !strings.Contains(err.Error(), tree) {
		t.Errorf("the refusal does not name the work tree it found: %v", err)
	}

	// A .git ABOVE the chosen directory is the case a .gitignore would miss,
	// and the one that put a capture of a whole desktop in a public repository.
	deep := filepath.Join(tree, "testdata", "pictures")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := captureTarget(deep); err == nil {
		t.Error("a directory several levels under a .git was accepted")
	}

	// Outside one: accepted, and at the place asked for.
	outside := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := captureTarget(outside)
	if err != nil {
		t.Fatalf("a directory outside every work tree was refused: %v", err)
	}
	if want := filepath.Join(outside, "go-pdfkit", "captures"); got != want {
		t.Errorf("it chose %s, not %s", got, want)
	}
}

// TestCaptureTheWorkbench renders the panels and writes them where a person
// can look. It asserts nothing about what they look like -- that is what eyes
// are for -- only that each one drew something.
func TestCaptureTheWorkbench(t *testing.T) {
	if os.Getenv("PDFKIT_CAPTURE") == "" {
		t.Skip("set PDFKIT_CAPTURE=1 to write a rendering of the workbench")
	}
	dir := captureDir(t)
	s, _ := opened(t, 5)
	for _, name := range append([]string{""}, groupNames...) {
		if name != "" {
			openGroup(t, s, name)
		}
		buf := buffer()
		s.draw(buf)
		img := image.NewRGBA(image.Rect(0, 0, surfaceW, surfaceH))
		for i := 0; i+3 < len(buf); i += 4 {
			p := i / 4
			img.SetRGBA(p%surfaceW, p/surfaceW, color.RGBA{buf[i], buf[i+1], buf[i+2], 255})
		}
		at := filepath.Join(dir, "workbench-"+either(name, "page")+".png")
		f, err := os.Create(at)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		f.Close()
		t.Logf("wrote %s", at)
	}
}

func either(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
