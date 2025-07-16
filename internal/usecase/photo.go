// internal/usecase/photo.go
package usecase

import (
	"context"
	"io"

	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/google/uuid"
)

// PhotoFetcher определяет интерфейс для получения данных о фотографиях из внешних источников (например, Unsplash API).
// Этот Fetcher будет принимать данные от Unsplash и маппить их в нашу внутреннюю доменную модель Photo.
type PhotoFetcher interface {
	// FetchPhotoByIDFromExternal возвращает ОДНУ Photo (из нашей БД), полученную по ID из Unsplash.
	// Возможно, он сначала сходит на Unsplash, получит данные, сохранит их в БД, а затем вернет.
	FetchPhotoByIDFromExternal(ctx context.Context, unsplashID string) (*domain.Photo, error)

	// SearchPhotosFromExternal ищет фото во внешнем источнике и возвращает список наших доменных Photo.
	SearchPhotosFromExternal(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error)

	// ListNewPhotosFromExternal получает новые фото из внешнего источника и возвращает список наших доменных Photo.
	ListNewPhotosFromExternal(ctx context.Context, page, perPage int) ([]domain.Photo, error)
}

// PhotoStorage определяет интерфейс для взаимодействия с нашим хранилищем фотографий (PostgreSQL + S3).
// Это "порт" для сохранения и получения наших доменных Photo.
type PhotoStorage interface {
	// SavePhoto сохраняет фото в нашей базе данных и S3.
	SavePhoto(ctx context.Context, photo *domain.Photo) error

	// GetPhotoByIDFromDB получает фото из нашей базы данных по нашему внутреннему ID.
	GetPhotoByIDFromDB(ctx context.Context, id uuid.UUID) (*domain.Photo, error)

	// GetPhotosByUnsplashIDFromDB получает фото из нашей базы данных по ID от Unsplash.
	GetPhotosByUnsplashIDFromDB(ctx context.Context, unsplashID string) (*domain.Photo, error)

	// SearchPhotosInDB ищет фото в нашей базе данных.
	SearchPhotosInDB(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error)

	// ListAllPhotosInDB получает все фото из нашей базы данных (например, для главной страницы).
	ListAllPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error)

	ListPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error)
}

// FileStorage определяет интерфейс для работы с файловым хранилищем (например, S3).
// Это "порт" для хранения бинарных данных (самих изображений).
type FileStorage interface {
	// UploadFile загружает файл в хранилище и возвращает его публичный URL.
	// `key` - это уникальное имя файла в хранилище (например, UUID фото).
	// `reader` - это источник данных файла (например, тело HTTP-ответа после скачивания).
	// `contentType` - MIME-тип файла (например, "image/jpeg").
	UploadFile(ctx context.Context, key string, reader io.Reader, contentType string) (string, error)

	// DeleteFile удаляет файл из хранилища по его ключу. (Пока не требуется, но полезно для будущего).
	DeleteFile(ctx context.Context, key string) error
}

// PhotoUseCase определяет интерфейс для бизнес-логики работы с фотографиями.
// Этот интерфейс будет реализован Interactor'ом.
type PhotoUseCase interface {
	// GetOrCreatePhotoByUnsplashID ищет фото по ID от Unsplash.
	// Если оно уже есть в нашей БД, возвращает его. Иначе, получает от Unsplash, сохраняет в БД и возвращает.
	GetOrCreatePhotoByUnsplashID(ctx context.Context, unsplashID string) (*domain.Photo, error)

	// SearchAndSavePhotos ищет фото по запросу пользователя.
	// Результаты сохраняются в нашей БД, и возвращается список сохраненных фото.
	SearchAndSavePhotos(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error)

	// GetPhotoDetailsFromDB получает детали фото из нашей БД по нашему внутреннему ID.
	GetPhotoDetailsFromDB(ctx context.Context, id uuid.UUID) (*domain.Photo, error)

	// GetRecentPhotosFromDB получает последние фото из нашей БД.
	GetRecentPhotosFromDB(ctx context.Context, page, perPage int) ([]domain.Photo, error)
}
