package debriefstore

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dcsmanager/internal/db"
	"dcsmanager/internal/model"
)

const debriefSample = `mission_file_path	=	".\\Missions\\test.miz"
callsign	=	"Tester"
mission_time	=	100.0
result	=	0
events =
{
	[1] =
	{
		type	=	"takeoff",
		initiatorPilotName	=	"Tester",
		place	=	"Batumi",
		t	=	10.0,
	},
	[2] =
	{
		type	=	"land",
		initiatorPilotName	=	"Tester",
		place	=	"Batumi",
		t	=	90.0,
	},
}
`

func newAssembler(t *testing.T) (*Assembler, *db.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return New(database), database
}

// sendChunks splits content and feeds it to the assembler, mimicking the Lua side.
func sendChunks(a *Assembler, content string, chunkBytes int, transferID string) {
	total := (len(content) + chunkBytes - 1) / chunkBytes
	for i := 0; i < total; i++ {
		start := i * chunkBytes
		end := start + chunkBytes
		if end > len(content) {
			end = len(content)
		}
		a.Handle(model.Message{
			Type:       "debrief",
			TransferID: transferID,
			Chunk:      i,
			Chunks:     total,
			Size:       len(content),
			Name:       "Test",
			Data:       base64.StdEncoding.EncodeToString([]byte(content[start:end])),
		})
	}
}

func TestSingleChunk(t *testing.T) {
	a, database := newAssembler(t)
	_ = database

	var got model.Debrief
	a.OnDebrief = func(d model.Debrief) { got = d }

	sendChunks(a, debriefSample, len(debriefSample), "t1")

	if got.ID == 0 {
		t.Fatal("expected a stored debrief")
	}
	if len(got.Parsed.Events) != 2 {
		t.Fatalf("want 2 events, got %d", len(got.Parsed.Events))
	}
	if got.Parsed.Summary.Takeoffs != 1 || got.Parsed.Summary.Landings != 1 {
		t.Fatalf("unexpected summary: %+v", got.Parsed.Summary)
	}
	if a.InFlight() != 0 {
		t.Fatalf("transfer should be cleaned up, inFlight=%d", a.InFlight())
	}
}

func TestMultiChunk(t *testing.T) {
	a, database := newAssembler(t)

	var count int
	a.OnDebrief = func(model.Debrief) { count++ }

	// Small chunks force several frames.
	sendChunks(a, debriefSample, 40, "t2")

	if count != 1 {
		t.Fatalf("want exactly one stored debrief, got %d", count)
	}
	list, err := database.Debriefs(10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 debrief in db, got %d", len(list))
	}
	if list[0].Parsed.Summary.Pilots[0] != "Tester" {
		t.Fatalf("unexpected pilots: %v", list[0].Parsed.Summary.Pilots)
	}
}

func TestOutOfOrderChunks(t *testing.T) {
	a, database := newAssembler(t)

	// Feed chunk 1 before chunk 0.
	half := len(debriefSample) / 2
	chunks := []model.Message{
		{Type: "debrief", TransferID: "t3", Chunk: 1, Chunks: 2,
			Data: base64.StdEncoding.EncodeToString([]byte(debriefSample[half:]))},
		{Type: "debrief", TransferID: "t3", Chunk: 0, Chunks: 2,
			Data: base64.StdEncoding.EncodeToString([]byte(debriefSample[:half]))},
	}
	for _, m := range chunks {
		a.Handle(m)
	}

	list, err := database.Debriefs(10)
	if err != nil || len(list) != 1 {
		t.Fatalf("want 1 reassembled debrief (err=%v, n=%d)", err, len(list))
	}
	if len(list[0].Parsed.Events) != 2 {
		t.Fatalf("reassembly order wrong: %d events", len(list[0].Parsed.Events))
	}
}

func TestInvalidBase64DoesNotCrash(t *testing.T) {
	a, _ := newAssembler(t)
	a.Handle(model.Message{Type: "debrief", TransferID: "bad", Chunk: 0, Chunks: 1, Data: "!!!not base64!!!"})
	if a.InFlight() != 0 {
		t.Fatalf("invalid transfer should not stay in flight: %d", a.InFlight())
	}
}

func TestNonDebriefMessageNotHandled(t *testing.T) {
	a, _ := newAssembler(t)
	if a.Handle(model.Message{Type: "event", Event: "kill"}) {
		t.Fatal("event messages should not be handled by the debrief assembler")
	}
}

// TestAbandonedTransferIsDropped covers the leak: a transfer whose connection
// dropped mid-stream used to keep its chunks for the life of the process, since a
// reconnecting client starts a new transferId.
func TestAbandonedTransferIsDropped(t *testing.T) {
	a, _ := newAssembler(t)

	// Half a transfer, then silence.
	a.Handle(model.Message{
		Type: "debrief", TransferID: "abandoned", Chunk: 0, Chunks: 4,
		Data: base64.StdEncoding.EncodeToString([]byte("half")),
	})
	if got := a.InFlight(); got != 1 {
		t.Fatalf("want 1 transfer in flight, got %d", got)
	}

	// Age it past the TTL, then let the expiry run.
	a.mu.Lock()
	a.transfers["abandoned"].lastSeen = time.Now().Add(-2 * transferTTL)
	a.mu.Unlock()
	a.ExpireNow()

	if got := a.InFlight(); got != 0 {
		t.Fatalf("an abandoned transfer should have been dropped, %d still in flight", got)
	}
}

// TestInFlightTransfersAreCapped covers the other bound: a flood of incomplete
// transfers must not grow without limit either.
func TestInFlightTransfersAreCapped(t *testing.T) {
	a, _ := newAssembler(t)

	payload := base64.StdEncoding.EncodeToString([]byte("x"))
	for i := 0; i < maxInFlight+5; i++ {
		a.Handle(model.Message{
			Type: "debrief", TransferID: string(rune('a' + i)), Chunk: 0, Chunks: 3,
			Data: payload,
		})
	}
	if got := a.InFlight(); got > maxInFlight {
		t.Fatalf("in-flight transfers should be capped at %d, got %d", maxInFlight, got)
	}
}

// TestConcurrentTransfersStayIndependent interleaves two transfers chunk by chunk.
func TestConcurrentTransfersStayIndependent(t *testing.T) {
	a, database := newAssembler(t)

	var stored int
	a.OnDebrief = func(model.Debrief) { stored++ }

	// Interleave two transfers chunk by chunk.
	sendChunks(a, debriefSample, 50, "a")
	sendChunks(a, debriefSample, 50, "b")

	if stored != 2 {
		t.Fatalf("want 2 stored debriefs, got %d", stored)
	}
	list, _ := database.Debriefs(10)
	if len(list) != 2 {
		t.Fatalf("want 2 debriefs in db, got %d", len(list))
	}
}

// TestSizeMismatchIsRefused covers the corruption seen in the field: the sender
// declares the file size in every chunk, but the assembled content came out at
// roughly twice that (a duplicated base64 fragment on the wire). The wrong bytes
// must never be parsed or stored.
func TestSizeMismatchIsRefused(t *testing.T) {
	a, database := newAssembler(t)
	a.DumpDir = t.TempDir()

	var stored int
	a.OnDebrief = func(model.Debrief) { stored++ }

	// Two chunks whose declared total size does not match what they assemble to.
	a.Handle(model.Message{
		Type: "debrief", TransferID: "corrupt", Chunk: 0, Chunks: 2, Size: len(debriefSample),
		Data: base64.StdEncoding.EncodeToString([]byte(debriefSample[:20])),
	})
	a.Handle(model.Message{
		Type: "debrief", TransferID: "corrupt", Chunk: 1, Chunks: 2, Size: len(debriefSample),
		Data: base64.StdEncoding.EncodeToString([]byte("this is more than the declared total")),
	})

	if stored != 0 {
		t.Fatalf("a size mismatch must not be stored, got %d stored", stored)
	}
	if a.InFlight() != 0 {
		t.Fatalf("the corrupt transfer should be dropped, %d in flight", a.InFlight())
	}
	list, _ := database.Debriefs(10)
	if len(list) != 0 {
		t.Fatalf("nothing should be in the database, got %d", len(list))
	}
	// The bytes were kept for inspection.
	entries, _ := os.ReadDir(a.DumpDir)
	if len(entries) == 0 {
		t.Error("a refused transfer should be dumped for inspection")
	}
}

// TestSizeMatchIsStored checks a matching size still stores normally, so the
// guard does not reject valid transfers.
func TestSizeMatchIsStored(t *testing.T) {
	a, _ := newAssembler(t)

	var stored int
	a.OnDebrief = func(model.Debrief) { stored++ }
	sendChunks(a, debriefSample, len(debriefSample), "ok")

	if stored != 1 {
		t.Fatalf("a matching transfer should be stored, got %d", stored)
	}
}

// TestOverlongTransferIsRecovered locks the recovery of the corruption seen in the
// field: the reassembled content was the valid file followed by a duplicated base64
// fragment, and the sender declares the real size. The valid prefix must be stored
// rather than the whole thing dropped.
func TestOverlongTransferIsRecovered(t *testing.T) {
	a, database := newAssembler(t)

	var got model.Debrief
	a.OnDebrief = func(d model.Debrief) { got = d }

	// Assemble the real content plus two copies of a base64 fragment, exactly the
	// corruption observed, while declaring the true size.
	blob := debriefSample + debriefSample[100:180] + debriefSample[100:180]
	half := len(blob) / 2
	a.Handle(model.Message{Type: "debrief", TransferID: "over", Chunk: 0, Chunks: 2, Size: len(debriefSample),
		Data: base64.StdEncoding.EncodeToString([]byte(blob[:half]))})
	a.Handle(model.Message{Type: "debrief", TransferID: "over", Chunk: 1, Chunks: 2, Size: len(debriefSample),
		Data: base64.StdEncoding.EncodeToString([]byte(blob[half:]))})

	if got.ID == 0 {
		t.Fatal("the valid prefix should have been recovered and stored")
	}
	if len(got.Parsed.Events) != 2 {
		t.Fatalf("want 2 events from the recovered debrief, got %d", len(got.Parsed.Events))
	}
	if list, _ := database.Debriefs(10); len(list) != 1 {
		t.Fatalf("want 1 stored debrief, got %d", len(list))
	}
}

// TestInvalidChunkIndexIsRefused checks an out-of-range chunk index is rejected
// instead of being stored: chunk 99 of 1 is a broken sender.
func TestInvalidChunkIndexIsRefused(t *testing.T) {
	a, database := newAssembler(t)
	a.DumpDir = t.TempDir()

	var stored int
	a.OnDebrief = func(model.Debrief) { stored++ }

	a.Handle(model.Message{
		Type: "debrief", TransferID: "badidx", Chunk: 99, Chunks: 1, Size: len(debriefSample),
		Data: base64.StdEncoding.EncodeToString([]byte(debriefSample)),
	})

	if stored != 0 {
		t.Fatalf("an out-of-range chunk must not be stored, got %d", stored)
	}
	if a.InFlight() != 0 {
		t.Fatalf("the broken transfer must not stay in flight, %d", a.InFlight())
	}
	if list, _ := database.Debriefs(10); len(list) != 0 {
		t.Fatalf("nothing should be in the database, got %d", len(list))
	}
}

// TestChunkCountChangeDropsTransfer checks a transfer that announces one chunk
// count and then another is dropped rather than assembled from mixed data.
func TestChunkCountChangeDropsTransfer(t *testing.T) {
	a, _ := newAssembler(t)

	var stored int
	a.OnDebrief = func(model.Debrief) { stored++ }

	payload := base64.StdEncoding.EncodeToString([]byte("x"))
	a.Handle(model.Message{Type: "debrief", TransferID: "changing", Chunk: 0, Chunks: 2, Data: payload})
	a.Handle(model.Message{Type: "debrief", TransferID: "changing", Chunk: 1, Chunks: 3, Data: payload})

	if stored != 0 {
		t.Fatalf("a changing chunk count must be dropped, got %d stored", stored)
	}
	if a.InFlight() != 0 {
		t.Fatalf("the confused transfer must be dropped, %d in flight", a.InFlight())
	}
}
