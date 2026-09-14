package relay

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// Backend wires every inbound SMTP session to the relay's auth, rate
// limiting and upstream-forwarding logic. One Backend instance is shared by
// both listeners (STARTTLS on 587 and implicit TLS on 465).
type Backend struct {
	cfg     *Config
	auth    *Authenticator
	limiter *RateLimiter
	logger  *slog.Logger
}

func NewBackend(cfg *Config, auth *Authenticator, limiter *RateLimiter, logger *slog.Logger) *Backend {
	return &Backend{cfg: cfg, auth: auth, limiter: limiter, logger: logger}
}

func (b *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	ip := remoteIP(c)
	if !b.limiter.AllowConn(ip) {
		b.logger.Warn("connection rate limit exceeded", "ip", ip)
		return nil, &smtp.SMTPError{Code: 421, Message: "too many connections, try again later"}
	}
	return &Session{backend: b, remoteIP: ip}, nil
}

// Session tracks the state of a single SMTP transaction. It implements
// smtp.Session and smtp.AuthSession.
type Session struct {
	backend  *Backend
	remoteIP string

	authenticated bool
	username      string

	from string
	to   []string
}

func (s *Session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

func (s *Session) Auth(mech string) (sasl.Server, error) {
	return sasl.NewPlainServer(func(identity, username, password string) error {
		if !s.backend.auth.Validate(username, password) {
			s.backend.logger.Warn("authentication failed", "ip", s.remoteIP, "user", maskEmail(username))
			return fmt.Errorf("invalid username or password")
		}
		s.authenticated = true
		s.username = username
		s.backend.logger.Info("authentication succeeded", "ip", s.remoteIP, "user", maskEmail(username))
		return nil
	}), nil
}

// Mail is called on MAIL FROM. It enforces two things beyond basic parsing:
// the session must already be authenticated, and the envelope sender must
// match the authenticated account. The second check stops an authenticated
// session from forging an arbitrary From address through this relay -
// narrower than classic open-relay abuse, but the same "don't let a single
// mistake turn this into a spam vector" spirit as the auth requirement.
func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	if !s.authenticated {
		s.backend.logger.Warn("MAIL FROM rejected: not authenticated", "ip", s.remoteIP)
		return &smtp.SMTPError{Code: 530, Message: "authentication required"}
	}
	if !addressMatches(from, s.username) {
		s.backend.logger.Warn("MAIL FROM rejected: sender does not match authenticated account", "ip", s.remoteIP)
		return &smtp.SMTPError{Code: 553, Message: "sender address does not match authenticated account"}
	}
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	if !s.authenticated {
		return &smtp.SMTPError{Code: 530, Message: "authentication required"}
	}
	if len(s.to) >= s.backend.cfg.MaxRecipients {
		return &smtp.SMTPError{Code: 452, Message: "too many recipients"}
	}
	s.to = append(s.to, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	if !s.authenticated {
		return &smtp.SMTPError{Code: 530, Message: "authentication required"}
	}
	if !s.backend.limiter.AllowMessage(s.remoteIP) {
		s.backend.logger.Warn("message rate limit exceeded", "ip", s.remoteIP, "user", maskEmail(s.username))
		return &smtp.SMTPError{Code: 452, Message: "message rate limit exceeded, try again later"}
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		return &smtp.SMTPError{Code: 451, Message: "failed to read message"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.backend.cfg.UpstreamTimeout)
	defer cancel()

	if err := relayToUpstream(ctx, s.backend.cfg, s.from, s.to, &buf); err != nil {
		s.backend.logger.Error("upstream relay failed", "ip", s.remoteIP, "user", maskEmail(s.username), "error", err.Error())
		return &smtp.SMTPError{Code: 451, Message: "temporary failure relaying to upstream, try again later"}
	}

	s.backend.logger.Info("message relayed", "ip", s.remoteIP, "user", maskEmail(s.username), "recipients", len(s.to))
	return nil
}

func (s *Session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *Session) Logout() error {
	return nil
}

// addressMatches compares an envelope address to the authenticated username
// case-insensitively, since SMTP local-parts are conventionally treated as
// case-insensitive by consumer mail providers such as UOL.
func addressMatches(addr, username string) bool {
	return normalizeAddr(addr) == normalizeAddr(username)
}

func normalizeAddr(addr string) string {
	b := []byte(addr)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func remoteIP(c *smtp.Conn) string {
	addr := c.Conn().RemoteAddr()
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

// newSTARTTLSServer returns a Server listening in plaintext that offers
// STARTTLS (RFC 3207) — the standard "submission" pattern on port 587.
func NewSTARTTLSServer(cfg *Config, backend *Backend, tlsConfig *tls.Config) *smtp.Server {
	s := smtp.NewServer(backend)
	s.Addr = cfg.ListenAddrSTARTTLS
	s.Domain = cfg.Domain
	s.TLSConfig = tlsConfig
	s.MaxMessageBytes = cfg.MaxMessageBytes
	s.MaxRecipients = cfg.MaxRecipients
	s.ReadTimeout = cfg.ReadTimeout
	s.WriteTimeout = cfg.WriteTimeout
	s.AllowInsecureAuth = false
	return s
}

// newImplicitTLSServer returns a Server that expects TLS from the first
// byte (RFC 8314 "SMTPS") — the legacy-but-still-common pattern on port 465.
func NewImplicitTLSServer(cfg *Config, backend *Backend, tlsConfig *tls.Config) *smtp.Server {
	s := smtp.NewServer(backend)
	s.Addr = cfg.ListenAddrImplicit
	s.Domain = cfg.Domain
	s.TLSConfig = tlsConfig
	s.MaxMessageBytes = cfg.MaxMessageBytes
	s.MaxRecipients = cfg.MaxRecipients
	s.ReadTimeout = cfg.ReadTimeout
	s.WriteTimeout = cfg.WriteTimeout
	s.AllowInsecureAuth = false
	return s
}
