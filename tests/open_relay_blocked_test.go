package tests

import (
	"crypto/tls"
	"net/smtp"
	"testing"
)

// Explicit proof that this is not an open relay: even a client that
// authenticates successfully cannot send as an arbitrary sender address --
// only as the account it authenticated with. Combined with
// TestNoAuthRejected (unauthenticated clients rejected outright), this
// covers both ways an open relay could otherwise happen.
func TestOpenRelayBlocked_SenderMismatch(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1")
	addr, _, cleanup := startSTARTTLSServer(t, cfg)
	defer cleanup()

	c, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil { //nolint:gosec // test-only
		t.Fatalf("starttls: %v", err)
	}

	auth := smtp.PlainAuth("", testUser, testPassword, "127.0.0.1")
	if err := c.Auth(auth); err != nil {
		t.Fatalf("expected valid credentials to be accepted: %v", err)
	}

	if err := c.Mail("spoofed@example.com"); err == nil {
		t.Fatal("expected MAIL FROM with a sender that doesn't match the authenticated account to be rejected")
	}
}
