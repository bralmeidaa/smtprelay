package tests

import (
	"crypto/tls"
	"net/smtp"
	"testing"
)

// AUTH must never be offered/accepted on a plaintext connection -- TLS is
// mandatory on the client-facing side per the security requirements.
func TestTLS_AuthRejectedBeforeSTARTTLS(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1")
	addr, _, cleanup := startSTARTTLSServer(t, cfg)
	defer cleanup()

	c, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()

	auth := smtp.PlainAuth("", testUser, testPassword, "localhost")
	if err := c.Auth(auth); err == nil {
		t.Fatal("expected AUTH to be refused before STARTTLS, got no error")
	}
}

// The implicit-TLS listener (465-style) must accept a direct TLS connection
// and authenticate normally, for mail clients that only support "SSL" mode.
func TestTLS_ImplicitListenerAccepts(t *testing.T) {
	cfg := testConfig("127.0.0.1", "1")
	addr, clientTLSConfig, cleanup := startImplicitTLSServer(t, cfg)
	defer cleanup()

	conn, err := tls.Dial("tcp", addr, clientTLSConfig)
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		t.Fatalf("smtp handshake over implicit TLS: %v", err)
	}
	defer c.Close()

	auth := smtp.PlainAuth("", testUser, testPassword, "localhost")
	if err := c.Auth(auth); err != nil {
		t.Fatalf("expected valid credentials to be accepted over implicit TLS: %v", err)
	}
}
