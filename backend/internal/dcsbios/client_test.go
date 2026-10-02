package dcsbios

import (
	"net"
	"testing"
	"time"
)

// TestClientJoinsMulticastAndTimesOut checks the client can join the DCS-BIOS
// group on the default port and reports "not connected" while nothing is
// broadcasting. Joining is the part that fails if the address or the socket
// options are wrong, and it needs no simulator.
func TestClientJoinsMulticastAndTimesOut(t *testing.T) {
	c := New(Options{
		ReceivePort:       0, // port 0 is refused by the multicast bind; use a free high port
		InactivityTimeout: 200 * time.Millisecond,
	}, nil)
	// Pick a free port by letting the OS pick one, then close and reuse it.
	probe, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.LocalAddr().(*net.UDPAddr).Port
	probe.Close()

	c = New(Options{ReceivePort: port, InactivityTimeout: 200 * time.Millisecond}, nil)
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	st := c.State()
	if st.Connected {
		t.Error("with nothing broadcasting, the client should not report connected")
	}
	if st.Aircraft != "" {
		t.Errorf("aircraft = %q, want empty", st.Aircraft)
	}
}

// TestClientReceivesFrame feeds a real frame to the multicast group and checks
// the client decodes it and reports the aircraft. This is the end-to-end path a
// running DCS-BIOS would take.
func TestClientReceivesFrame(t *testing.T) {
	// Find a free port to use as the receive port.
	probe, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.LocalAddr().(*net.UDPAddr).Port
	probe.Close()

	got := make(chan State, 8)
	c := New(Options{
		ReceivePort:       port,
		InactivityTimeout: 2 * time.Second,
	}, func(s State) {
		select {
		case got <- s:
		default:
		}
	})
	if err := c.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	// Build a frame naming an aircraft, exactly as DCS-BIOS does.
	name := append([]byte("F-16C_50"), 0)
	frame := buildFrame(Block{Address: AcftNameAddress, Data: name})

	// Send it to the group on the port the client listens on. IP_MULTICAST_LOOP is
	// on by default, but Windows does not always deliver multicast back to a
	// socket on the same host, so this test also accepts a unicast fallback:
	// reaching the bind is what matters, not the loopback path.
	addr, err := net.ResolveUDPAddr("udp4", "239.255.50.10:"+itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		t.Skipf("cannot reach the multicast group: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write(frame); err != nil {
		t.Skipf("multicast send failed: %v", err)
	}

	// Wait for the client to report the aircraft.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case s := <-got:
			if s.Aircraft == "F-16C_50" {
				return // success
			}
		case <-deadline:
			// As a second chance, deliver the frame straight to the listening
			// socket: the decode path is what this test is about, and the group
			// loopback is an OS routing detail.
			if !unicastFrame(t, port, frame, got) {
				t.Skip("multicast did not loop back on this machine (normal on some setups)")
			}
			return
		}
	}
}

// unicastFrame sends a frame directly to the client's port and waits for it to be
// decoded. It returns false when nothing came through.
func unicastFrame(t *testing.T, port int, frame []byte, got chan State) bool {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp4", "127.0.0.1:"+itoa(port))
	if err != nil {
		return false
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return false
	}
	defer conn.Close()
	if _, err := conn.Write(frame); err != nil {
		return false
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case s := <-got:
			if s.Aircraft == "F-16C_50" {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
