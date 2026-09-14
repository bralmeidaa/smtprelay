package tests

import (
	"crypto/tls"
	"net/smtp"
	"testing"
)

// Wrong username or wrong password must both be rejected -- this is the
// core guarantee that keeps the relay from becoming an open relay by
// misconfiguration: there is exactly one valid identity.
func TestAuthInvalidPassword(t *testing.T) {
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

	auth := smtp.PlainAuth("", testUser, "wrong-password", "localhost")
	if err := c.Auth(auth); err == nil {
		t.Fatal("expected wrong password to be rejected, got no error")
	}
}

func TestAuthInvalidUser(t *testing.T) {
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

	auth := smtp.PlainAuth("", "someoneelse@uol.com.br", testPassword, "127.0.0.1")
	if err := c.Auth(auth); err == nil {
		t.Fatal("expected unknown username to be rejected, got no error")
	}
}
