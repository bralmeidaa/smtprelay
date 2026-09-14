package tests

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/bralmeidaa/smtprelay/src/relay"
	"github.com/emersion/go-smtp"
)

// testCredentials are the fixed relay account used across every test.
const (
	testUser     = "cliente@uol.com.br"
	testPassword = "s3nh4-super-secreta"
)

// selfSignedCert generates a throwaway TLS certificate for "localhost", used
// to serve the relay's own listeners in tests. Test clients connect with
// InsecureSkipVerify since they're deliberately testing our server, not a
// real CA chain.
func selfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("build tls.Certificate: %v", err)
	}
	return cert
}

// testConfig returns a Config wired for the given upstream. TLS cert paths
// are left empty because in tests the TLS cert is handed directly to the
// server constructors rather than loaded from disk.
func testConfig(upstreamHost, upstreamPort string) *relay.Config {
	return &relay.Config{
		RelayUser:              testUser,
		RelayPassword:          testPassword,
		Domain:                 "localhost",
		UpstreamHost:           upstreamHost,
		UpstreamPort:           upstreamPort,
		MaxMessageBytes:        1024,
		MaxRecipients:          5,
		ReadTimeout:            2 * time.Second,
		WriteTimeout:           2 * time.Second,
		UpstreamTimeout:        1 * time.Second,
		// STARTTLS forces a second EHLO after the upgrade (go-smtp
		// correctly discards pre-TLS session state per RFC 3207), which
		// consumes a second connection-rate-limit credit -- so this needs
		// headroom for a handful of full STARTTLS+Auth round trips per
		// test. Individual tests override this when they specifically
		// want to exercise the limit.
		RateLimitConnPerMinute: 20,
		RateLimitMsgsPerHour:   2,
	}
}

// startSTARTTLSServer starts the relay's submission-style listener on an
// ephemeral local port and returns its address, the underlying server
// (tests that need direct Shutdown/Close control can use it) and a cleanup
// func safe to call unconditionally (e.g. via defer).
func startSTARTTLSServer(t *testing.T, cfg *relay.Config) (addr string, srv *smtp.Server, cleanup func()) {
	t.Helper()

	logger := relay.NewLogger()
	auth := relay.NewAuthenticator(cfg)
	limiter := relay.NewRateLimiter(cfg.RateLimitConnPerMinute, cfg.RateLimitMsgsPerHour)
	backend := relay.NewBackend(cfg, auth, limiter, logger)

	cert := selfSignedCert(t)
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}

	srv = relay.NewSTARTTLSServer(cfg, backend, tlsConfig)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		_ = srv.Serve(ln)
	}()

	return ln.Addr().String(), srv, func() {
		_ = srv.Close()
	}
}

// startImplicitTLSServer starts the SMTPS-style (implicit TLS) listener.
func startImplicitTLSServer(t *testing.T, cfg *relay.Config) (addr string, tlsConfig *tls.Config, cleanup func()) {
	t.Helper()

	logger := relay.NewLogger()
	auth := relay.NewAuthenticator(cfg)
	limiter := relay.NewRateLimiter(cfg.RateLimitConnPerMinute, cfg.RateLimitMsgsPerHour)
	backend := relay.NewBackend(cfg, auth, limiter, logger)

	cert := selfSignedCert(t)
	serverTLSConfig := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}

	srv := relay.NewImplicitTLSServer(cfg, backend, serverTLSConfig)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	tlsLn := tls.NewListener(ln, serverTLSConfig)

	go func() {
		_ = srv.Serve(tlsLn)
	}()

	clientTLSConfig := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test-only, trusts our own throwaway cert
	return ln.Addr().String(), clientTLSConfig, func() {
		_ = srv.Close()
	}
}
