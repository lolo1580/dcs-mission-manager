package panelplugin

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func until(t *testing.T, test func() bool) {
	t.Helper()
	end := time.Now().Add(2 * time.Second)
	for time.Now().Before(end) {
		if test() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}
func TestTrimRequiresMatchingPluginAndHandlesAcknowledgements(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	c := New()
	if err := c.SendTrim("FA-18C_hornet", "1"); err == nil {
		t.Fatal("disconnected command accepted")
	}
	if err := c.Start(server.LocalAddr().String()); err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	buf := make([]byte, 256)
	server.SetReadDeadline(time.Now().Add(time.Second))
	n, peer, err := server.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	ping := strings.Fields(string(buf[:n]))
	if len(ping) != 4 || ping[3] != "PING" {
		t.Fatalf("ping: %q", buf[:n])
	}
	server.WriteToUDP([]byte(fmt.Sprintf("DCSM1 %s %s PONG FA-18C_hornet", ping[1], ping[2])), peer)
	until(t, func() bool { return c.State().Connected })
	for _, input := range []struct{ aircraft, value string }{{"F-16C_50", "1"}, {"FA-18C_hornet", "0"}, {"FA-18C_hornet", "INC"}} {
		if err := c.SendTrim(input.aircraft, input.value); err == nil {
			t.Fatal("invalid command accepted")
		}
	}
	for _, direction := range []struct{ value, packet, status string }{{"1", "UP", "OK"}, {"-1", "DN", "ERR_DEVICE"}} {
		if err := c.SendTrim("FA-18C_hornet", direction.value); err != nil {
			t.Fatal(err)
		}
		n, peer, err = server.ReadFromUDP(buf)
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.Fields(string(buf[:n]))
		if len(parts) != 5 || parts[3] != "TRIM" || parts[4] != direction.packet {
			t.Fatalf("trim: %q", buf[:n])
		}
		server.WriteToUDP([]byte(fmt.Sprintf("DCSM1 wrong %s OK FA-18C_hornet", parts[2])), peer)
		server.WriteToUDP([]byte(fmt.Sprintf("DCSM1 %s %s %s FA-18C_hornet", parts[1], parts[2], direction.status)), peer)
		if direction.status == "OK" {
			until(t, func() bool { return c.State().Accepted == 1 })
		} else {
			until(t, func() bool { return c.State().Error == "ERR_DEVICE" })
		}
	}
	c.mu.Lock()
	c.seen = time.Now().Add(-4 * time.Second)
	c.mu.Unlock()
	if c.State().Connected || c.State().Aircraft != "" {
		t.Fatal("stale plugin remains connected")
	}
	if err := c.SendTrim("FA-18C_hornet", "1"); err == nil {
		t.Fatal("stale plugin accepted trim")
	}
}
