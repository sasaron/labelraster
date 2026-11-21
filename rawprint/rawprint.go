// Package rawprint sends raw printer data over TCP (port 9100).
package rawprint

import (
	"context"
	"net"
	"strings"
	"time"
)

// DefaultPort is the raw printing port.
const DefaultPort = "9100"

// Send dials addr, writes data and closes the connection. Port 9100 is used
// when addr has none. Without a ctx deadline, the connection times out after
// one minute. Send does not report whether the printer finished printing.
func Send(ctx context.Context, addr string, data []byte) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", withPort(addr))
	if err != nil {
		return err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(time.Minute)
	}
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return err
	}
	if _, err := conn.Write(data); err != nil {
		_ = conn.Close()
		return err
	}
	return conn.Close()
}

// withPort adds DefaultPort to addr when it has no port.
func withPort(addr string) string {
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}
	host := strings.TrimSuffix(strings.TrimPrefix(addr, "["), "]")
	return net.JoinHostPort(host, DefaultPort)
}
