package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	loadConfig()
	setupLogger()

	server := newServer()
	slog.Info(fmt.Sprintf("configuration: %+v", *cfg))
	slog.Info("running on " + server.Addr)

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
