package config

import (
	"fmt"
	"os"

	// Будет нужен для ручного парсинга bool из строки
	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

// Config хранит все конфигурационные параметры приложения.
type Config struct {
	DatabaseURL    string `env:"DATABASE_URL,required"`
	ServerPort     string `env:"SERVER_PORT"` // УБРАН "default=8080"
	UnsplashAPIKey string `env:"UNSPLASH_API_KEY,required"`

	// Настройки для MinIO
	MinioEndpoint        string `env:"MINIO_ENDPOINT,required"`
	MinioAccessKeyID     string `env:"MINIO_ACCESS_KEY_ID,required"`
	MinioSecretAccessKey string `env:"MINIO_SECRET_ACCESS_KEY,required"`
	MinioUseSSL          bool   `env:"MINIO_USE_SSL"` // УБРАН "default=false"
	MinioBucketName      string `env:"MINIO_BUCKET_NAME,required"`

	MinioRegion string `env:"MINIO_REGION,required"` // <-- ДОБАВЬТЕ ЭТУ СТРОКУ

	// --- НОВЫЙ БЛОК ДЛЯ RABBITMQ ---
	RabbitMQ struct {
		RabbitMQURL       string `env:"RABBITMQ_URL,required"`
		RabbitMQQueueName string `env:"RABBITMQ_QUEUE_NAME" envDefault:"photo_search_queue"`
	}
	// --- КОНЕЦ НОВОГО БЛОКА ---
}

// LoadConfig загружает конфигурацию из переменных окружения.
// В режиме разработки пытается загрузить .env файл.
func LoadConfig() (*Config, error) {
	if _, err := os.Stat(".env"); !os.IsNotExist(err) {
		if err := godotenv.Load(); err != nil {
			return nil, fmt.Errorf("ошибка загрузки .env файла: %w", err)
		}
	}

	cfg := Config{}
	// Инициализируем структуру, но без учета default= из тегов
	// env.Parse все еще обрабатывает "required" и парсит типы (например, bool)
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("ошибка парсинга конфигурации из окружения: %w", err)
	}

	// Вручную устанавливаем значения по умолчанию для тех полей, где они нужны
	if cfg.ServerPort == "" {
		cfg.ServerPort = "8080"
	}

	// Для MinioUseSSL: если в env переменной "MINIO_USE_SSL" нет значения,
	// то env.Parse оставит cfg.MinioUseSSL как `false` (дефолтное значение для `bool`).
	// Это как раз то, что нам нужно (`default=false`).
	// Дополнительный код нужен, только если бы дефолт был `true`,
	// или если бы мы хотели парсить из строки, если переменная пустая, но это не наш случай.
	// `env.Parse` уже справится, если MINIO_USE_SSL="true" или "false".

	return &cfg, nil
}
