package dcsbios

import (
	"net"

	"golang.org/x/sys/windows"
)

func enableLocalMulticast(conn *net.UDPConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var optionErr error
	if err := raw.Control(func(fd uintptr) {
		optionErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, windows.IP_MULTICAST_LOOP, 1)
	}); err != nil {
		return err
	}
	return optionErr
}
