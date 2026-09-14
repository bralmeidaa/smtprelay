package relay

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds every runtime setting for the relay, loaded from environment
// variables (populated from Container Apps secrets/env in production).
type Config struct {
	// RelayUser/RelayPassword are the single fixed credential pair the relay
	// accepts from clients. They are also the real UOL account credentials,
	// reused to authenticate the outbound leg to UOL.
	RelayUser     string
	RelayPassword string

	Domain string // public hostname of the relay, e.g. smtps.bratech.me

	ListenAddrSTARTTLS string // e.g. ":587"
	ListenAddrImplicit string // e.g. ":465"
	HealthAddr         string // e.g. ":8080"

	TLSCertFile string
	TLSKeyFile  string

	UpstreamHost string
	UpstreamPort string

	MaxMessageBytes int64
	MaxRecipients   int

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	UpstreamTimeout time.Duration

	RateLimitConnPerMinute int
	RateLimitMsgsPerHour   int
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Domain:                 getEnv("RELAY_DOMAIN", "smtps.bratech.me"),
		ListenAddrSTARTTLS:     getEnv("LISTEN_ADDR_STARTTLS", ":587"),
		ListenAddrImplicit:     getEnv("LISTEN_ADDR_IMPLICIT_TLS", ":465"),
		HealthAddr:             getEnv("HEALTH_ADDR", ":8080"),
		UpstreamHost:           getEnv("SMTP_UPSTREAM_HOST", "smtps.uol.com.br"),
		UpstreamPort:           getEnv("SMTP_UPSTREAM_PORT", "587"),
		MaxMessageBytes:        getEnvInt64("MAX_MESSAGE_SIZE_BYTES", 15*1024*1024),
		MaxRecipients:          getEnvInt("MAX_RECIPIENTS", 20),
		ReadTimeout:            getEnvDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:           getEnvDuration("WRITE_TIMEOUT", 60*time.Second),
		UpstreamTimeout:        getEnvDuration("UPSTREAM_TIMEOUT", 30*time.Second),
		RateLimitConnPerMinute: getEnvInt("RATE_LIMIT_CONN_PER_MINUTE", 30),
		RateLimitMsgsPerHour:   getEnvInt("RATE_LIMIT_MSGS_PER_HOUR", 200),
	}

	cfg.RelayUser = os.Getenv("RELAY_USER")
	cfg.RelayPassword = os.Getenv("RELAY_PASSWORD")
	cfg.TLSCertFile = os.Getenv("TLS_CERT_FILE")
	cfg.TLSKeyFile = os.Getenv("TLS_KEY_FILE")

	if cfg.RelayUser == "" || cfg.RelayPassword == "" {
		return nil, fmt.Errorf("RELAY_USER and RELAY_PASSWORD must be set")
	}
	if cfg.TLSCertFile == "" || cfg.TLSKeyFile == "" {
		return nil, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must be set (TLS is mandatory)")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
