package main

import (
	"fmt"
	"log/slog"
)

func main() {
	loadConfig()
	setupLogger()

	server := newServer()
	slog.Info(fmt.Sprintf("configuration: %+v", *cfg))
	slog.Info("running on " + server.Addr)

	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}
