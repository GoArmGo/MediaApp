package di

import (
	"log"

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
	dbClient, err := client.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	// 3. Инициализация хранилищ
	photoStorage := storage.NewPostgresStorage(dbClient.DB)
	userStorage := storage.NewUserStorage(dbClient.DB)

	// 4. Инициализация клиентов внешних сервисов
	unsplashClient := unsplash.NewUnsplashAPIClient(cfg)
	fileStorage, err := minio.NewMinioClient(cfg) // S3 / MinIO адаптер
	if err != nil {
		return nil, err
	}

	// 5. Инициализация RabbitMQ клиента
	rabbitMQClient, err := rabbitmq.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	// 6. Инициализация Publisher / Consumer
	photoSearchPublisher := rabbitMQClient
	if err != nil {
		return nil, err
	}

	photoSearchConsumer := rabbitMQClient
	if err != nil {
		return nil, err
	}

	// 7. Инициализация бизнес-логики (usecases)
	photoUseCase := usecase.NewPhotoUseCase(photoStorage, userStorage, unsplashClient, fileStorage)

	// 8. Создание лимитера загрузок (например, ограничиваем 5 параллельных загрузок)
	uploadLimiter := make(chan struct{}, 5)

	// 9. Сборка итогового приложения
	application := app.NewApp(
		cfg,
		slogger,
		dbClient.DB,
		photoUseCase,
		photoSearchPublisher,
		photoSearchConsumer,
		uploadLimiter,
	)

	log.Println("[container] Все зависимости успешно инициализированы.")
	return application, nil
}
