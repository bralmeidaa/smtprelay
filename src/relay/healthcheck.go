package relay

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// newHealthServer returns an HTTP server, entirely separate from the SMTP
// ports, exposing:
//   - /healthz: liveness. Always 200 while the process is up. Used to decide
//     whether to restart the container.
//   - /readyz: readiness. Does a cheap TCP dial to the UOL upstream. Returns
//     503 when UOL is unreachable, but this must NEVER be wired to a restart
//     policy — a temporary UOL outage is not a reason to kill the relay.
func NewHealthServer(cfg *Config, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		addr := net.JoinHostPort(cfg.UpstreamHost, cfg.UpstreamPort)
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err != nil {
			logger.Warn("readiness check: upstream unreachable", "upstream", cfg.UpstreamHost)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "upstream_unreachable"})
			return
		}
		_ = conn.Close()
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return &http.Server{
		Addr:              cfg.HealthAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func writeJSON(w http.ResponseWriter, status int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func shutdownHealthServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
