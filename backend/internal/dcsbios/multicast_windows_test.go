package dcsbios

import (
	"net"
	"testing"

	"golang.org/x/sys/windows"
)

func TestLocalDCSBIOSMulticastIsEnabled(t *testing.T) {
	probe, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.LocalAddr().(*net.UDPAddr).Port
	probe.Close()

	client := New(Options{ReceivePort: port}, nil)
	if err := client.Start(); err != nil {
		t.Fatal(err)
	}
	defer client.Stop()

	raw, err := client.conn.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var option int
	var optionErr error
	if err := raw.Control(func(fd uintptr) {
		option, optionErr = windows.GetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, windows.IP_MULTICAST_LOOP)
	}); err != nil {
		t.Fatal(err)
	}
	if optionErr != nil {
		t.Fatal(optionErr)
	}
	if option != 1 {
		t.Fatalf("IP_MULTICAST_LOOP = %d, want 1 for DCS-BIOS on the same computer", option)
	}
}
