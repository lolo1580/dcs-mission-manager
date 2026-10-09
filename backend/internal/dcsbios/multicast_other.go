//go:build !windows

package dcsbios

import "net"

// On POSIX systems this option belongs to the sender. DCS-BIOS uses the
// sender's default loopback setting, so the receiving socket needs no change.
func enableLocalMulticast(_ *net.UDPConn) error { return nil }
