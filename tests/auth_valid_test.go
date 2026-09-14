package tests

import (
	"crypto/tls"
	"net/smtp"
	"testing"
)

// A client presenting the exact configured relay credentials over STARTTLS
// must be allowed to authenticate.
func TestAuthValid(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1") // upstream irrelevant for this test
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
		t.Fatalf("expected valid credentials to be accepted, got error: %v", err)
	}
}
