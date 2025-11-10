package usecase

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/GoArmGo/MediaApp/internal/core/ports"
	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/google/uuid"
)

// photoUseCase implements PhotoUseCase
type photoUseCase struct {
	photoStorage ports.PhotoStorage
	userStorage  ports.UserStorage
	photoFetcher PhotoFetcher
	fileStorage  FileStorage
}

// NewPhotoUseCase создает новый экземпляр PhotoUseCase
// принимает реализации портов PhotoStorage и PhotoFetcher
func NewPhotoUseCase(
	photoStorage ports.PhotoStorage,
	userStorage ports.UserStorage,
	photoFetcher PhotoFetcher,
	fileStorage FileStorage,
) PhotoUseCase {
	return &photoUseCase{
		photoStorage: photoStorage,
		userStorage:  userStorage,
		photoFetcher: photoFetcher,
		fileStorage:  fileStorage,
	}
}

// GetOrCreatePhotoByUnsplashID получает фото по его Unsplash ID
// Сначала ищет в локальной бд. Если не найдено, получает из Unsplash API,
// загружает в S3, сохраняет в бд и возвращает
func (uc *photoUseCase) GetOrCreatePhotoByUnsplashID(ctx context.Context, unsplashID string) (*domain.Photo, error) {
	// 1. Попытка получить фото из собственной базы данных
	photo, err := uc.photoStorage.GetPhotosByUnsplashIDFromDB(ctx, unsplashID)

	if err != nil && err != sql.ErrNoRows { // Проверяем на ошибку, кроме "нет строк"
		return nil, fmt.Errorf("usecase: ошибка при получении фото из БД по Unsplash ID: %w", err)
	}
	if photo != nil {
		// Фото найдено в бд, возвращаем его
		log.Printf("usecase: Фото с Unsplash ID %s найдено в локальной БД (ID: %s).", unsplashID, photo.ID)
		return photo, nil
	}

	// 2. Если фото не найдено в бд, получаем его из Unsplash API
	log.Printf("usecase: Фото с Unsplash ID %s не найдено в БД. Получаем из Unsplash API...", unsplashID)

	unsplashPhoto, err := uc.photoFetcher.FetchPhotoByIDFromExternal(ctx, unsplashID)
	if err != nil {
		return nil, fmt.Errorf("usecase: ошибка при получении фото из Unsplash API по ID %s: %w", unsplashID, err)
	}
	if unsplashPhoto == nil {
		return nil, fmt.Errorf("usecase: фото с Unsplash ID %s не найдено во внешнем API", unsplashID)
	}

	// 3. Скачиваем оригинальное фото и загружаем его в S3
	log.Printf("usecase: Скачиваем оригинальное фото с Unsplash URL: %s", unsplashPhoto.OriginalURL)
	resp, err := http.Get(unsplashPhoto.OriginalURL)
	if err != nil {
		return nil, fmt.Errorf("usecase: ошибка при скачивании фото с Unsplash URL %s: %w", unsplashPhoto.OriginalURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usecase: неуспешный статус при скачивании фото с Unsplash: %s", resp.Status)
	}

	// Используем resp.Body напрямую как io.Reader
	fileStream := resp.Body

	// Определяем Content-Type для S3
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Генерируем уникальный ключ для S3 на основе UnsplashID или нашего внутреннего ID
	// Используем UnsplashID, так как это уникальный идентификатор фото во внешней системе,
	// и это упрощает его связывание с файлом в S3
	s3Key := fmt.Sprintf("unsplash-photos/%s", unsplashPhoto.UnsplashID) // Можно добавить расширение: ".jpg"

	s3URL, err := uc.fileStorage.UploadFile(ctx, s3Key, fileStream, contentType)
	if err != nil {
		return nil, fmt.Errorf("usecase: ошибка загрузки фото %s в S3: %w", unsplashPhoto.UnsplashID, err)
	}
	unsplashPhoto.S3URL = s3URL // Сохраняем полученный S3 URL

	// 4. Сохраняем полученное и обработанное фото в собственной бд
	// photo.UserID будет установлен в SavePhoto
	systemUserID, err := uc.userStorage.GetOrCreateSystemUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("usecase: ошибка при сохранении фото %s в локальной БД: %w", unsplashPhoto.ID, err)
	}

	unsplashPhoto.UserID = systemUserID

	err = uc.photoStorage.SavePhoto(ctx, unsplashPhoto)
	if err != nil {
		return nil, fmt.Errorf("usecase: ошибка при сохранении фото %s в локальной БД: %w", unsplashPhoto.ID, err)
	}

	log.Printf("usecase: Фото с Unsplash ID %s успешно получено из API, (потенциально) загружено в S3 и сохранено в БД (ID: %s).", unsplashID, unsplashPhoto.ID)
	return unsplashPhoto, nil
}

// SearchAndSavePhotos ищет фото по запросу пользователя во внешнем API, сохраняет их в бд
// и возвращает список сохраненных фото
func (uc *photoUseCase) SearchAndSavePhotos(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error) {

	// Устанавливаем значение по умолчанию, если perPage не указан или равен 0
	if perPage <= 0 {
		perPage = 3
	}
	if page <= 0 {
		page = 1
	}

	// 1. Ищем фото во внешнем API (Unsplash)
	log.Printf("usecase: Поиск фото по запросу '%s' во внешнем API (страница %d, на страницу %d)...", query, page, perPage)
	externalPhotos, err := uc.photoFetcher.SearchPhotosFromExternal(ctx, query, page, perPage)

	if err != nil {
		return nil, fmt.Errorf("usecase: ошибка при поиске фото во внешнем API: %w", err)
	}
	if len(externalPhotos) == 0 {
		log.Printf("usecase: Поиск по запросу '%s' не дал результатов во внешнем API.", query)
		return []domain.Photo{}, nil
	}

	var savedPhotos []domain.Photo
	// 2. Сохраняем каждое найденное фото в нашей бд и S3
	systemUserID, err := uc.userStorage.GetOrCreateSystemUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("usecase: не удалось получить или создать системного пользователя для пачки фото: %w", err)
	}

	for _, photo := range externalPhotos {
		// Избегаем дублирования: проверяем, существует ли уже фото по UnsplashID
		existingPhoto, err := uc.photoStorage.GetPhotosByUnsplashIDFromDB(ctx, photo.UnsplashID)
		if err != nil && err != sql.ErrNoRows {
			log.Printf("usecase: ошибка при проверке существования фото с Unsplash ID %s в БД: %v", photo.UnsplashID, err)
			continue // пропускаем это фото, если нет ошибки "нет строк"
		}
		if existingPhoto != nil {
			log.Printf("usecase: Фото с Unsplash ID %s уже существует в БД (ID: %s), пропускаем сохранение.",
				photo.UnsplashID, existingPhoto.ID)
			savedPhotos = append(savedPhotos, *existingPhoto) // добавляем существующее фото в список возвращаемых
			continue
		}

		// Скачиваем оригинальное фото с Unsplash
		log.Printf("usecase: Скачиваем оригинальное фото с Unsplash URL: %s", photo.OriginalURL)
		resp, err := http.Get(photo.OriginalURL)
		if err != nil {
			log.Printf("usecase: ошибка при скачивании фото с Unsplash URL %s: %v", photo.OriginalURL, err)
			continue // Пропускаем это фото, если не удалось скачать
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("usecase: неуспешный статус при скачивании фото с Unsplash URL %s: %s", photo.OriginalURL, resp.Status)
			continue // Пропускаем, если статус не 200 OK
		}

		fileStream := resp.Body

		// Определяем Content-Type для S3
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// Генерируем уникальный ключ для S3
		s3Key := fmt.Sprintf("unsplash-photos/%s", photo.UnsplashID)

		s3URL, err := uc.fileStorage.UploadFile(ctx, s3Key, fileStream, contentType)
		if err != nil {
			log.Printf("usecase: ошибка загрузки фото %s в S3: %v", photo.UnsplashID, err)
			continue // пропускаем, если не удалось загрузить в S3
		}

		photo.S3URL = s3URL

		photo.UserID = systemUserID

		// Сохраняем полученное и обработанное фото в собственной базе данных
		err = uc.photoStorage.SavePhoto(ctx, &photo)
		if err != nil {
			log.Printf("usecase: ошибка сохранения фото %s (Unsplash ID: %s) в БД: %v", photo.ID, photo.UnsplashID, err)
			continue // Продолжаем цикл, даже если одно фото не сохранилось
		}
		savedPhotos = append(savedPhotos, photo)
	}

	log.Printf("usecase: Поиск по запросу '%s' завершен. Сохранено %d фото из %d найденных во внешнем API.", query, len(savedPhotos), len(externalPhotos))
	return savedPhotos, nil
}

// GetPhotoDetailsFromDB получает детали фото из бд по нашему внутреннему ID
func (uc *photoUseCase) GetPhotoDetailsFromDB(ctx context.Context, id uuid.UUID) (*domain.Photo, error) {
	photo, err := uc.photoStorage.GetPhotoByIDFromDB(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("usecase: фото с ID %s не найдено в БД", id)
		}
		return nil, fmt.Errorf("usecase: ошибка при получении фото из БД по ID %s: %w", id, err)
	}
	log.Printf("usecase: Фото с ID %s успешно получено из БД.", id)
	return photo, nil
}

// GetRecentPhotosFromDB получает последние фото из бд с пагинацией
func (uc *photoUseCase) GetRecentPhotosFromDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {
	photos, err := uc.photoStorage.ListPhotosInDB(ctx, page, perPage)
	if err != nil {
		return nil, fmt.Errorf("usecase: ошибка при получении последних фото из БД: %w", err)
	}
	log.Printf("usecase: Получено %d последних фото из БД (страница %d, на страницу %d).", len(photos), page, perPage)
	return photos, nil
}
