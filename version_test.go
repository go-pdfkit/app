package main

import (
	"runtime/debug"
	"strings"
	"testing"
)

// stampedWith makes the toolchain's answer the test's, and puts the real one
// back afterwards.
func stampedWith(t *testing.T, info *debug.BuildInfo, ok bool) {
	t.Helper()
	was := buildInfo
	t.Cleanup(func() { buildInfo = was })
	buildInfo = func() (*debug.BuildInfo, bool) { return info, ok }
}

// linkedAs is a -X flag arriving the way the deploy workflow sends one.
func linkedAs(t *testing.T, v string) {
	t.Helper()
	was := release
	t.Cleanup(func() { release = was })
	release = v
}

func TestTheVersionIsTheOneLinkedIn(t *testing.T) {
	linkedAs(t, "v0.19.0-2-gdeadbee")
	// Stamped differently on purpose: the link-time string wins, because it
	// is the one that names a RELEASE and the commit alone does not.
	stampedWith(t, &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef"},
	}}, true)
	if got := appVersion(); got != "v0.19.0-2-gdeadbee" {
		t.Errorf("the workbench says it is %q", got)
	}
}

func TestWithoutALinkedVersionItNamesItsCommit(t *testing.T) {
	linkedAs(t, "")
	stampedWith(t, &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.time", Value: "2026-11-05T10:00:00Z"},
		{Key: "vcs.revision", Value: "0123456789abcdef"},
	}}, true)
	if got := appVersion(); got != "0123456" {
		t.Errorf("a build with no version linked in says it is %q", got)
	}
}

// ⛔ Three ways of having nothing to say, and all three say the SAME thing
// rather than a number. A wrong version is worse than no version: it would
// answer the distribution question confidently and wrongly, which is the one
// failure this whole file exists to stop.
func TestWithNothingToGoOnItSaysSo(t *testing.T) {
	for _, c := range []struct {
		name string
		info *debug.BuildInfo
		ok   bool
	}{
		{"built outside a repository", &debug.BuildInfo{}, true},
		{"a revision too short to be one", &debug.BuildInfo{Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc"},
		}}, true},
		{"no build information at all", nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			linkedAs(t, "")
			stampedWith(t, c.info, c.ok)
			if got := appVersion(); got != unreleased {
				t.Errorf("it says it is %q, and it has nothing to go on", got)
			}
		})
	}
}

// ⛔ The version is in the status line of BOTH screens, and the empty one is
// the half that matters: somebody asking "why can I not see the thing you
// said you shipped" is looking at an empty workbench when they ask it.
func TestTheStatusLineSaysWhichBuildThisIs(t *testing.T) {
	linkedAs(t, "v0.19.0")

	empty := newState(surfaceW, surfaceH, &fakeHost{})
	if !strings.Contains(strings.Join(empty.statusLine(), " | "), "v0.19.0") {
		t.Errorf("the empty workbench's status line is %q", empty.statusLine())
	}

	s, _ := opened(t, 2)
	if !strings.Contains(strings.Join(s.statusLine(), " | "), "v0.19.0") {
		t.Errorf("an open document's status line is %q", s.statusLine())
	}
}
