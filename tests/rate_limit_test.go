package tests

import (
	"crypto/tls"
	"net/smtp"
	"strings"
	"testing"
)

// Beyond RateLimitConnPerMinute connections from the same IP within the
// window, new connections must be refused with a 421.
func TestRateLimit_Connections(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1")
	cfg.RateLimitConnPerMinute = 2
	addr, _, cleanup := startSTARTTLSServer(t, cfg)
	defer cleanup()

	// The rate limiter is only consulted when a session is actually
	// created, which go-smtp does lazily on the first HELO/EHLO -- a bare
	// TCP connect (which always gets the 220 banner) doesn't trigger it.
	// First two EHLOs are allowed.
	for i := 0; i < 2; i++ {
		c, err := smtp.Dial(addr)
		if err != nil {
			t.Fatalf("connection %d: dial: %v", i+1, err)
		}
		if err := c.Hello("tester"); err != nil {
			t.Fatalf("connection %d: expected EHLO to be allowed, got error: %v", i+1, err)
		}
		c.Close()
	}

	// The third, within the same window, must be rejected.
	c, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	err = c.Hello("tester")
	if err == nil {
		t.Fatal("expected the EHLO past the connection rate limit to be rejected, got no error")
	}
	if !strings.Contains(err.Error(), "421") {
		t.Fatalf("expected a 421 response, got: %v", err)
	}
}

// Beyond RateLimitMsgsPerHour messages from the same IP within the window,
// DATA must be refused with a 452, independently of whether the upstream
// would have accepted the message.
func TestRateLimit_Messages(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1") // upstream unreachable: fine, message rate limit is checked first
	cfg.RateLimitMsgsPerHour = 1
	addr, _, cleanup := startSTARTTLSServer(t, cfg)
	defer cleanup()

	send := func() error {
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
		return w.Close()
	}

	// First message: allowed by the rate limiter (fails later, at the
	// unreachable-upstream stage, with a 451 -- not a 452).
	if err := send(); err == nil || strings.Contains(err.Error(), "452") {
		t.Fatalf("expected the first message to pass the rate limiter (fail with 451 instead), got: %v", err)
	}

	// Second message, same hour: rejected by the rate limiter itself.
	err := send()
	if err == nil {
		t.Fatal("expected the second message to be rejected by the message rate limit")
	}
	if !strings.Contains(err.Error(), "452") {
		t.Fatalf("expected a 452 rate-limit response, got: %v", err)
	}
}
