package relay

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/emersion/go-smtp"
)

// Run wires the whole relay together (config, TLS, backend, both SMTP
// listeners and the health server), blocks until SIGINT/SIGTERM, then drains
// connections and exits. It's the single entrypoint cmd/main.go calls.
func Run() int {
	logger := NewLogger()

	cfg, err := LoadConfig()
	if err != nil {
		logger.Error("invalid configuration", "error", err.Error())
		return 1
	}

	tlsCert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		logger.Error("failed to load TLS certificate", "error", err.Error())
		return 1
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	auth := NewAuthenticator(cfg)
	limiter := NewRateLimiter(cfg.RateLimitConnPerMinute, cfg.RateLimitMsgsPerHour)
	backend := NewBackend(cfg, auth, limiter, logger)

	starttlsSrv := NewSTARTTLSServer(cfg, backend, tlsConfig)
	implicitSrv := NewImplicitTLSServer(cfg, backend, tlsConfig)
	healthSrv := NewHealthServer(cfg, logger)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		logger.Info("starting STARTTLS listener", "addr", cfg.ListenAddrSTARTTLS)
		if err := starttlsSrv.ListenAndServe(); err != nil {
			logger.Info("STARTTLS listener stopped", "error", err.Error())
		}
	}()

	go func() {
		defer wg.Done()
		logger.Info("starting implicit TLS listener", "addr", cfg.ListenAddrImplicit)
		if err := implicitSrv.ListenAndServeTLS(); err != nil {
			logger.Info("implicit TLS listener stopped", "error", err.Error())
		}
	}()

	go func() {
		defer wg.Done()
		logger.Info("starting health server", "addr", cfg.HealthAddr)
		if err := healthSrv.ListenAndServe(); err != nil {
			logger.Info("health server stopped", "error", err.Error())
		}
	}()

	waitForShutdown(logger, starttlsSrv, implicitSrv, healthSrv)
	wg.Wait()
	logger.Info("shutdown complete")
	return 0
}

func waitForShutdown(logger *slog.Logger, starttlsSrv, implicitSrv *smtp.Server, healthSrv *http.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutdown signal received, draining connections")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := starttlsSrv.Shutdown(ctx); err != nil {
		logger.Warn("error shutting down STARTTLS listener", "error", err.Error())
	}
	if err := implicitSrv.Shutdown(ctx); err != nil {
		logger.Warn("error shutting down implicit TLS listener", "error", err.Error())
	}
	shutdownHealthServer(healthSrv)
}
