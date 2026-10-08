// Package panelplugin connects to the allowlisted DCS Manager cockpit extension.
// It never sends arbitrary Lua or device IDs received from profiles.
package panelplugin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const TrimCommand = "DCSM_PITCH_TRIM"
const Address = "127.0.0.1:7780"

type State struct {
	TrimEnabled bool   `json:"trimEnabled"`
	Connected   bool   `json:"connected"`
	Aircraft    string `json:"aircraft"`
	Accepted    uint64 `json:"accepted"`
	Error       string `json:"error,omitempty"`
}
type Client struct {
	mu      sync.Mutex
	conn    *net.UDPConn
	token   string
	seq     uint64
	state   State
	seen    time.Time
	pending map[uint64]time.Time
	done    chan struct{}
	wg      sync.WaitGroup
}

func New() *Client { return &Client{pending: make(map[uint64]time.Time), done: make(chan struct{})} }
func (c *Client) Start(address string) error {
	addr, err := net.ResolveUDPAddr("udp4", address)
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		conn.Close()
		return err
	}
	c.conn, c.token = conn, hex.EncodeToString(nonce[:])
	c.wg.Add(2)
	go c.read()
	go c.heartbeat()
	return nil
}
func (c *Client) Close() {
	_ = c.SetTrimEnabled(false)
	close(c.done)
	if c.conn != nil {
		c.conn.Close()
	}
	c.wg.Wait()
}

// Trim is deliberately opt-in per application session until cockpit validation.
// Disabling also cancels queued impulses and requests release in the Lua plugin.
func (c *Client) SetTrimEnabled(enabled bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expireLocked(time.Now())
	if enabled {
		if c.conn == nil || time.Since(c.seen) >= 3*time.Second || c.state.Aircraft != "FA-18C_hornet" {
			return errors.New("plugin: F/A-18C connecté requis pour activer le trim")
		}
		c.state.TrimEnabled = true
		return nil
	}
	c.state.TrimEnabled = false
	clear(c.pending)
	if c.conn != nil {
		_, err := c.sendLocked("CANCEL")
		return err
	}
	return nil
}

// A lost plugin connection ends the trim opt-in, even if a later heartbeat
// reconnects to the same aircraft. The Lua side also discards its session after
// three seconds without a heartbeat.
func (c *Client) expireLocked(now time.Time) {
	if c.seen.IsZero() || now.Sub(c.seen) < 3*time.Second {
		return
	}
	c.state.TrimEnabled = false
	clear(c.pending)
}

func (c *Client) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expireLocked(time.Now())
	s := c.state
	s.Connected = c.conn != nil && time.Since(c.seen) < 3*time.Second
	if !s.Connected {
		s.Aircraft = ""
	}
	return s
}

// SendTrim only enqueues one bounded impulse. An acknowledgement means the Lua
// device call was accepted, not proof that the simulated trim moved.
func (c *Client) SendTrim(aircraft, value string) error {
	n, err := strconv.Atoi(value)
	if err != nil || n == 0 || n < -65535 || n > 65535 {
		return errors.New("invalid trim direction")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.expireLocked(time.Now())
	if !c.state.TrimEnabled {
		return errors.New("trim expérimental désactivé")
	}
	if c.conn == nil || time.Since(c.seen) >= 3*time.Second {
		return errors.New("plugin DCS non connecté")
	}
	if aircraft != "FA-18C_hornet" || c.state.Aircraft != aircraft {
		return errors.New("plugin: F/A-18C actif requis")
	}
	if len(c.pending) >= 8 {
		return errors.New("plugin: file de trim pleine")
	}
	direction := "UP"
	if n < 0 {
		direction = "DN"
	}
	seq, err := c.sendLocked("TRIM " + direction)
	if err == nil {
		c.pending[seq] = time.Now()
	}
	return err
}
func (c *Client) sendLocked(command string) (uint64, error) {
	c.seq++
	_, err := fmt.Fprintf(c.conn, "DCSM1 %s %d %s", c.token, c.seq, command)
	return c.seq, err
}
func (c *Client) heartbeat() {
	defer c.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		c.mu.Lock()
		c.expireLocked(time.Now())
		_, _ = c.sendLocked("PING")
		for seq, at := range c.pending {
			if time.Since(at) > 2*time.Second {
				delete(c.pending, seq)
				c.state.Error = "trim: accusé de réception absent"
			}
		}
		c.mu.Unlock()
		select {
		case <-c.done:
			return
		case <-ticker.C:
		}
	}
}
func (c *Client) read() {
	defer c.wg.Done()
	buf := make([]byte, 256)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			select {
			case <-c.done:
				return
			default:
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		parts := strings.Fields(string(buf[:n]))
		if len(parts) != 5 || parts[0] != "DCSM1" || parts[1] != c.token {
			continue
		}
		seq, err := strconv.ParseUint(parts[2], 10, 64)
		if err != nil {
			continue
		}
		c.mu.Lock()
		if seq == 0 || seq > c.seq {
			c.mu.Unlock()
			continue
		}
		if parts[3] == "PONG" || parts[3] == "OK" || parts[3] == "CANCELLED" || strings.HasPrefix(parts[3], "ERR_") {
			c.expireLocked(time.Now())
			c.seen = time.Now()
			c.state.Aircraft = parts[4]
			if parts[4] == "NONE" {
				c.state.Aircraft = ""
			}
			if parts[3] == "ERR_RELEASE" {
				c.state.Error = "échec du relâchement du trim ; recharge la mission"
				c.state.TrimEnabled = false
			}
			if _, pending := c.pending[seq]; pending {
				delete(c.pending, seq)
				if parts[3] == "OK" {
					c.state.Accepted++
					c.state.Error = ""
				} else {
					c.state.Error = parts[3]
				}
			}
		}
		c.mu.Unlock()
	}
}
