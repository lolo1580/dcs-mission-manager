// Package debriefstore reassembles chunked debrief transfers arriving over TCP,
// parses them and hands the result to persistence.
package debriefstore

import (
	"encoding/base64"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"dcsmanager/internal/db"
	"dcsmanager/internal/debrief"
	"dcsmanager/internal/model"
)

// Assembler collects chunks of a debrief transfer and, once complete, parses
// and stores the result. Transfers are keyed by transferId and are independent,
// so a slow or aborted transfer never blocks another.
type Assembler struct {
	db *db.DB

	// DumpDir, when set, is where an assembled debrief that fails to parse is
	// written for inspection. A malformed transfer is otherwise dropped with only
	// its first log line, which is not enough to tell a transport corruption from
	// a parser gap. Empty disables the dump.
	DumpDir string

	mu        sync.Mutex
	transfers map[string]*transfer

	// OnDebrief, when set, is called after a debrief is stored (for SSE).
	OnDebrief func(model.Debrief)
}

type transfer struct {
	chunks  map[int][]byte
	total   int
	size    int
	theatre string
	mission string
	// lastSeen is when a chunk last arrived, used to expire a transfer whose
	// connection dropped mid-stream.
	lastSeen time.Time
}

// Transfer lifetime limits. A transfer whose TCP connection drops keeps its
// received chunks forever otherwise, because a reconnecting client starts a new
// transferId. Neither time nor volume may accumulate without bound.
const (
	transferTTL     = 10 * time.Minute
	maxInFlight     = 16
	maxTransferSize = 64 << 20 // 64 MiB, well above any real debrief.log
)

// New creates an assembler writing to database.
func New(database *db.DB) *Assembler {
	return &Assembler{
		db:        database,
		transfers: make(map[string]*transfer),
	}
}

// expire drops transfers that have seen no chunk for too long, and any that
// grew past the size cap. It must be called with a.mu held.
func (a *Assembler) expireLocked(now time.Time) {
	for id, tr := range a.transfers {
		if now.Sub(tr.lastSeen) > transferTTL {
			log.Printf("debrief: dropping transfer %s (no chunk for %s, %d/%d received)",
				id, transferTTL, len(tr.chunks), tr.total)
			delete(a.transfers, id)
		}
	}
}

// Handle consumes a "debrief" message. Returns true if the message was a
// debrief chunk (handled or not).
func (a *Assembler) Handle(m model.Message) bool {
	if m.Type != "debrief" || m.TransferID == "" {
		return m.Type == "debrief"
	}

	// The chunk metadata must be self-consistent before any of it is trusted: a
	// negative or out-of-range index, or a non-positive count, is a broken sender
	// and must not become a stored debrief. (A count of 0 is allowed by some
	// senders meaning "unknown", but then only chunk 0 is meaningful.)
	if m.Chunk < 0 || m.Chunks < 0 || (m.Chunks > 0 && m.Chunk >= m.Chunks) {
		log.Printf("debrief: rejecting chunk %d/%d of %s: invalid index",
			m.Chunk, m.Chunks, m.TransferID)
		return true
	}

	raw, err := base64.StdEncoding.DecodeString(m.Data)
	if err != nil {
		log.Printf("debrief: invalid chunk %d of %s: %v", m.Chunk, m.TransferID, err)
		return true
	}

	now := time.Now()

	a.mu.Lock()
	a.expireLocked(now)

	tr := a.transfers[m.TransferID]
	if tr == nil {
		// A flood of incomplete transfers must not grow without bound either.
		if len(a.transfers) >= maxInFlight {
			log.Printf("debrief: refusing transfer %s, %d already in flight", m.TransferID, len(a.transfers))
			a.mu.Unlock()
			return true
		}
		tr = &transfer{chunks: make(map[int][]byte), total: m.Chunks}
		a.transfers[m.TransferID] = tr
	} else if m.Chunks > 0 && tr.total > 0 && m.Chunks != tr.total {
		// The number of chunks changed mid-transfer: the sender is confused (or
		// two transfers collided on the same id). Refuse rather than assemble a
		// mix of two different files.
		log.Printf("debrief: transfer %s announced %d chunks then %d; dropping",
			m.TransferID, tr.total, m.Chunks)
		delete(a.transfers, m.TransferID)
		a.mu.Unlock()
		return true
	}
	tr.lastSeen = now
	tr.chunks[m.Chunk] = raw
	if m.Chunks > 0 {
		tr.total = m.Chunks
	}
	if m.Size > 0 {
		tr.size = m.Size
	}
	if m.Theatre != "" {
		tr.theatre = m.Theatre
	}
	if m.Name != "" {
		tr.mission = m.Name
	}

	// Guard against a transfer that never completes but keeps growing.
	received := 0
	for _, c := range tr.chunks {
		received += len(c)
	}
	if received > maxTransferSize {
		log.Printf("debrief: dropping transfer %s, %d bytes exceeds the cap", m.TransferID, received)
		delete(a.transfers, m.TransferID)
		a.mu.Unlock()
		return true
	}

	if len(tr.chunks) < tr.total {
		a.mu.Unlock()
		return true
	}

	// Complete: assemble in chunk order.
	indexes := make([]int, 0, len(tr.chunks))
	for i := range tr.chunks {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)

	var content []byte
	for _, i := range indexes {
		content = append(content, tr.chunks[i]...)
	}

	// The hook declares the file size in every chunk. A mismatch means the
	// transfer was corrupted on the wire (a duplicated or truncated chunk). When
	// the assembled content is *longer* than declared, the real file is its first
	// `size` bytes — the corruption appends a base64 fragment after the valid data
	// — so the transfer can be recovered instead of lost. When it is shorter, the
	// data is genuinely missing and the transfer is dropped.
	if tr.size > 0 && len(content) != tr.size {
		if len(content) > tr.size {
			candidate := content[:tr.size]
			if _, err := debrief.Parse(candidate); err == nil {
				log.Printf("debrief: transfer %s assembled %d bytes but the sender declared %d; recovered the first %d bytes",
					m.TransferID, len(content), tr.size, tr.size)
				content = candidate
			} else {
				log.Printf("debrief: transfer %s assembled %d bytes but the sender declared %d, and the prefix does not parse; dropping",
					m.TransferID, len(content), tr.size)
				a.dump(content, m.TransferID)
				delete(a.transfers, m.TransferID)
				a.mu.Unlock()
				return true
			}
		} else {
			log.Printf("debrief: transfer %s assembled %d bytes but the sender declared %d (truncated); dropping",
				m.TransferID, len(content), tr.size)
			a.dump(content, m.TransferID)
			delete(a.transfers, m.TransferID)
			a.mu.Unlock()
			return true
		}
	}

	delete(a.transfers, m.TransferID)
	a.mu.Unlock()

	a.store(content, tr.theatre, tr.mission)
	return true
}

func (a *Assembler) store(content []byte, theatre, mission string) {
	if a.db == nil {
		return
	}

	// A malformed debrief must never crash the backend: log and drop.
	parsed, err := debrief.Parse(content)
	if err != nil {
		log.Printf("debrief: parse failed (%d bytes): %v", len(content), err)
		a.dump(content, "parse")
		return
	}
	data := parsed.ToModel()

	missionID := a.db.OpenMissionID()
	rec := model.Debrief{
		MissionID: missionID,
		Mission:   mission,
		Theatre:   theatre,
		Raw:       string(content),
		Parsed:    data,
		Size:      len(content),
	}
	saved, err := a.db.SaveDebrief(rec)
	if err != nil {
		log.Printf("debrief: save failed: %v", err)
		return
	}
	log.Printf("debrief: stored #%d (%d events, %d bytes)", saved.ID, len(data.Events), len(content))

	if a.OnDebrief != nil {
		// Do not ship the raw text over SSE.
		saved.Raw = ""
		a.OnDebrief(saved)
	}
}

// dump writes a corrupt assembled debrief where it can be examined. It is
// best-effort: a failure to write must not turn a dropped debrief into a crash.
func (a *Assembler) dump(content []byte, tag string) {
	if a.DumpDir == "" {
		return
	}
	name := filepath.Join(a.DumpDir, "debrief-failed-"+tag+"-"+time.Now().Format("20060102-150405")+".bin")
	if err := os.MkdirAll(a.DumpDir, 0o755); err != nil {
		log.Printf("debrief: could not create dump dir: %v", err)
		return
	}
	if err := os.WriteFile(name, content, 0o644); err != nil {
		log.Printf("debrief: could not dump failed transfer: %v", err)
		return
	}
	log.Printf("debrief: dumped failed transfer to %s", name)
}

// InFlight returns the number of transfers currently being assembled (useful
// for diagnostics and tests).
func (a *Assembler) InFlight() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.transfers)
}

// ExpireNow drops transfers idle for longer than the TTL. It exists so the
// expiry can be tested without waiting ten minutes.
func (a *Assembler) ExpireNow() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.expireLocked(time.Now())
}
