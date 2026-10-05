package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallSurvivesOrphanBeginMarker locks the fix for user-code deletion: a
// hand-edited Export.lua whose END marker went missing used to make the next
// install swallow — and delete — everything between the orphan BEGIN and the
// block it appends.
func TestInstallSurvivesOrphanBeginMarker(t *testing.T) {
	in, sg := setup(t)

	existing := "user code A\n" + BeginMarker + "\nstale managed code\nuser code B\n"
	mustWrite(t, filepath.Join(sg, "Scripts", "Export.lua"), existing)

	if _, err := in.Install(DefaultTargets()); err != nil {
		t.Fatalf("install: %v", err)
	}
	got := mustRead(t, filepath.Join(sg, "Scripts", "Export.lua"))
	if !strings.Contains(got, "user code A") {
		t.Error("user code A was lost")
	}
	if !strings.Contains(got, "user code B") {
		t.Errorf("user code B was deleted by an orphan BEGIN:\n%s", got)
	}
}

// TestFindBlocksIgnoresDanglingBegin is the unit-level view of the same fix.
func TestFindBlocksIgnoresDanglingBegin(t *testing.T) {
	s := "a\n" + BeginMarker + "\nuser\n"
	if spans := findBlocks(s); len(spans) != 0 {
		t.Fatalf("a dangling begin must not be a block, got %v", spans)
	}
}

// TestFindBlocksRejectsKeywordInsideWord locks the word-boundary guard: a name
// like DCSMANAGER-BEGINNING must not be mistaken for the marker.
func TestFindBlocksRejectsKeywordInsideWord(t *testing.T) {
	s := "-- DCSMANAGER-BEGINNING of something\nuser\n"
	if spans := findBlocks(s); len(spans) != 0 {
		t.Fatalf("a keyword inside a word must not match, got %v", spans)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
