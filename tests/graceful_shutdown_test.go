package tests

import (
	"context"
	"net/smtp"
	"testing"
	"time"
)

// Shutdown must stop accepting new connections while letting an
// already-open one keep talking until it finishes on its own.
func TestGracefulShutdown(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1")
	addr, srv, cleanup := startSTARTTLSServer(t, cfg)
	defer cleanup()

	// Open a connection and keep it alive across the shutdown.
	inFlight, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer inFlight.Close()

	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownDone <- srv.Shutdown(ctx)
	}()

	// Give Shutdown a moment to stop the listener before we probe it.
	time.Sleep(100 * time.Millisecond)

	// New connections must now be refused.
	if _, err := smtp.Dial(addr); err == nil {
		t.Fatal("expected new connections to be refused once shutdown has started")
	}

	// The connection that was already open must still be able to finish
	// its command instead of being killed outright.
	if err := inFlight.Noop(); err != nil {
		t.Fatalf("expected the in-flight connection to keep working during drain, got: %v", err)
	}
	inFlight.Close()

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("expected Shutdown to complete cleanly, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not complete in time after the in-flight connection closed")
	}
}
