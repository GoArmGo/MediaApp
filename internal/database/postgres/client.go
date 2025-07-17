package postgres

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"gorm.io/gorm"
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

	if err := applyMigrations(cfg.DatabaseURL); err != nil {
		return nil, fmt.Errorf("ошибка при применении миграций: %w", err)
	}

	return &Client{DB: db}, nil
}

// applyMigrations применяет все доступные миграции к бд
func applyMigrations(databaseURL string) error {
	m, err := migrate.New(
		"file://internal/database/postgres/migrations",
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("не удалось создать экземпляр мигратора: %w", err)
	}

	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("ошибка выполнения миграций: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("Миграции не требуются, база данных актуальна.")
	} else {
		log.Println("Миграции успешно применены.")
	}
	return nil
}

func (c *Client) Close() error {
	return c.DB.Close()
}

type PostgresStorage struct {
	db *gorm.DB
}

func NewPostgresStorage(db *gorm.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

// SavePhoto сохраняет метаданные фотографии в базе данных с помощью GORM
func (s *PostgresStorage) SavePhoto(ctx context.Context, photo *domain.Photo) error {

	if photo.ID == uuid.Nil {
		photo.ID = uuid.New()
	}

	result := s.db.WithContext(ctx).Create(photo)
	if result.Error != nil {
		return fmt.Errorf("ошибка при сохранении фото в БД с помощью GORM: %w", result.Error)
	}

	log.Printf("Фото %s (Unsplash ID: %s) успешно сохранено в БД (GORM).", photo.ID, photo.UnsplashID)
	return nil
}

// GetPhotoByIDFromDB получает детали фото по ID с помощью GORM
func (s *PostgresStorage) GetPhotoByIDFromDB(ctx context.Context, id uuid.UUID) (*domain.Photo, error) {
	var photo domain.Photo
	result := s.db.WithContext(ctx).First(&photo, "id = ?", id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка при получении фото по ID из БД с помощью GORM: %w", result.Error)
	}
	return &photo, nil
}

// GetPhotosByUnsplashIDFromDB получает фото по Unsplash ID с помощью GORM.
func (s *PostgresStorage) GetPhotosByUnsplashIDFromDB(ctx context.Context, unsplashID string) (*domain.Photo, error) {
	var photo domain.Photo
	result := s.db.WithContext(ctx).Where("unsplash_id = ?", unsplashID).First(&photo)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка при получении фото по Unsplash ID из БД с помощью GORM: %w", result.Error)
	}
	return &photo, nil
}

// SearchPhotosInDB ищет фото с помощью GORM.
func (s *PostgresStorage) SearchPhotosInDB(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error) {
	var photos []domain.Photo
	// Офсет и лимит для пагинации
	offset := (page - 1) * perPage

	result := s.db.WithContext(ctx).
		Where("LOWER(title) LIKE LOWER(?) OR LOWER(description) LIKE LOWER(?) OR LOWER(author_name) LIKE LOWER(?)",
			"%"+query+"%", "%"+query+"%", "%"+query+"%").
		Order("uploaded_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&photos)

	if result.Error != nil {
		return nil, fmt.Errorf("ошибка при поиске фото в БД с помощью GORM: %w", result.Error)
	}
	return photos, nil
}

// ListAllPhotosInDB получает все фото с помощью GORM
func (s *PostgresStorage) ListAllPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {
	var photos []domain.Photo
	offset := (page - 1) * perPage

	result := s.db.WithContext(ctx).
		Order("uploaded_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&photos)

	if result.Error != nil {
		return nil, fmt.Errorf("ошибка при получении всех фото из БД с помощью GORM: %w", result.Error)
	}
	return photos, nil
}

// ListPhotosInDB получает список фотографий из БД с пагинацией с помощью GORM
func (s *PostgresStorage) ListPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {
	var photos []domain.Photo
	offset := (page - 1) * perPage

	result := s.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&photos)

	if result.Error != nil {
		return nil, fmt.Errorf("ошибка при получении списка фото из БД с помощью GORM: %w", result.Error)
	}
	return photos, nil
}
