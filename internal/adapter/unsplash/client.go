// internal/adapter/unsplash/client.go
package unsplash

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config" // Для получения UnsplashAccessKey
	"github.com/GoArmGo/MediaApp/internal/domain" // Для маппинга в domain.Photo

	"github.com/google/uuid" // Для генерации UUID для domain.Photo
)

const (
	baseURL = "https://api.unsplash.com" // Базовый URL для Unsplash API
)

// UnsplashAPIClient представляет клиент для взаимодействия с Unsplash API.
type UnsplashAPIClient struct {
	httpClient *http.Client // HTTP-клиент для выполнения запросов
	accessKey  string       // Ваш Unsplash Access Key
}

// NewUnsplashAPIClient создает новый экземпляр UnsplashAPIClient.
func NewUnsplashAPIClient(cfg *config.Config) *UnsplashAPIClient {
	return &UnsplashAPIClient{
		httpClient: &http.Client{Timeout: 10 * time.Second}, // Устанавливаем таймаут для HTTP-запросов
		accessKey:  cfg.UnsplashAPIKey,
	}
}

// fetchAndMapPhoto выполняет HTTP-запрос к Unsplash и маппит ответ в domain.Photo.
// Это вспомогательная функция, которая используется всеми методами fetcher'а.
func (c *UnsplashAPIClient) fetchAndMapPhoto(endpoint string) (*domain.Photo, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP-запроса: %w", err)
	}

	req.Header.Set("Authorization", "Client-ID "+c.accessKey) // Добавляем заголовок авторизации

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HTTP-запроса к Unsplash: %w", err)
	}
	defer resp.Body.Close() // Важно закрыть тело ответа

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body) // Прочитаем тело ответа для получения деталей ошибки
		return nil, fmt.Errorf("unsplash API вернул статус %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var unsplashPhoto UnsplashPhotoResponse
	if err := json.NewDecoder(resp.Body).Decode(&unsplashPhoto); err != nil {
		return nil, fmt.Errorf("ошибка декодирования JSON ответа Unsplash: %w", err)
	}

	// Маппинг UnsplashPhotoResponse в domain.Photo
	photo := c.mapUnsplashPhotoToDomain(&unsplashPhoto)
	return photo, nil
}

// mapUnsplashPhotoToDomain преобразует UnsplashPhotoResponse в domain.Photo.
func (c *UnsplashAPIClient) mapUnsplashPhotoToDomain(unsplashPhoto *UnsplashPhotoResponse) *domain.Photo {
	// Генерируем новый UUID для нашего внутреннего ID, так как это новое фото для нас.
	newPhotoID := uuid.New()

	// Используем AltDescription, если Description пуст
	description := unsplashPhoto.Description
	if description == "" {
		description = unsplashPhoto.AltDescription
	}

	return &domain.Photo{
		ID:         newPhotoID,
		UnsplashID: unsplashPhoto.ID,
		// UserID будет установлен PhotoUseCase, или мы можем привязать его к системному пользователю здесь.
		// Пока оставим nil/empty, usecase будет решать.
		S3URL:          "",          // S3 URL будет установлен после загрузки в S3, не тут.
		Title:          description, // В качестве заголовка используем описание или alt_description
		Description:    description,
		AuthorName:     unsplashPhoto.User.Name,
		Width:          unsplashPhoto.Width,
		Height:         unsplashPhoto.Height,
		LikesCount:     unsplashPhoto.Likes,
		OriginalURL:    unsplashPhoto.URLs.Full, // Или Regular, в зависимости от предпочтений качества
		UploadedAt:     unsplashPhoto.CreatedAt,
		ViewsCount:     unsplashPhoto.Views,
		DownloadsCount: unsplashPhoto.Downloads,
		Tags:           nil, // Теги из Unsplash часто в другом формате, пока игнорируем
	}
}

// FetchPhotoByIDFromExternal реализует метод PhotoFetcher.
func (c *UnsplashAPIClient) FetchPhotoByIDFromExternal(ctx context.Context, id string) (*domain.Photo, error) {
	endpoint := fmt.Sprintf("%s/photos/%s", baseURL, id)
	return c.fetchAndMapPhoto(endpoint)
}

// SearchPhotosFromExternal реализует метод PhotoFetcher.
func (c *UnsplashAPIClient) SearchPhotosFromExternal(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error) {
	// Строим URL для поиска
	params := url.Values{}
	params.Add("query", query)
	params.Add("page", strconv.Itoa(page))
	params.Add("per_page", strconv.Itoa(perPage))

	endpoint := fmt.Sprintf("%s/search/photos?%s", baseURL, params.Encode())

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP-запроса для поиска: %w", err)
	}
	req.Header.Set("Authorization", "Client-ID "+c.accessKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HTTP-запроса к Unsplash для поиска: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unsplash API поиска вернул статус %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResponse UnsplashSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("ошибка декодирования JSON ответа поиска Unsplash: %w", err)
	}

	var domainPhotos []domain.Photo
	for _, unsplashPhoto := range searchResponse.Results {
		domainPhotos = append(domainPhotos, *c.mapUnsplashPhotoToDomain(&unsplashPhoto))
	}
	return domainPhotos, nil
}

// ListNewPhotosFromExternal реализует метод PhotoFetcher.
func (c *UnsplashAPIClient) ListNewPhotosFromExternal(ctx context.Context, page, perPage int) ([]domain.Photo, error) {
	// Строим URL для получения списка фото (например, /photos эндпоинт)
	params := url.Values{}
	params.Add("page", strconv.Itoa(page))
	params.Add("per_page", strconv.Itoa(perPage))

	endpoint := fmt.Sprintf("%s/photos?%s", baseURL, params.Encode())

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP-запроса для списка фото: %w", err)
	}
	req.Header.Set("Authorization", "Client-ID "+c.accessKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения HTTP-запроса к Unsplash для списка фото: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unsplash API списка фото вернул статус %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var unsplashPhotos []UnsplashPhotoResponse // Список фото напрямую
	if err := json.NewDecoder(resp.Body).Decode(&unsplashPhotos); err != nil {
		return nil, fmt.Errorf("ошибка декодирования JSON ответа списка фото Unsplash: %w", err)
	}

	var domainPhotos []domain.Photo
	for _, unsplashPhoto := range unsplashPhotos {
		domainPhotos = append(domainPhotos, *c.mapUnsplashPhotoToDomain(&unsplashPhoto))
	}
	return domainPhotos, nil
}
