package debriefstore

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dcsmanager/internal/model"
)

// TestDumpTagSanitized locks the fix for an arbitrary-file-write: the transfer
// id arrives over the network and was used verbatim in a filename, so a value
// containing "/../" could escape the dump directory.
func TestDumpTagSanitized(t *testing.T) {
	a, _ := newAssembler(t)
	dir := t.TempDir()
	a.DumpDir = dir

	a.dump([]byte("payload"), "a/../../../pwned")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one dump file, got %d", len(entries))
	}
	name := entries[0].Name()
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		t.Fatalf("dump filename still contains a path separator: %q", name)
	}
	// Nothing may have been written outside the dump dir.
	parent := filepath.Dir(dir)
	if _, err := os.Stat(filepath.Join(parent, "pwned")); !os.IsNotExist(err) {
		t.Fatal("transfer id escaped the dump directory")
	}
}

// TestAssemblerByteCap verifies the running-size guard still drops an oversized
// transfer once the O(n) accounting replaced the O(n²) rescan.
func TestAssemblerByteCap(t *testing.T) {
	a, _ := newAssembler(t)

	// One chunk past the cap must drop the transfer immediately.
	big := make([]byte, maxTransferSize+1)
	m := model.Message{
		Type:       "debrief",
		TransferID: "big",
		Chunk:      0,
		Chunks:     10,
		Size:       len(big) * 10,
		Data:       base64.StdEncoding.EncodeToString(big),
	}
	if !a.Handle(m) {
		t.Fatal("Handle should claim the message")
	}
	if a.InFlight() != 0 {
		t.Fatal("an oversized transfer must be dropped, not kept in flight")
	}
}

// TestAssemblerChunkCap verifies many tiny chunks are rejected independently of
// their total size.
func TestAssemblerChunkCap(t *testing.T) {
	a, _ := newAssembler(t)
	one := base64.StdEncoding.EncodeToString([]byte("x"))
	for i := 0; i < maxChunks+1; i++ {
		a.Handle(model.Message{
			Type:       "debrief",
			TransferID: "many",
			Chunk:      i,
			Chunks:     maxChunks + 10,
			Data:       one,
		})
	}
	if a.InFlight() != 0 {
		t.Fatal("a transfer past the chunk cap must be dropped")
	}
}
