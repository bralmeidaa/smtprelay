package tests

import (
	"crypto/tls"
	"net/smtp"
	"testing"
)

// Without authenticating at all, MAIL FROM must be rejected.
func TestNoAuthRejected(t *testing.T) {
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

	if err := c.Mail(testUser); err == nil {
		t.Fatal("expected MAIL FROM without authentication to be rejected, got no error")
	}
}
