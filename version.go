// Which build this is, said on the screen.
//
// ⛔ THE QUESTION THIS ANSWERS IS A DISTRIBUTION QUESTION, NOT A VANITY ONE.
// The workbench is a wasm binary served from a page with a service worker in
// front of it, and the one failure a reader cannot tell apart from a missing
// feature is an OLD BINARY: the page looks right, the control they were told
// about is not there, and nothing on the screen says which build is running.
// This repository has already paid for that once — web/sw.js carries the note:
// five releases went out and reached nobody who had visited before, and it was
// reported as the layout regressing. A version in the status line turns "I
// still cannot see it" into one glance at the bottom of the window.

package main

import "runtime/debug"

// release is set at LINK TIME by the deploy workflow:
//
//	-ldflags="-X main.release=$(git describe --tags --always --dirty)"
//
// It is empty in every other build, which is why appVersion has a second
// answer rather than showing nothing.
//
// ⛔ A -X flag that names a variable which no longer exists is accepted in
// SILENCE: the build succeeds and the string is simply absent. The deploy
// workflow therefore greps the built wasm for the string it just passed, so
// renaming this variable fails the build rather than quietly turning the
// status line back into a guess.
var release = ""

// unreleased is what a build made by hand, from a tree with no version stamped
// into it at all, calls itself. It says what it is rather than inventing a
// number, because a wrong version is worse than no version: it would answer the
// distribution question confidently and wrongly.
const unreleased = "built from source"

// buildInfo is a variable so a test can say what the toolchain stamped,
// including that it stamped nothing — which is a real case (a binary built
// outside a repository) and not one a test can otherwise reach.
var buildInfo = debug.ReadBuildInfo

// shortRevision is how much of a commit hash is shown: enough to tell two
// builds apart and name one to somebody, which is all this has to do.
const shortRevision = 7

// appVersion is what the status line says this build is.
func appVersion() string {
	if release != "" {
		return release
	}
	// Go stamps the commit into the binary with no flag from anybody, so a
	// build made by hand still says which commit it came from.
	if info, ok := buildInfo(); ok {
		for _, set := range info.Settings {
			if set.Key == "vcs.revision" && len(set.Value) >= shortRevision {
				return set.Value[:shortRevision]
			}
		}
	}
	return unreleased
}
