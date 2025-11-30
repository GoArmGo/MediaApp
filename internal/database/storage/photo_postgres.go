package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type PostgresStorage struct {
	db     *sqlx.DB
	logger *slog.Logger
}

func NewPostgresStorage(db *sqlx.DB, logger *slog.Logger) *PostgresStorage {
	return &PostgresStorage{db: db, logger: logger}
}

// SavePhoto сохраняет метаданные фотографии в базе данных
func (s *PostgresStorage) SavePhoto(ctx context.Context, photo *domain.Photo) error {
	start := time.Now()

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
		s.logger.Error("failed to save photo", "unsplash_id", photo.UnsplashID, "error", err)
		return fmt.Errorf("ошибка при сохранении фото: %w", err)
	}

	s.logger.Info("photo saved successfully",
		"id", photo.ID,
		"unsplash_id", photo.UnsplashID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return nil
}

// GetPhotoByIDFromDB получает детали фото по ID
func (s *PostgresStorage) GetPhotoByIDFromDB(ctx context.Context, id uuid.UUID) (*domain.Photo, error) {
	start := time.Now()

	var photo domain.Photo
	query := `SELECT * FROM photos WHERE id = $1 LIMIT 1`

	err := s.db.GetContext(ctx, &photo, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Warn("photo not found by id", "id", id)
			return nil, nil
		}
		s.logger.Error("failed to get photo by id", "id", id, "error", err)
		return nil, fmt.Errorf("ошибка при получении фото по ID: %w", err)
	}

	s.logger.Info("photo retrieved by id",
		"id", id,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return &photo, nil
}

// GetPhotosByUnsplashIDFromDB получает фото по Unsplash ID.
func (s *PostgresStorage) GetPhotosByUnsplashIDFromDB(ctx context.Context, unsplashID string) (*domain.Photo, error) {
	start := time.Now()

	var photo domain.Photo
	query := `SELECT * FROM photos WHERE unsplash_id = $1 LIMIT 1`

	err := s.db.GetContext(ctx, &photo, query, unsplashID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Warn("photo not found by unsplash_id", "unsplash_id", unsplashID)
			return nil, nil
		}
		s.logger.Error("failed to get photo by unsplash_id", "unsplash_id", unsplashID, "error", err)
		return nil, fmt.Errorf("ошибка при получении фото по Unsplash ID: %w", err)
	}

	s.logger.Info("photo retrieved by unsplash_id",
		"unsplash_id", unsplashID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return &photo, nil
}

// SearchPhotosInDB ищет фото.
func (s *PostgresStorage) SearchPhotosInDB(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error) {
	start := time.Now()

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
		s.logger.Error("failed to search photos",
			"query", query,
			"page", page,
			"per_page", perPage,
			"error", err,
		)
		return nil, fmt.Errorf("ошибка при поиске фото: %w", err)
	}

	s.logger.Info("photos search completed",
		"query", query,
		"found", len(photos),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return photos, nil
}

// ListAllPhotosInDB получает все фото
func (s *PostgresStorage) ListAllPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {
	start := time.Now()

	offset := (page - 1) * perPage
	q := `
	SELECT * FROM photos
	ORDER BY uploaded_at DESC
	LIMIT $1 OFFSET $2
	`

	var photos []domain.Photo
	if err := s.db.SelectContext(ctx, &photos, q, perPage, offset); err != nil {
		s.logger.Error("failed to list all photos", "page", page, "per_page", perPage, "error", err)
		return nil, fmt.Errorf("ошибка при получении всех фото: %w", err)
	}

	s.logger.Info("listed all photos successfully",
		"page", page,
		"per_page", perPage,
		"count", len(photos),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return photos, nil
}

// ListPhotosInDB получает список фотографий из БД с пагинацией
func (s *PostgresStorage) ListPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {
	start := time.Now()

	offset := (page - 1) * perPage
	q := `
	SELECT * FROM photos
	ORDER BY created_at DESC
	LIMIT $1 OFFSET $2
	`

	var photos []domain.Photo
	if err := s.db.SelectContext(ctx, &photos, q, perPage, offset); err != nil {
		s.logger.Error("failed to list photos", "page", page, "per_page", perPage, "error", err)
		return nil, fmt.Errorf("ошибка при получении списка фото: %w", err)
	}

	s.logger.Info("listed photos successfully",
		"page", page,
		"per_page", perPage,
		"count", len(photos),
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return photos, nil
}
