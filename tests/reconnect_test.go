package tests

import (
	"crypto/tls"
	"net/smtp"
	"testing"
)

// After a client disconnects, a fresh connection must behave exactly like
// the first one -- no per-connection state (authentication, transaction)
// leaks into the next session on the shared backend.
func TestReconnectAfterDisconnect(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1")
	addr, _, cleanup := startSTARTTLSServer(t, cfg)
	defer cleanup()

	authenticate := func() error {
		c, err := smtp.Dial(addr)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer c.Close()

		if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil { //nolint:gosec // test-only
			t.Fatalf("starttls: %v", err)
		}
		auth := smtp.PlainAuth("", testUser, testPassword, "127.0.0.1")
		return c.Auth(auth)
	}

	if err := authenticate(); err != nil {
		t.Fatalf("first connection: expected auth to succeed, got: %v", err)
	}
	if err := authenticate(); err != nil {
		t.Fatalf("second connection after reconnect: expected auth to succeed, got: %v", err)
	}

	// A fresh connection must not be authenticated by default just because
	// a previous connection on the same backend was.
	c, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil { //nolint:gosec // test-only
		t.Fatalf("starttls: %v", err)
	}
	if err := c.Mail(testUser); err == nil {
		t.Fatal("expected a brand new, unauthenticated connection to reject MAIL FROM")
	}
}
