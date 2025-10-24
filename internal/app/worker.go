package app

import (
	"context"
	"fmt"
	"log"
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
) error {
	log.Println("Воркер запущен. Ожидание сообщений в очереди RabbitMQ...")

	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()

	// Определяем функцию-обработчик для сообщений RabbitMQ
	messageHandler := func(ctx context.Context, payload payloads.PhotoSearchPayload) error {
		log.Printf("Worker: Обработка задачи: Поиск '%s', страница %d, на странице %d", payload.Query, payload.Page, payload.PerPage)

		// Вызываем PhotoUseCase для выполнения реальной работы
		_, err := photoUseCase.SearchAndSavePhotos(ctx, payload.Query, payload.Page, payload.PerPage)
		if err != nil {
			log.Printf("Worker: Ошибка при обработке задачи %v: %v", payload, err)
			return err
		}
		log.Printf("Worker: Задача успешно обработана: Поиск '%s', страница %d, на странице %d", payload.Query, payload.Page, payload.PerPage)
		return nil
	}

	// Запускаем потребление сообщений
	err := photoSearchConsumer.StartConsumingPhotoSearchRequests(workerCtx, messageHandler)
	if err != nil {
		//log.Fatalf("Worker: Ошибка при запуске потребителя RabbitMQ: %v", err)
		return fmt.Errorf("ошибка при запуске потребителя RabbitMQ: %w", err)
	}

	// Graceful Shutdown для воркера
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Worker: Получен сигнал завершения. Завершаем работу воркера...")

	cancelWorker()

	time.Sleep(2 * time.Second) // Небольшая задержка, чтобы логи успели выйти
	log.Println("Worker: Воркер успешно завершил работу.")

	return nil
}
