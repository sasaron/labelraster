package rawprint

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestSend_Listener_ReceivesData(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	got := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			got <- nil
			return
		}
		defer conn.Close()
		b, _ := io.ReadAll(conn)
		got <- b
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	want := []byte("^XA^XZ")
	if err := Send(ctx, ln.Addr().String(), want); err != nil {
		t.Fatal(err)
	}
	if b := <-got; !bytes.Equal(b, want) {
		t.Errorf("received %q, want %q", b, want)
	}
}

func TestSend_ClosedPort_ReturnsError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	if err := Send(t.Context(), addr, []byte("x")); err == nil {
		t.Errorf("Send(%q) error = nil, want an error", addr)
	}
}

func TestWithPort_Address_AddsDefaultPortWhenMissing(t *testing.T) {
	tests := []struct{ addr, want string }{
		{"192.0.2.10", "192.0.2.10:9100"},
		{"printer.local", "printer.local:9100"},
		{"::1", "[::1]:9100"},
		{"[::1]", "[::1]:9100"},
		{"192.0.2.10:6101", "192.0.2.10:6101"},
		{"[::1]:6101", "[::1]:6101"},
	}
	for _, tt := range tests {
		if got := withPort(tt.addr); got != tt.want {
			t.Errorf("withPort(%q) = %q, want %q", tt.addr, got, tt.want)
		}
	}
}
