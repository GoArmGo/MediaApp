package app

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/core/ports"
	"github.com/GoArmGo/MediaApp/internal/usecase"
	"github.com/jmoiron/sqlx"
)

type App struct {
	Config               *config.Config
	Logger               *slog.Logger
	db                   *sqlx.DB
	photoUseCase         usecase.PhotoUseCase
	photoSearchPublisher ports.PhotoSearchPublisher
	photoSearchConsumer  ports.PhotoSearchConsumer
	uploadLimiter        chan struct{}
}

func NewApp(cfg *config.Config,
	Logger *slog.Logger,
	db *sqlx.DB,
	photoUseCase usecase.PhotoUseCase,
	photoSearchPublisher ports.PhotoSearchPublisher,
	photoSearchConsumer ports.PhotoSearchConsumer,
	uploadLimiter chan struct{}) *App {
	return &App{
		db:                   db,
		Logger:               Logger,
		photoUseCase:         photoUseCase,
		photoSearchPublisher: photoSearchPublisher,
		photoSearchConsumer:  photoSearchConsumer,
		uploadLimiter:        uploadLimiter,
	}
}

func (a *App) Run(ctx context.Context, mode *string) error {
	// канал для graceful shutdown
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a.Logger.Info("application starting", "mode", *mode)

	var err error

	switch *mode {
	case "server":
		a.Logger.Info("starting server mode")
		err = runServer(ctx, a.Config, a.photoUseCase, a.photoSearchPublisher, a.uploadLimiter, a.Logger)

	case "worker":
		a.Logger.Info("starting worker mode")
		err = runWorker(ctx, a.Config, a.photoUseCase, a.photoSearchConsumer, a.Logger)

	default:
		err = fmt.Errorf("неизвестный режим: %s (используйте 'server' или 'worker')", mode)
		a.Logger.Error("invalid mode", "mode", *mode, "error", err)
	}

	if err != nil {
		a.Logger.Error("application run error", "error", err)
		return err
	}

	// ожидаем сигнал завершения
	<-ctx.Done()
	a.Logger.Info("shutdown signal received")

	// аккуратно закрываем ресурсы
	if closeErr := a.Shutdown(); closeErr != nil {
		a.Logger.Error("shutdown error", "error", closeErr)
	} else {
		a.Logger.Info("shutdown completed successfully")
	}

	a.Logger.Info("application stopped gracefully")
	return nil
}

// Shutdown закрывает все ресурсы приложения
func (a *App) Shutdown() error {
	if a.db != nil {
		a.Logger.Info("closing database connection")
		if err := a.db.Close(); err != nil {
			a.Logger.Error("failed to close database", "error", err)
			return fmt.Errorf("ошибка закрытия БД: %w", err)
		}
	}

	// если publisher/consumer имеют методы Close — вызываем их
	if closer, ok := a.photoSearchPublisher.(interface{ Close() error }); ok {
		a.Logger.Info("closing photo search publisher")
		if err := closer.Close(); err != nil {
			a.Logger.Error("failed to close publisher", "error", err)
		}
	}
	if closer, ok := a.photoSearchConsumer.(interface{ Close() error }); ok {
		a.Logger.Info("closing photo search consumer")
		if err := closer.Close(); err != nil {
			a.Logger.Error("failed to close consumer", "error", err)
		}
	}

	return nil
}

// LoggerIns возвращает основной экземпляр slog.Logger приложения
func (a *App) LoggerIns() *slog.Logger {
	return a.Logger
}
