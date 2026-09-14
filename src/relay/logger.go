package relay

import (
	"log/slog"
	"os"
	"strings"
)

// NewLogger returns a structured JSON logger writing to stdout. Callers must
// use maskEmail/maskIP helpers below instead of logging raw addresses, and
// must never pass credentials to it.
func NewLogger() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}

// maskEmail keeps the domain but redacts most of the local part, e.g.
// "cliente@uol.com.br" -> "c***e@uol.com.br". Safe to log; never log the
// password under any circumstance, including at debug level.
func maskEmail(addr string) string {
	at := strings.LastIndex(addr, "@")
	if at <= 0 {
		return "***"
	}
	local, domain := addr[:at], addr[at:]
	if len(local) <= 2 {
		return "*" + domain
	}
	return string(local[0]) + strings.Repeat("*", len(local)-2) + string(local[len(local)-1]) + domain
}
