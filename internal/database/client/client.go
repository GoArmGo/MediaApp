package client

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/jmoiron/sqlx"
)

// Client представляет клиент для взаимодействия с PostgreSQL
// пока остается для golang-migrate, который использует sqlx.DB
type Client struct {
	DB     *sqlx.DB
	logger *slog.Logger
}

// NewClient инициализирует новое подключение к PostgreSQL и применяет миграции
func NewClient(cfg *config.Config, logger *slog.Logger) (*Client, error) {
	start := time.Now()

	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open PostgreSQL connection", "error", err)
		return nil, fmt.Errorf("ошибка открытия соединения с БД: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		logger.Error("failed to ping database", "error", err)
		return nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	logger.Info("PostgreSQL connection established successfully",
		"dsn", cfg.DatabaseURL,
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return &Client{DB: db, logger: logger}, nil
}

func (c *Client) Close() error {
	start := time.Now()
	err := c.DB.Close()
	if err != nil {
		c.logger.Error("failed to close database connection", "error", err)
		return err
	}
	c.logger.Info("database connection closed", "duration_ms", time.Since(start).Milliseconds())
	return nil
}
