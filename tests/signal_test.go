package tests

import (
	"net"
	"syscall"
	"testing"
	"time"
)

var sigterm = syscall.SIGTERM

func waitListening(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("nothing listens on %s", addr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
