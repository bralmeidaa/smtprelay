package tests

import (
	"bytes"
	"crypto/tls"
	"net/smtp"
	"testing"
)

// A message body larger than MaxMessageBytes must be rejected.
func TestMaxMessageSizeExceeded(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1")
	cfg.MaxMessageBytes = 256
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
		t.Fatalf("auth: %v", err)
	}
	if err := c.Mail(testUser); err != nil {
		t.Fatalf("mail: %v", err)
	}
	if err := c.Rcpt("destinatario@example.com"); err != nil {
		t.Fatalf("rcpt: %v", err)
	}

	w, err := c.Data()
	if err != nil {
		t.Fatalf("data: %v", err)
	}
	oversized := bytes.Repeat([]byte("x"), int(cfg.MaxMessageBytes)+1024)
	if _, err := w.Write(oversized); err != nil {
		// Some implementations fail the write itself, which also proves
		// the limit is enforced.
		return
	}
	if err := w.Close(); err == nil {
		t.Fatal("expected a message larger than MaxMessageBytes to be rejected")
	}
}
