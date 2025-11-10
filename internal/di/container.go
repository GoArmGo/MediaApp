package di

import (
	"github.com/GoArmGo/MediaApp/internal/adapter/storage/minio"
	"github.com/GoArmGo/MediaApp/internal/adapter/unsplash"
	"github.com/GoArmGo/MediaApp/internal/app"
	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/database/client"
	"github.com/GoArmGo/MediaApp/internal/database/storage"
	"github.com/GoArmGo/MediaApp/internal/logger"
	"github.com/GoArmGo/MediaApp/internal/rabbitmq"
	"github.com/GoArmGo/MediaApp/internal/usecase"
)

// BuildApp инициализирует все зависимости и возвращает готовый объект App.
func BuildApp() (*app.App, error) {
	// 1. Загрузка конфигурации
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, err
	}

	slogCfg := logger.SlogConfig{
		Level:  cfg.LogLevel,
		Format: cfg.LogFormat,
	}
	slogger := logger.NewSlog(slogCfg)
	slogger.Info("logger initialized", "level", cfg.LogLevel, "format", cfg.LogFormat)

	// 2. Инициализация PostgreSQL клиента
	slogger.Info("initializing PostgreSQL client", "db-URL", cfg.DatabaseURL)
	dbClient, err := client.NewClient(cfg)
	if err != nil {
		slogger.Error("failed to initialize PostgreSQL client", "error", err)
		return nil, err
	}
	slogger.Info("PostgreSQL client initialized successfully")

	// 3. Инициализация хранилищ
	slogger.Info("initializing storages")
	photoStorage := storage.NewPostgresStorage(dbClient.DB)
	userStorage := storage.NewUserStorage(dbClient.DB)
	slogger.Info("storages initialized successfully")

	// 4. Инициализация клиентов внешних сервисов
	slogger.Info("initializing external clients: Unsplash, MinIO")
	unsplashClient := unsplash.NewUnsplashAPIClient(cfg)
	fileStorage, err := minio.NewMinioClient(cfg) // S3 / MinIO адаптер
	if err != nil {
		slogger.Error("failed to initialize MinIO client", "error", err)
		return nil, err
	}

	// 5. Инициализация RabbitMQ клиента
	slogger.Info("initializing RabbitMQ client", "url", cfg.RabbitMQ.RabbitMQURL)
	rabbitMQClient, err := rabbitmq.NewClient(cfg)
	if err != nil {
		slogger.Error("failed to initialize RabbitMQ client", "error", err)
		return nil, err
	}
	slogger.Info("RabbitMQ client initialized successfully")

	// 6. Инициализация Publisher / Consumer
	slogger.Info("initializing publisher and consumer for photo search")
	photoSearchPublisher := rabbitMQClient
	photoSearchConsumer := rabbitMQClient
	slogger.Info("publisher and consumer initialized")

	// 7. Инициализация бизнес-логики (usecases)
	slogger.Info("initializing usecases")
	photoUseCase := usecase.NewPhotoUseCase(photoStorage, userStorage, unsplashClient, fileStorage)
	slogger.Info("usecases initialized successfully")

	// 8. Создание лимитера загрузок (например, ограничиваем 5 параллельных загрузок)
	slogger.Info("creating upload limiter", "limit", 5)
	uploadLimiter := make(chan struct{}, 5)

	// 9. Сборка итогового приложения
	slogger.Info("building final application instance")
	application := app.NewApp(
		cfg,
		slogger,
		dbClient.DB,
		photoUseCase,
		photoSearchPublisher,
		photoSearchConsumer,
		uploadLimiter,
	)

	slogger.Info("application built successfully — all dependencies initialized")
	return application, nil
}
