// Package probe checks whether a server accepts connections. It is the
// one place crannog opens a network connection itself (ADR 0031).
package probe

import (
	"context"
	"net"
	"net/netip"
)

// TCP returns nil if something accepts a TCP connection on addr before
// ctx ends. It closes the connection at once and sends nothing.
func TCP(ctx context.Context, addr netip.AddrPort) error {
	var d net.Dialer
	c, err := d.DialContext(ctx, "tcp", addr.String())
	if err != nil {
		return err
	}
	return c.Close()
}
