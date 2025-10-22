package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresStorage struct {
	db *sqlx.DB
}

func NewPostgresStorage(db *sqlx.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

// SavePhoto сохраняет метаданные фотографии в базе данных с помощью GORM
func (s *PostgresStorage) SavePhoto(ctx context.Context, photo *domain.Photo) error {

	if photo.ID == uuid.Nil {
		photo.ID = uuid.New()
	}

	query := `
	INSERT INTO photos (id, unsplash_id, title, description, author_name, width, height, url_full, url_thumb, uploaded_at, created_at, updated_at)
	VALUES (:id, :unsplash_id, :title, :description, :author_name, :width, :height, :url_full, :url_thumb, :uploaded_at, :created_at, :updated_at)
	ON CONFLICT (unsplash_id) DO NOTHING
	`

	_, err := s.db.NamedExecContext(ctx, query, photo)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении фото: %w", err)
	}

	log.Printf("[db] Фото %s (Unsplash ID: %s) сохранено в БД.", photo.ID, photo.UnsplashID)
	return nil
}

// GetPhotoByIDFromDB получает детали фото по ID с помощью GORM
func (s *PostgresStorage) GetPhotoByIDFromDB(ctx context.Context, id uuid.UUID) (*domain.Photo, error) {

	var photo domain.Photo
	query := `SELECT * FROM photos WHERE id = $1 LIMIT 1`

	err := s.db.GetContext(ctx, &photo, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка при получении фото по ID: %w", err)
	}

	return &photo, nil
}

// GetPhotosByUnsplashIDFromDB получает фото по Unsplash ID с помощью GORM.
func (s *PostgresStorage) GetPhotosByUnsplashIDFromDB(ctx context.Context, unsplashID string) (*domain.Photo, error) {

	var photo domain.Photo
	query := `SELECT * FROM photos WHERE unsplash_id = $1 LIMIT 1`

	err := s.db.GetContext(ctx, &photo, query, unsplashID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("ошибка при получении фото по Unsplash ID: %w", err)
	}

	return &photo, nil
}

// SearchPhotosInDB ищет фото с помощью GORM.
func (s *PostgresStorage) SearchPhotosInDB(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error) {

	offset := (page - 1) * perPage
	q := `
	SELECT * FROM photos
	WHERE LOWER(title) LIKE LOWER($1)
	   OR LOWER(description) LIKE LOWER($1)
	   OR LOWER(author_name) LIKE LOWER($1)
	ORDER BY uploaded_at DESC
	LIMIT $2 OFFSET $3
	`

	searchTerm := "%" + query + "%"
	var photos []domain.Photo

	if err := s.db.SelectContext(ctx, &photos, q, searchTerm, perPage, offset); err != nil {
		return nil, fmt.Errorf("ошибка при поиске фото: %w", err)
	}

	return photos, nil
}

// ListAllPhotosInDB получает все фото с помощью GORM
func (s *PostgresStorage) ListAllPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {

	offset := (page - 1) * perPage
	q := `
	SELECT * FROM photos
	ORDER BY uploaded_at DESC
	LIMIT $1 OFFSET $2
	`

	var photos []domain.Photo
	if err := s.db.SelectContext(ctx, &photos, q, perPage, offset); err != nil {
		return nil, fmt.Errorf("ошибка при получении всех фото: %w", err)
	}

	return photos, nil
}

// ListPhotosInDB получает список фотографий из БД с пагинацией с помощью GORM
func (s *PostgresStorage) ListPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {

	offset := (page - 1) * perPage
	q := `
	SELECT * FROM photos
	ORDER BY created_at DESC
	LIMIT $1 OFFSET $2
	`

	var photos []domain.Photo
	if err := s.db.SelectContext(ctx, &photos, q, perPage, offset); err != nil {
		return nil, fmt.Errorf("ошибка при получении списка фото: %w", err)
	}

	return photos, nil
}
