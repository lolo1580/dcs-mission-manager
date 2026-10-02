package config

import "testing"

// TestUDPPortAvoidsDCSBIOS locks the default UDP port away from 7778.
//
// DCS-BIOS owns UDP 7778 for its command channel, and plenty of cockpits run it
// alongside this manager. A default that collides would make one of the two fail
// to start, and the symptom (a silent bind failure) is easy to misread. This test
// exists so a future "tidy-up" of the defaults cannot quietly reintroduce it.
func TestUDPPortAvoidsDCSBIOS(t *testing.T) {
	cfg := Load()
	t.Setenv("DCSMANAGER_UDP_ADDR", "") // make sure the default is what is read
	cfg = Load()
	if cfg.UDPAddr == "127.0.0.1:7778" || cfg.UDPAddr == ":7778" {
		t.Fatalf("UDP default must not be 7778: DCS-BIOS owns it, got %q", cfg.UDPAddr)
	}
	if cfg.UDPAddr != "127.0.0.1:7776" {
		t.Fatalf("UDP default = %q, want 127.0.0.1:7776", cfg.UDPAddr)
	}
}

// TestTCPPortAvoidsDCSBIOS locks the TCP default too: DCS-BIOS also has a TCP
// listener on 7778, so 7779 stays clear of it.
func TestTCPPortAvoidsDCSBIOS(t *testing.T) {
	cfg := Load()
	if cfg.TCPAddr != "127.0.0.1:7779" {
		t.Fatalf("TCP default = %q, want 127.0.0.1:7779", cfg.TCPAddr)
	}
}
