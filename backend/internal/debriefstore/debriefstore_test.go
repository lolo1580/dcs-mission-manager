package debriefstore

import (
	"encoding/base64"
	"path/filepath"
	"testing"

	"dcsmm/internal/db"
	"dcsmm/internal/model"
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
