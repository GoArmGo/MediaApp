package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/GoArmGo/MediaApp/internal/di"
)

func main() {

	mode := flag.String("mode", "server", "Режим запуска приложения: server или worker")
	flag.Parse()

	// bootstrap-логгер (используется только на этапе инициализации т.к еще не создал slogger)
	bootstrapLogger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
	)
	bootstrapLogger.Info("starting application", "mode", *mode)

	ctx := context.Background()

	app, err := di.BuildApp()
	if err != nil {
		bootstrapLogger.Error("failed to build app", "error", err)
		os.Exit(1)
	}

	if app == nil {
		bootstrapLogger.Error("app instance is nil — BuildApp returned nil without error?")
		os.Exit(1)
	}

	bootstrapLogger.Info("application initialized successfully")

	slog := app.LoggerIns()
	if slog == nil {
		bootstrapLogger.Error("main logger is nil — app.LoggerIns() returned nil")
		os.Exit(1)
	}

	slog.Info("application using main logger")

	if err := app.Run(ctx, mode); err != nil {
		slog.Error("application run failed", "error", err)
		os.Exit(1)
	}

	slog.Info("application stopped gracefully")
}
