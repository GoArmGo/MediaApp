package client

import (
	"fmt"
	"log"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/jmoiron/sqlx"
)

// Client представляет клиент для взаимодействия с PostgreSQL
// пока остается для golang-migrate, который использует sqlx.DB
type Client struct {
	DB *sqlx.DB
}

// NewClient инициализирует новое подключение к PostgreSQL и применяет миграции
func NewClient(cfg *config.Config) (*Client, error) {
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия соединения с БД: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	log.Println("Успешное подключение к базе данных PostgreSQL (для миграций).")

	return &Client{DB: db}, nil
}

func (c *Client) Close() error {
	return c.DB.Close()
}
