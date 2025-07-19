package ports

import (
	"context"

	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/google/uuid"
)

// PhotoStorage определяет методы для взаимодействия с хранилищем фотографий
type PhotoStorage interface {
	SavePhoto(ctx context.Context, photo *domain.Photo) error
	GetPhotoByIDFromDB(ctx context.Context, id uuid.UUID) (*domain.Photo, error)
	GetPhotosByUnsplashIDFromDB(ctx context.Context, unsplashID string) (*domain.Photo, error)
	SearchPhotosInDB(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error)
	ListAllPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error)
	ListPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error)
}

// UserStorage определяет методы для взаимодействия с хранилищем пользователей
type UserStorage interface {
	GetOrCreateSystemUser(ctx context.Context) (uuid.UUID, error)
}
