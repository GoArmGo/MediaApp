package config

import (
	"fmt"
	"os"

	// Будет нужен для ручного парсинга bool из строки
	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

// Config хранит все конфигурационные параметры приложения
type Config struct {
	DatabaseURL    string `env:"DATABASE_URL,required"`
	ServerPort     string `env:"SERVER_PORT"`
	UnsplashAPIKey string `env:"UNSPLASH_API_KEY,required"`

	// Настройки для MinIO
	MinioEndpoint        string `env:"MINIO_ENDPOINT,required"`
	MinioAccessKeyID     string `env:"MINIO_ACCESS_KEY_ID,required"`
	MinioSecretAccessKey string `env:"MINIO_SECRET_ACCESS_KEY,required"`
	MinioUseSSL          bool   `env:"MINIO_USE_SSL"`
	MinioBucketName      string `env:"MINIO_BUCKET_NAME,required"`

	MinioRegion string `env:"MINIO_REGION,required"`

	RabbitMQ struct {
		RabbitMQURL       string `env:"RABBITMQ_URL,required"`
		RabbitMQQueueName string `env:"RABBITMQ_QUEUE_NAME" envDefault:"photo_search_queue"`
	}
}

// LoadConfig загружает конфигурацию из переменных окружения
// В режиме разработки пытается загрузить .env файл
func LoadConfig() (*Config, error) {
	if _, err := os.Stat(".env"); !os.IsNotExist(err) {
		if err := godotenv.Load(); err != nil {
			return nil, fmt.Errorf("ошибка загрузки .env файла: %w", err)
		}
	}

	cfg := Config{}
	// Инициализируем структуру, но без учета default= из тегов
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("ошибка парсинга конфигурации из окружения: %w", err)
	}

	// Вручную устанавливаем значения по умолчанию для тех полей, где они нужны
	if cfg.ServerPort == "" {
		cfg.ServerPort = "8080"
	}

	return &cfg, nil
}
