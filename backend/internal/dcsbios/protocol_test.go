package dcsbios

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildFrame assembles a frame from blocks, the way DCS-BIOS does, so the tests
// exercise the real byte layout rather than a helper that could drift from it.
func buildFrame(blocks ...Block) []byte {
	var b bytes.Buffer
	b.Write(SyncSequence[:])
	for _, blk := range blocks {
		var hdr [4]byte
		binary.LittleEndian.PutUint16(hdr[0:2], blk.Address)
		binary.LittleEndian.PutUint16(hdr[2:4], uint16(len(blk.Data)))
		b.Write(hdr[:])
		b.Write(blk.Data)
	}
	return b.Bytes()
}

// TestDecodeSingleBlock checks the basic block layout: little-endian address and
// length, then the data.
func TestDecodeSingleBlock(t *testing.T) {
	raw := buildFrame(Block{Address: 0x1234, Data: []byte{0xAB, 0xCD}})
	f, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(f.Blocks) != 1 {
		t.Fatalf("blocks = %d, want 1", len(f.Blocks))
	}
	if f.Blocks[0].Address != 0x1234 {
		t.Errorf("address = %#04x, want 0x1234", f.Blocks[0].Address)
	}
	if !bytes.Equal(f.Blocks[0].Data, []byte{0xAB, 0xCD}) {
		t.Errorf("data = % X, want AB CD", f.Blocks[0].Data)
	}
	if f.Bytes != len(raw) {
		t.Errorf("Bytes = %d, want %d (the whole frame)", f.Bytes, len(raw))
	}
}

// TestDecodeMultipleBlocks checks several blocks in one frame, which is the
// common case: a frame carries every value that changed.
func TestDecodeMultipleBlocks(t *testing.T) {
	raw := buildFrame(
		Block{Address: 0x0000, Data: []byte{0x02, 0x00}},
		Block{Address: 0x0010, Data: []byte{0x01, 0x00, 0x00, 0x00}},
		Block{Address: 0x0200, Data: []byte{0xFF}},
	)
	f, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(f.Blocks) != 3 {
		t.Fatalf("blocks = %d, want 3", len(f.Blocks))
	}
	if f.Blocks[2].Address != 0x0200 || len(f.Blocks[2].Data) != 1 {
		t.Errorf("third block wrong: %+v", f.Blocks[2])
	}
}

// TestDecodeTwoFramesInOneDatagram checks a datagram holding two frames decodes
// the first and reports its length, so the caller can continue.
func TestDecodeTwoFramesInOneDatagram(t *testing.T) {
	first := buildFrame(Block{Address: 0x0000, Data: []byte{0x01, 0x00}})
	second := buildFrame(Block{Address: 0x0002, Data: []byte{0x02, 0x00}})
	raw := append(append([]byte{}, first...), second...)

	f, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if f.Bytes != len(first) {
		t.Fatalf("Bytes = %d, want %d (only the first frame)", f.Bytes, len(first))
	}
	// And the remainder is the second frame.
	f2, err := Decode(raw[f.Bytes:])
	if err != nil {
		t.Fatalf("Decode second: %v", err)
	}
	if f2.Blocks[0].Address != 0x0002 {
		t.Errorf("second frame address = %#04x, want 0x0002", f2.Blocks[0].Address)
	}
}

// TestDecodeResynchronises checks a frame preceded by garbage is still found: the
// stream is UDP, and a lost datagram can leave a partial one behind.
func TestDecodeResynchronises(t *testing.T) {
	junk := []byte{0x00, 0x01, 0x02, 0x03, 0x55, 0x55} // ends with a partial sync
	raw := append(junk, buildFrame(Block{Address: 0x0010, Data: []byte{0x42, 0x00}})...)

	f, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(f.Blocks) != 1 || f.Blocks[0].Address != 0x0010 {
		t.Fatalf("did not resynchronise: %+v", f.Blocks)
	}
}

// TestDecodeIncomplete checks a truncated frame reports ErrShortFrame rather than
// returning nonsense, so the caller can wait for more.
func TestDecodeIncomplete(t *testing.T) {
	cases := map[string][]byte{
		"empty":          {},
		"partial sync":   {0x55, 0x55},
		"sync only":      SyncSequence[:],
		"header cut":     append(append([]byte{}, SyncSequence[:]...), 0x00, 0x00),
		"data cut":       append(append([]byte{}, SyncSequence[:]...), 0x00, 0x00, 0x08, 0x00, 0x01, 0x02),
		"no sync at all": {0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
	}
	for name, raw := range cases {
		if _, err := Decode(raw); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

// TestFrameApply checks a frame's blocks land at the right addresses in the
// memory image, including a multi-byte value spanning two addresses.
func TestFrameApply(t *testing.T) {
	raw := buildFrame(
		Block{Address: 0x0000, Data: []byte{0x46, 0x2D, 0x31, 0x36, 0x43}}, // "F-16C"
		Block{Address: 0x0100, Data: []byte{0x34, 0x12}},                   // 0x1234 LE
	)
	f, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	mem := map[uint16]byte{}
	f.Apply(mem)

	if got := readString(mem, 0x0000, 24); got != "F-16C" {
		t.Errorf("string = %q, want F-16C", got)
	}
	got := binary.LittleEndian.Uint16([]byte{mem[0x0100], mem[0x0101]})
	if got != 0x1234 {
		t.Errorf("int = %#04x, want 0x1234", got)
	}
}

// TestReadStringStopsAtNullAndNONE checks the two things that matter for the
// aircraft name: it is null-terminated inside its fixed field, and "NONE" means
// no aircraft rather than an aircraft called NONE.
func TestReadStringStopsAtNullAndNONE(t *testing.T) {
	mem := map[uint16]byte{}
	// "A-10C" then a zero, then garbage that must be ignored.
	for i, c := range []byte("A-10C") {
		mem[uint16(i)] = c
	}
	mem[5] = 0
	mem[6] = 'X'
	if got := readString(mem, 0, AcftNameLength); got != "A-10C" {
		t.Errorf("name = %q, want A-10C", got)
	}

	mem2 := map[uint16]byte{}
	for i, c := range []byte("NONE") {
		mem2[uint16(i)] = c
	}
	if got := readString(mem2, 0, AcftNameLength); got != "" {
		t.Errorf("NONE should read as no aircraft, got %q", got)
	}

	if got := readString(map[uint16]byte{}, 0, AcftNameLength); got != "" {
		t.Errorf("an empty image should read as an empty name, got %q", got)
	}
}

// TestEncodeCommand checks the command line DCS-BIOS expects: identifier, space,
// value, newline.
func TestEncodeCommand(t *testing.T) {
	got := string(EncodeCommand("MASTER_ARM_SW", 1))
	if got != "MASTER_ARM_SW 1\n" {
		t.Errorf("EncodeCommand = %q, want %q", got, "MASTER_ARM_SW 1\n")
	}
}
