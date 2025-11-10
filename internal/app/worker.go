package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/core/ports"
	"github.com/GoArmGo/MediaApp/internal/messaging/payloads"
	"github.com/GoArmGo/MediaApp/internal/usecase"
)

// runWorker запускает потребителя RabbitMQ и обрабатывает сообщения
func runWorker(
	ctx context.Context,
	cfg *config.Config,
	photoUseCase usecase.PhotoUseCase,
	photoSearchConsumer ports.PhotoSearchConsumer,
	logger *slog.Logger, // ← добавили логгер
) error {
	logger.Info("worker started", "queue", cfg.RabbitMQ.RabbitMQQueueName)

	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	// Определяем функцию-обработчик для сообщений RabbitMQ
	messageHandler := func(ctx context.Context, payload payloads.PhotoSearchPayload) error {
		logger.Info("processing task",
			"query", payload.Query,
			"page", payload.Page,
			"per_page", payload.PerPage,
		)

		// Вызываем PhotoUseCase для выполнения реальной работы
		_, err := photoUseCase.SearchAndSavePhotos(ctx, payload.Query, payload.Page, payload.PerPage)
		if err != nil {
			logger.Error("failed to process task",
				"query", payload.Query,
				"page", payload.Page,
				"per_page", payload.PerPage,
				"error", err,
			)
			return err
		}

		logger.Info("task processed successfully",
			"query", payload.Query,
			"page", payload.Page,
			"per_page", payload.PerPage,
		)
		return nil
	}

	// Запускаем потребление сообщений
	err := photoSearchConsumer.StartConsumingPhotoSearchRequests(workerCtx, messageHandler)
	if err != nil {
		logger.Error("failed to start RabbitMQ consumer", "error", err)
		return fmt.Errorf("ошибка при запуске потребителя RabbitMQ: %w", err)
	}

	// Graceful Shutdown для воркера
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Warn("shutdown signal received, stopping worker...")

	cancelWorker()

	time.Sleep(2 * time.Second) // небольшая задержка для корректного завершения
	logger.Info("worker stopped gracefully")

	return nil
}
