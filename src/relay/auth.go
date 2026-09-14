package relay

import "crypto/subtle"

// Authenticator validates SASL PLAIN credentials against the single fixed
// relay account. It never accepts any other username/password, which is
// what guarantees the relay can't be turned into an open relay by
// misconfiguration: there is exactly one valid identity, period.
type Authenticator struct {
	cfg *Config
}

func NewAuthenticator(cfg *Config) *Authenticator {
	return &Authenticator{cfg: cfg}
}

// Validate returns true only if username and password exactly match the
// configured relay account. Uses constant-time comparison to avoid leaking
// timing information about how much of the credential matched.
func (a *Authenticator) Validate(username, password string) bool {
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(a.cfg.RelayUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), []byte(a.cfg.RelayPassword)) == 1
	return userOK && passOK
}
