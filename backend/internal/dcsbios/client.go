package dcsbios

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Defaults from DCS-BIOS' own configuration.
const (
	// DefaultMulticast is the export group DCS-BIOS broadcasts to.
	DefaultMulticast = "239.255.50.10"
	// DefaultReceivePort is the export port.
	DefaultReceivePort = 5010
	// DefaultCommandPort is where DCS-BIOS listens for commands.
	DefaultCommandPort = 7778

	// AcftNameAddress is the address of the "_ACFT_NAME" string in the
	// MetadataStart module, which every DCS-BIOS install exports.
	AcftNameAddress = 0x0000
	// AcftNameLength is that string's length, null-terminated.
	AcftNameLength = 24
)

// Options configures a Client.
type Options struct {
	// Multicast is the export group (default 239.255.50.10).
	Multicast string
	// ReceivePort is the export port (default 5010).
	ReceivePort int
	// CommandPort is DCS-BIOS' command port (default 7778).
	CommandPort int
	// CommandAddress is where commands are sent (default 127.0.0.1).
	CommandAddress string
	// InactivityTimeout is how long without a frame before the link is considered
	// down (default 3s, as DCS-BIOS itself assumes).
	InactivityTimeout time.Duration
}

// DefaultOptions returns the options matching a stock DCS-BIOS install.
func DefaultOptions() Options {
	return Options{
		Multicast:         DefaultMulticast,
		ReceivePort:       DefaultReceivePort,
		CommandPort:       DefaultCommandPort,
		CommandAddress:    "127.0.0.1",
		InactivityTimeout: 3 * time.Second,
	}
}

// Client listens to the DCS-BIOS export stream and sends commands to it.
type Client struct {
	opts Options

	// onChange is called when the visible state changes (a new frame, a new
	// aircraft). It must not block.
	onChange func(State)
	// OnFrame is called after every applied frame, without exception. Unlike
	// onChange (which is coalesced), it is the tick that redraws outputs: a frame
	// that does not change the aircraft may still change an exported value an LED or
	// the LCD shows. It must not block, and it runs once per frame. Set it before
	// Start.
	OnFrame func()

	mu       sync.RWMutex
	mem      map[uint16]byte
	aircraft string
	lastSeen time.Time
	frames   uint64

	conn *net.UDPConn
	stop chan struct{}
	done chan struct{}
}

// New creates a client. onChange may be nil.
func New(opts Options, onChange func(State)) *Client {
	if opts.Multicast == "" {
		opts.Multicast = DefaultMulticast
	}
	if opts.ReceivePort == 0 {
		opts.ReceivePort = DefaultReceivePort
	}
	if opts.CommandPort == 0 {
		opts.CommandPort = DefaultCommandPort
	}
	if opts.CommandAddress == "" {
		opts.CommandAddress = "127.0.0.1"
	}
	if opts.InactivityTimeout == 0 {
		opts.InactivityTimeout = 3 * time.Second
	}
	if onChange == nil {
		onChange = func(State) {}
	}
	return &Client{
		opts:     opts,
		onChange: onChange,
		mem:      make(map[uint16]byte),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// State is what the client exposes to the rest of the manager.
type State struct {
	// Connected is true when a frame arrived within the inactivity timeout.
	Connected bool
	// Aircraft is the active aircraft's DCS name, or "" when none is loaded.
	Aircraft string
	// Frames is how many frames have been received, useful as a liveness signal.
	Frames uint64
	// LastSeen is when the last frame arrived.
	LastSeen time.Time
}

// State returns the current state.
func (c *Client) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return State{
		Connected: c.frames > 0 && time.Since(c.lastSeen) < c.opts.InactivityTimeout,
		Aircraft:  c.aircraft,
		Frames:    c.frames,
		LastSeen:  c.lastSeen,
	}
}

// Memory returns a copy of the decoded memory image, for callers that need to
// read an address the state does not expose.
func (c *Client) Memory() map[uint16]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[uint16]byte, len(c.mem))
	for k, v := range c.mem {
		out[k] = v
	}
	return out
}

// Start joins the multicast group and reads until Stop. It returns immediately.
func (c *Client) Start() error {
	group, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", c.opts.Multicast, c.opts.ReceivePort))
	if err != nil {
		return fmt.Errorf("dcsbios: resolve %s: %w", c.opts.Multicast, err)
	}
	// Binding to the wildcard address and joining the group is the standard way
	// to receive multicast on every local interface.
	conn, err := net.ListenMulticastUDP("udp4", nil, group)
	if err != nil {
		return fmt.Errorf("dcsbios: join %s: %w", group, err)
	}
	// Go disables multicast loopback on this socket. On Windows the option is
	// checked by the receiver, so the manager would miss DCS-BIOS packets sent
	// by DCS on the same computer unless it is enabled again.
	if err := enableLocalMulticast(conn); err != nil {
		conn.Close()
		return fmt.Errorf("dcsbios: enable local multicast: %w", err)
	}
	// A generous buffer: DCS-BIOS caps itself around 11 KB/s but a burst can be
	// larger than the default, and a truncated datagram would corrupt the stream.
	_ = conn.SetReadBuffer(1 << 20)
	c.conn = conn

	go c.readLoop()
	return nil
}

// Stop finishes. It is safe to call once.
func (c *Client) Stop() {
	select {
	case <-c.stop:
		return
	default:
	}
	close(c.stop)
	if c.conn != nil {
		_ = c.conn.Close()
	}
	<-c.done
}

func (c *Client) readLoop() {
	defer close(c.done)

	buf := make([]byte, 64*1024)
	for {
		select {
		case <-c.stop:
			return
		default:
		}

		n, _, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-c.stop:
				return // closing the socket is how Stop interrupts this
			default:
				continue
			}
		}
		c.handleDatagram(buf[:n])
	}
}

// handleDatagram decodes one datagram and applies it. A datagram may hold more
// than one frame; it is decoded until nothing complete remains.
func (c *Client) handleDatagram(data []byte) {
	rest := data
	for len(rest) > 0 {
		frame, err := Decode(rest)
		if err != nil {
			return // incomplete: nothing more to do with what is left
		}
		if frame.Bytes <= 0 {
			return // no progress: give up rather than spin
		}

		changed := c.applyFrame(frame)
		rest = rest[frame.Bytes:]

		if changed {
			c.onChange(c.State())
		}
		// Every applied frame may change an exported output, whether or not the
		// visible state did.
		if c.OnFrame != nil {
			c.OnFrame()
		}
	}
}

// applyFrame writes a frame into the memory image and updates the derived state.
// It reports whether the visible state changed.
func (c *Client) applyFrame(f Frame) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	f.Apply(c.mem)
	c.frames++
	c.lastSeen = time.Now()

	// The active aircraft name is what a cockpit tool watches most.
	name := readString(c.mem, AcftNameAddress, AcftNameLength)
	if name != c.aircraft {
		// Aircraft modules reuse export addresses. Discard old cockpit values,
		// then retain only values actually supplied in the new aircraft's frame.
		for address := range c.mem {
			if address >= AcftNameLength {
				delete(c.mem, address)
			}
		}
		f.Apply(c.mem)
		c.aircraft = name
		return true
	}
	// A frame that only touched the name is not itself interesting; report the
	// first frame so a connection is noticed.
	return c.frames == 1
}

// readString reads a null-terminated string from the memory image.
func readString(mem map[uint16]byte, addr uint16, length int) string {
	b := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		v, ok := mem[addr+uint16(i)]
		if !ok || v == 0 {
			break
		}
		b = append(b, v)
	}
	s := strings.TrimSpace(string(b))
	if s == "NONE" {
		return ""
	}
	return s
}

// SendCommand sends one command to DCS-BIOS' command port. An empty identifier
// is refused: DCS-BIOS would log a malformed line.
func (c *Client) SendCommand(identifier string, value int) error {
	if identifier == "" {
		return fmt.Errorf("dcsbios: empty command identifier")
	}
	addr, err := net.ResolveUDPAddr("udp4",
		fmt.Sprintf("%s:%d", c.opts.CommandAddress, c.opts.CommandPort))
	if err != nil {
		return fmt.Errorf("dcsbios: resolve command address: %w", err)
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("dcsbios: dial command port: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write(EncodeCommand(identifier, value)); err != nil {
		return fmt.Errorf("dcsbios: send command: %w", err)
	}
	return nil
}

// ReadInt reads a 16-bit little-endian value from the memory image, which is how
// DCS-BIOS stores its integer outputs.
func (c *Client) ReadInt(addr uint16) (uint16, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	lo, ok1 := c.mem[addr]
	hi, ok2 := c.mem[addr+1]
	if !ok1 || !ok2 {
		return 0, false
	}
	return binary.LittleEndian.Uint16([]byte{lo, hi}), true
}
