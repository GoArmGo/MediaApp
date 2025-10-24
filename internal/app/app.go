package app

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/core/ports"
	"github.com/GoArmGo/MediaApp/internal/usecase"
	"github.com/jmoiron/sqlx"
)

type App struct {
	Config               *config.Config
	db                   *sqlx.DB
	photoUseCase         usecase.PhotoUseCase
	photoSearchPublisher ports.PhotoSearchPublisher
	photoSearchConsumer  ports.PhotoSearchConsumer
	uploadLimiter        chan struct{}
}

func NewApp(cfg *config.Config,
	db *sqlx.DB,
	photoUseCase usecase.PhotoUseCase,
	photoSearchPublisher ports.PhotoSearchPublisher,
	photoSearchConsumer ports.PhotoSearchConsumer,
	uploadLimiter chan struct{}) *App {
	return &App{
		db:                   db,
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

	log.Printf("[app] Запуск в режиме: %s", mode)

	var err error

	switch *mode {
	case "server":
		err = runServer(ctx, a.Config, a.photoUseCase, a.photoSearchPublisher, a.uploadLimiter)

	case "worker":
		err = runWorker(ctx, a.Config, a.photoUseCase, a.photoSearchConsumer)

	default:
		err = fmt.Errorf("неизвестный режим: %s (используйте 'server' или 'worker')", mode)
	}

	if err != nil {
		return err
	}

	// ожидаем сигнал завершения
	<-ctx.Done()
	log.Println("[app] Завершение работы...")

	// аккуратно закрываем ресурсы
	if closeErr := a.Shutdown(); closeErr != nil {
		log.Printf("[app] ошибка при завершении: %v", closeErr)
	}

	log.Println("[app] Завершено корректно.")
	return nil
}

// Shutdown закрывает все ресурсы приложения
func (a *App) Shutdown() error {
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			return fmt.Errorf("ошибка закрытия БД: %w", err)
		}
	}

	// если publisher/consumer имеют методы Close — вызываем их
	if closer, ok := a.photoSearchPublisher.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
	if closer, ok := a.photoSearchConsumer.(interface{ Close() error }); ok {
		_ = closer.Close()
	}

	return nil
}
