package tests

import (
	"crypto/tls"
	"net"
	"net/smtp"
	"strings"
	"testing"
)

// When UOL is unreachable, the relay must fail the message with a temporary
// (4xx) SMTP error -- never crash, and never silently drop the message as
// accepted.
func TestUpstreamUnavailable(t *testing.T) {
	host, port := closedPort(t)

	cfg := testConfig(host, port)
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
	_, _ = w.Write([]byte("Subject: test\r\n\r\nbody\r\n"))
	err = w.Close()
	if err == nil {
		t.Fatal("expected relaying to an unreachable upstream to fail")
	}
	if !strings.Contains(err.Error(), "451") {
		t.Fatalf("expected a 451 temporary-failure response, got: %v", err)
	}
}

// closedPort returns a host:port pair that is guaranteed to have nothing
// listening on it (a port that was briefly bound and then released).
func closedPort(t *testing.T) (host, port string) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	host, port, err = net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	_ = ln.Close()
	return host, port
}
