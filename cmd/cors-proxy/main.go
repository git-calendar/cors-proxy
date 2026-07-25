package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/git-calendar/cors-proxy/internal/config"
	"github.com/git-calendar/cors-proxy/internal/httpserver"
	"github.com/git-calendar/cors-proxy/internal/proxy"
)

func main() {
	cfg, err := config.Load(context.Background())
	if err != nil {
		panic(err)
	}

	logger := config.NewLogger(cfg.Production)
	slog.SetDefault(logger)

	if cfg.AbuseContact == "" {
		slog.Warn("configure the ABUSE_CONTACT in production environment")
	}

	proxyHandler := proxy.New(proxy.Options{
		AllowedHosts:    cfg.AllowedHosts,
		UpstreamTimeout: cfg.UpstreamTimeout,
		MaxResponseSize: cfg.MaxResponseSize,
		IPSourceHeader:  cfg.IPSourceHeader,
		AbuseContact:    cfg.AbuseContact,
		Logger:          logger,
	})
	server, err := httpserver.New(cfg, proxyHandler, logger)
	if err != nil {
		panic(err)
	}

	logger.Info("configuration", "cfg", cfg)
	logger.Info("running", "address", server.Addr)

	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop

		slog.Info("shutting down")
		server.Shutdown(context.Background())
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		panic(err)
	}
}
