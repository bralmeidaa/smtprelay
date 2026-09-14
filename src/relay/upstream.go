package relay

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"time"
)

// relayToUpstream opens a fresh authenticated STARTTLS connection to UOL for
// every message and sends it. A new connection per message keeps this
// simple and matches the very low volume this relay is built for; there is
// no persistent connection pool to manage or leak.
func relayToUpstream(ctx context.Context, cfg *Config, from string, to []string, data io.Reader) error {
	addr := net.JoinHostPort(cfg.UpstreamHost, cfg.UpstreamPort)

	dialer := net.Dialer{Timeout: cfg.UpstreamTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("upstream dial %s: %w", addr, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(cfg.UpstreamTimeout))
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.UpstreamHost)
	if err != nil {
		return fmt.Errorf("upstream handshake: %w", err)
	}
	defer client.Close()

	tlsConfig := &tls.Config{
		ServerName: cfg.UpstreamHost,
		MinVersion: tls.VersionTLS12,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("upstream starttls: %w", err)
	}

	auth := smtp.PlainAuth("", cfg.RelayUser, cfg.RelayPassword, cfg.UpstreamHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("upstream auth: %w", err)
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("upstream MAIL FROM: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("upstream RCPT TO %s: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("upstream DATA: %w", err)
	}
	if _, err := io.Copy(w, data); err != nil {
		_ = w.Close()
		return fmt.Errorf("upstream write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("upstream finish body: %w", err)
	}

	return client.Quit()
}
