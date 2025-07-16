package main

import (
	"context"
	"flag" // <--- НОВЫЙ ИМПОРТ: для работы с флагами командной строки
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"

	"github.com/GoArmGo/MediaApp/internal/adapter/storage/minio"
	"github.com/GoArmGo/MediaApp/internal/adapter/unsplash"
	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/core/ports"
	"github.com/GoArmGo/MediaApp/internal/database/postgres"
	"github.com/GoArmGo/MediaApp/internal/handler"
	"github.com/GoArmGo/MediaApp/internal/messaging/payloads"
	"github.com/GoArmGo/MediaApp/internal/rabbitmq"
	"github.com/GoArmGo/MediaApp/internal/usecase"
)

// Определяем константу для максимального количества одновременных загрузок
const maxConcurrentUploads = 5

// Добавим константу для таймаута обработки запроса
const requestTimeout = 30 * time.Second

func main() {
	// 0. Парсинг аргументов командной строки для определения режима запуска
	mode := flag.String("mode", "server", "Режим запуска приложения: 'server' (API-сервер) или 'worker' (воркер RabbitMQ)")
	flag.Parse() // Парсим флаги

	// Загрузка переменных окружения из .env файла
	// Этот блок остается в main, так как переменные нужны обоим режимам.
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			log.Printf("WARN: Не удалось загрузить .env файл: %v. Продолжаем, предполагая, что переменные окружения установлены внешне.", err)
		} else {
			log.Println("INFO: .env файл успешно загружен.")
		}
	} else if os.IsNotExist(err) {
		log.Println("INFO: .env файл не найден. Предполагается, что переменные окружения установлены внешне.")
	} else {
		log.Fatalf("ERROR: Ошибка при проверке .env файла: %v", err)
	}

	// 1. Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// 2. Инициализация подключения к базе данных PostgreSQL
	// Клиент БД нужен обоим режимам (серверу для GetRecent/GetPhotoDetails, воркеру для сохранения)
	dbClient, err := postgres.NewClient(cfg)
	if err != nil {
		log.Fatalf("Ошибка подключения к базе данных: %v", err)
	}
	defer func() {
		if dbClient != nil {
			err := dbClient.Close()
			if err != nil {
				log.Printf("Ошибка при закрытии соединения с БД: %v", err)
			}
			log.Println("Соединение с БД закрыто.")
		}
	}()

	// 3. Инициализация S3 клиента (MinIO)
	// Клиент MinIO нужен обоим режимам (серверу для GetOrCreate, воркеру для сохранения)
	fileStorageClient, err := minio.NewMinioClient(cfg)
	if err != nil {
		log.Fatalf("Ошибка инициализации S3 клиента: %v", err)
	}

	// 4. Инициализация Unsplash API клиента
	// Клиент Unsplash нужен обоим режимам (серверу для GetOrCreate, воркеру для поиска)
	unsplashClient := unsplash.NewUnsplashAPIClient(cfg)

	// 5. Инициализация PostgresStorage (реализация PhotoStorage для usecase)
	// Нужен обоим режимам
	photoStorageImpl := postgres.NewPostgresStorage(dbClient.DB)

	// 6. Инициализация Use Case (интерактора)
	// Нужен обоим режимам
	photoUseCase := usecase.NewPhotoUseCase(photoStorageImpl, unsplashClient, fileStorageClient)

	// 7. Инициализация RabbitMQ клиента
	// Клиент RabbitMQ нужен обоим режимам (серверу для публикации, воркеру для потребления)
	rbmqClient, err := rabbitmq.NewClient(cfg)
	if err != nil {
		log.Fatalf("Ошибка инициализации RabbitMQ клиента: %v", err)
	}
	defer rbmqClient.Close()

	// Создаем буферизованный канал, который будет служить семафором.
	// Емкость канала (maxConcurrentUploads) определяет максимальное
	// количество горутин, которые могут одновременно выполнять
	// защищенную операцию.
	uploadLimiter := make(chan struct{}, maxConcurrentUploads)

	// 8. Запуск приложения в зависимости от выбранного режима
	switch *mode {
	case "server":
		log.Println("Запуск приложения в режиме API-сервера.")
		runServer(cfg, photoUseCase, rbmqClient, uploadLimiter)
	case "worker":
		log.Println("Запуск приложения в режиме воркера.")
		runWorker(cfg, photoUseCase, rbmqClient) // Передаем все необходимые зависимости
	default:
		log.Fatalf("Неизвестный режим приложения: %s. Используйте 'server' или 'worker'.", *mode)
	}
}

// runServer запускает HTTP-сервер и логику публикации сообщений.
func runServer(
	cfg *config.Config,
	photoUseCase usecase.PhotoUseCase,
	photoSearchPublisher ports.PhotoSearchPublisher, // rbmqClient будет передан сюда
	uploadLimiter chan struct{},
) {
	photoHandler := handler.NewPhotoHandler(photoUseCase, photoSearchPublisher, uploadLimiter)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(requestTimeout))

	r.Get("/photos/{unsplashID}", photoHandler.GetOrCreatePhotoByUnsplashID)
	r.Get("/photos/search", photoHandler.SearchAndSavePhotos)
	r.Get("/photos/recent", photoHandler.GetRecentPhotosFromDB)
	r.Get("/photos/{id}", photoHandler.GetPhotoDetailsFromDB)

	serverAddr := fmt.Sprintf(":%s", cfg.ServerPort)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: r,
	}

	go func() {
		log.Printf("Сервер запущен на %s", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка при запуске сервера: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM) // регаем канал на прием сис сигналов из|терминала|докера|

	<-quit
	log.Println("Получен сигнал завершения. Завершаем работу сервера...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка при завершении работы сервера: %v", err)
	}

	log.Println("Сервер успешно завершил работу.")
}

// runWorker запускает потребителя RabbitMQ и обрабатывает сообщения.
func runWorker(
	cfg *config.Config,
	photoUseCase usecase.PhotoUseCase,
	photoSearchConsumer ports.PhotoSearchConsumer, // <--- ИЗМЕНЕНО: rbmqClient теперь передан как PhotoSearchConsumer
) {
	log.Println("Воркер запущен. Ожидание сообщений в очереди RabbitMQ...")

	// Создаем контекст для воркера, чтобы можно было его корректно остановить.
	// Используем context.WithCancel, чтобы можно было отменить его при получении сигнала.
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker() // Гарантируем отмену контекста при выходе из функции

	// Определяем функцию-обработчик для сообщений RabbitMQ
	messageHandler := func(ctx context.Context, payload payloads.PhotoSearchPayload) error {
		log.Printf("Worker: Обработка задачи: Поиск '%s', страница %d, на странице %d", payload.Query, payload.Page, payload.PerPage)

		// Вызываем PhotoUseCase для выполнения реальной работы
		_, err := photoUseCase.SearchAndSavePhotos(ctx, payload.Query, payload.Page, payload.PerPage)
		if err != nil {
			log.Printf("Worker: Ошибка при обработке задачи %v: %v", payload, err)
			return err // Возвращаем ошибку, чтобы RabbitMQ знал, что нужно Nack
		}
		log.Printf("Worker: Задача успешно обработана: Поиск '%s', страница %d, на странице %d", payload.Query, payload.Page, payload.PerPage)
		return nil // Возвращаем nil, чтобы RabbitMQ знал, что нужно Ack
	}

	// Запускаем потребление сообщений
	err := photoSearchConsumer.StartConsumingPhotoSearchRequests(workerCtx, messageHandler)
	if err != nil {
		log.Fatalf("Worker: Ошибка при запуске потребителя RabbitMQ: %v", err)
	}

	// Graceful Shutdown для воркера
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit // Блокируем горутину воркера до получения сигнала
	log.Println("Worker: Получен сигнал завершения. Завершаем работу воркера...")

	// Отменяем контекст воркера, чтобы остановить потребление сообщений
	cancelWorker()

	// Даем небольшое время на завершение текущих операций, если это необходимо
	// (хотя StartConsumingPhotoSearchRequests уже должен обрабатывать контекст)
	time.Sleep(2 * time.Second) // Небольшая задержка, чтобы логи успели выйти
	log.Println("Worker: Воркер успешно завершил работу.")
}
