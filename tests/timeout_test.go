package tests

import (
	"bufio"
	"net"
	"testing"
	"time"
)

// An idle client (connected but not sending anything) must be dropped after
// ReadTimeout, so a slow or stuck peer can't hold a connection (and a slot
// in the rate limiter) forever.
func TestIdleConnectionTimesOut(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1")
	cfg.ReadTimeout = 300 * time.Millisecond
	cfg.WriteTimeout = 300 * time.Millisecond

	addr, _, cleanup := startSTARTTLSServer(t, cfg)
	defer cleanup()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("expected greeting banner, got error: %v", err)
	}

	// Stay idle without sending anything. The server should react to the
	// idle timeout by writing a timeout response (if it sends one) and, in
	// any case, closing the connection well before our own generous
	// client-side deadline -- proving it's the *server's* ReadTimeout that
	// fired, not just our own read giving up.
	start := time.Now()
	_ = conn.SetReadDeadline(start.Add(3 * time.Second))
	for {
		if _, err := reader.ReadString('\n'); err != nil {
			break
		}
	}
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Fatalf("expected the server to close the idle connection close to its 300ms ReadTimeout, took %v", elapsed)
	}
}
