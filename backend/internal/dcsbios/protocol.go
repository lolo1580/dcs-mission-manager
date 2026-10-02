// Package dcsbios reads the DCS-BIOS export stream and sends it commands.
//
// DCS-BIOS is the established bridge between DCS World and cockpit hardware, and
// many players run it. Rather than reimplement it, the manager speaks its
// protocol: it listens to the state it broadcasts (the active aircraft, every
// switch and gauge) and, when asked, sends commands back on the port DCS-BIOS
// reserves for them.
//
// The protocol, from DCS-BIOS' own source:
//
//	frame  := sync block*
//	sync   := 0x55 0x55 0x55 0x55
//	block  := address(LE16) length(LE16) data[length]
//
// Every integer is little-endian. A frame carries only what changed; a consumer
// keeps a memory image and applies the blocks it receives.
package dcsbios

import "fmt"

// SyncSequence marks the start of a frame. Four 0x55 bytes, which is why the
// value 0x5555 is reserved in the memory map and never used for data.
var SyncSequence = [4]byte{0x55, 0x55, 0x55, 0x55}

// Block is one write request inside a frame: `length` bytes written at `address`.
type Block struct {
	Address uint16
	Data    []byte
}

// Frame is one decoded export frame.
type Frame struct {
	Blocks []Block
	// Bytes is how much was consumed from the stream, sync included, so a caller
	// can account for what it processed.
	Bytes int
}

// Apply writes a frame's blocks into a memory image. The image is keyed by
// address and grows as needed: DCS-BIOS addresses are sparse and can be large.
func (f Frame) Apply(mem map[uint16]byte) {
	for _, b := range f.Blocks {
		for i, v := range b.Data {
			mem[b.Address+uint16(i)] = v
		}
	}
}

// ErrShortFrame means the buffer holds no complete frame yet; the caller should
// accumulate more and try again.
var ErrShortFrame = fmt.Errorf("dcsbios: incomplete frame")

// Decode parses one frame from the front of buf.
//
// It returns ErrShortFrame when the sync sequence or a block header is cut off.
// A frame whose sync sequence is missing is skipped, up to the next sync: the
// stream is UDP, so a lost datagram can leave a partial one behind, and the
// robust thing is to resynchronise rather than refuse everything that follows.
func Decode(buf []byte) (Frame, error) {
	// Find the sync sequence. Skipping to it is the resynchronisation.
	start := indexSync(buf)
	if start < 0 {
		return Frame{}, ErrShortFrame
	}
	// Bytes before the sync are discarded; they belong to a damaged frame.
	skipped := start

	pos := start + len(SyncSequence)
	var blocks []Block

	for pos < len(buf) {
		// A block header is 4 bytes: address then length.
		if pos+4 > len(buf) {
			if len(blocks) == 0 {
				return Frame{}, ErrShortFrame
			}
			break // a partial block at the end: the frame's complete part is used
		}
		addr := uint16(buf[pos]) | uint16(buf[pos+1])<<8
		length := int(uint16(buf[pos+2]) | uint16(buf[pos+3])<<8)
		pos += 4

		if length == 0 {
			// A zero-length block carries nothing; stop rather than loop.
			break
		}
		if pos+length > len(buf) {
			if len(blocks) == 0 {
				return Frame{}, ErrShortFrame
			}
			break
		}
		blocks = append(blocks, Block{Address: addr, Data: buf[pos : pos+length]})
		pos += length

		// A sync sequence inside the stream starts the next frame.
		if pos+len(SyncSequence) <= len(buf) && indexSync(buf[pos:]) == 0 {
			break
		}
	}

	if len(blocks) == 0 {
		// Sync but nothing after it: a datagram cut before its first block. There
		// is nothing to apply, and the caller should wait for more rather than
		// treat it as an empty frame.
		return Frame{}, ErrShortFrame
	}

	return Frame{Blocks: blocks, Bytes: skipped + pos - start}, nil
}

// indexSync returns the offset of the sync sequence in buf, or -1.
//
// A run of 0x55 longer than the sequence counts too: resynchronising on
// 0x55 0x55 0x55 0x55 0x55 should land on the last four, which is where the next
// frame really starts. Scanning byte by byte and taking the first match would
// instead split a longer run and misread the header that follows.
func indexSync(buf []byte) int {
	i := 0
	for i < len(buf) {
		if buf[i] != 0x55 {
			i++
			continue
		}
		// At the start of a run of 0x55: find its end.
		j := i
		for j < len(buf) && buf[j] == 0x55 {
			j++
		}
		if j-i >= len(SyncSequence) {
			// The sync is the LAST four bytes of the run: a longer run means the
			// stream lost bytes, and the frame's header starts right after the
			// run, not four bytes into it.
			return j - len(SyncSequence)
		}
		// Too short to be a sync: it is data, keep looking after it.
		i = j
	}
	return -1
}

// EncodeCommand builds the line DCS-BIOS expects for a command: the identifier,
// a space, and the argument, newline-terminated.
//
// Command names are the identifiers from its own metadata (MASTER_ARM_SW,
// AP_BTN_Hdg…), and the argument is conventionally "1" then "0" for a button.
func EncodeCommand(identifier string, value int) []byte {
	return []byte(fmt.Sprintf("%s %d\n", identifier, value))
}
