package tcp

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"

	"dcsmm/internal/live"
)

// TestSendCommandReachesHook checks the whole command path: a hook connects, the
// backend pushes a command, and the hook receives exactly one JSON line with a
// trailing newline.
func TestSendCommandReachesHook(t *testing.T) {
	ln, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	l := NewListener(live.New(0, 0))
	go l.Serve(ln)

	// Play the hook: connect, then read.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Wait for the listener to register the connection.
	deadline := time.Now().Add(2 * time.Second)
	for l.Connected() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := l.Connected(); got != 1 {
		t.Fatalf("Connected() = %d, want 1", got)
	}

	sent := l.SendCommand(map[string]any{
		"type":    "command",
		"command": "chat",
		"message": "hello from the backend",
		"from":    "Server",
	})
	if sent != 1 {
		t.Fatalf("SendCommand reached %d hook(s), want 1", sent)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		t.Fatalf("read command: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(line, &got); err != nil {
		t.Fatalf("command is not valid JSON: %v (%q)", err, line)
	}
	if got["command"] != "chat" || got["message"] != "hello from the backend" {
		t.Fatalf("unexpected command: %v", got)
	}
}

// TestSendCommandWithoutHook checks the "DCS not connected" case: nothing is
// reachable, so the count is zero and the API can answer 503.
func TestSendCommandWithoutHook(t *testing.T) {
	l := NewListener(live.New(0, 0))
	if got := l.SendCommand(map[string]any{"command": "chat"}); got != 0 {
		t.Fatalf("SendCommand with no hook reached %d, want 0", got)
	}
	if l.Connected() != 0 {
		t.Fatalf("Connected() = %d, want 0", l.Connected())
	}
}

// TestConnectedTracksDisconnects checks that a closed hook is forgotten, so the
// API stops claiming DCS is reachable.
func TestConnectedTracksDisconnects(t *testing.T) {
	ln, err := Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	l := NewListener(live.New(0, 0))
	go l.Serve(ln)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for l.Connected() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if l.Connected() != 1 {
		t.Fatalf("Connected() = %d, want 1 after connect", l.Connected())
	}

	conn.Close()
	deadline = time.Now().Add(2 * time.Second)
	for l.Connected() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if l.Connected() != 0 {
		t.Fatalf("Connected() = %d, want 0 after disconnect", l.Connected())
	}
}
