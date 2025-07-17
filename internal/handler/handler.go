package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/GoArmGo/MediaApp/internal/core/ports"
	"github.com/GoArmGo/MediaApp/internal/messaging/payloads"
	"github.com/GoArmGo/MediaApp/internal/usecase"
)

// PhotoHandler будет обрабатывать HTTP-запросы, связанные с фотографиями
type PhotoHandler struct {
	photoUseCase         usecase.PhotoUseCase
	photoSearchPublisher ports.PhotoSearchPublisher
	uploadLimiter        chan struct{}
}

// NewPhotoHandler создает новый экземпляр PhotoHandler
func NewPhotoHandler(uc usecase.PhotoUseCase, publisher ports.PhotoSearchPublisher, limiter chan struct{}) *PhotoHandler {
	return &PhotoHandler{
		photoUseCase:         uc,
		photoSearchPublisher: publisher,
		uploadLimiter:        limiter,
	}
}

// respondWithJSON - вспомогательная функция для отправки JSON-ответа
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("handler: ошибка Marshal JSON: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(response)
	if err != nil {
		log.Printf("handler: ошибка записи HTTP-ответа: %v", err)
	}
}

// respondWithError - вспомогательная функция для отправки JSON-ответа с ошибкой
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// --- Методы обработчиков ---

// GetOrCreatePhotoByUnsplashID обрабатывает запрос для получения или создания фото по Unsplash ID
// GET /photos/{unsplashID}
func (h *PhotoHandler) GetOrCreatePhotoByUnsplashID(w http.ResponseWriter, r *http.Request) {

	select {
	case h.uploadLimiter <- struct{}{}:
		defer func() { <-h.uploadLimiter }()
	case <-r.Context().Done():
		log.Printf("handler: Запрос на GetOrCreatePhotoByUnsplashID отменен клиентом: %v", r.Context().Err())
		return
	}

	unsplashID := chi.URLParam(r, "unsplashID")
	if unsplashID == "" {
		respondWithError(w, http.StatusBadRequest, "Unsplash ID не указан")
		return
	}

	log.Printf("handler: Запрос на получение/создание фото по Unsplash ID: %s", unsplashID)
	photo, err := h.photoUseCase.GetOrCreatePhotoByUnsplashID(r.Context(), unsplashID)
	if err != nil {
		log.Printf("handler: Ошибка в GetOrCreatePhotoByUnsplashID: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Ошибка при получении или создании фото")
		return
	}

	respondWithJSON(w, http.StatusOK, photo)
}

// SearchAndSavePhotos обрабатывает запрос для поиска и сохранения фото
// GET /photos/search?query={search_query}&page={page}&per_page={per_page}
func (h *PhotoHandler) SearchAndSavePhotos(w http.ResponseWriter, r *http.Request) {

	select {
	case h.uploadLimiter <- struct{}{}:
		defer func() { <-h.uploadLimiter }()
	case <-r.Context().Done():
		log.Printf("handler: Запрос на SearchAndSavePhotos отменен клиентом: %v", r.Context().Err())
		return
	case <-time.After(1 * time.Second): // таймаут на ожидание семафора
		log.Printf("handler: Превышен таймаут ожидания семафора для SearchAndSavePhotos")
		respondWithError(w, http.StatusServiceUnavailable, "Сервис временно перегружен, попробуйте позже.")
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		respondWithError(w, http.StatusBadRequest, "Параметр 'query' не указан")
		return
	}

	// парсинг параметров пагинации
	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	perPageStr := r.URL.Query().Get("per_page")
	perPage, err := strconv.Atoi(perPageStr)
	if err != nil || perPage <= 0 || perPage > 100 {
		perPage = 3
	}

	log.Printf("handler: Принят асинхронный запрос на поиск фото: '%s', страница %d, на странице %d",
		query, page, perPage)

	// Создаем полезную нагрузку для сообщения RabbitMQ
	payload := payloads.PhotoSearchPayload{
		Query:   query,
		Page:    page,
		PerPage: perPage,
	}

	// Публикуем сообщение в очередь RabbitMQ
	err = h.photoSearchPublisher.PublishPhotoSearchRequest(r.Context(), payload)
	if err != nil {
		log.Printf("handler: Ошибка при публикации сообщения в RabbitMQ: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Ошибка при постановке задачи в очередь")
		return
	}

	// Сразу возвращаем HTTP 202 Accepted, так как задача принята в обработку
	respondWithJSON(w, http.StatusAccepted, map[string]string{
		"message":  "Запрос на поиск и сохранение фото принят в обработку. Результаты будут доступны позже.",
		"query":    query,
		"page":     strconv.Itoa(page),
		"per_page": strconv.Itoa(perPage),
	})
	// без очереди
	// photos, err := h.photoUseCase.SearchAndSavePhotos(r.Context(), query, page, perPage)
	// if err != nil {
	// 	log.Printf("handler: Ошибка в SearchAndSavePhotos: %v", err)
	// 	respondWithError(w, http.StatusInternalServerError, "Ошибка при поиске и сохранении фото")
	// 	return
	// }

	// respondWithJSON(w, http.StatusOK, photos)
}

// GetRecentPhotosFromDB обрабатывает запрос для получения последних фото из бд
// GET /photos/recent?page={page}&per_page={per_page}
func (h *PhotoHandler) GetRecentPhotosFromDB(w http.ResponseWriter, r *http.Request) {

	pageStr := r.URL.Query().Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	perPageStr := r.URL.Query().Get("per_page")
	perPage, err := strconv.Atoi(perPageStr)
	if err != nil || perPage <= 0 || perPage > 100 {
		perPage = 10
	}

	log.Printf("handler: Запрос на получение последних фото из БД: страница %d, на странице %d", page, perPage)
	photos, err := h.photoUseCase.GetRecentPhotosFromDB(r.Context(), page, perPage)
	if err != nil {
		log.Printf("handler: Ошибка в GetRecentPhotosFromDB: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Ошибка при получении последних фото")
		return
	}

	respondWithJSON(w, http.StatusOK, photos)
}

// GetPhotoDetailsFromDB обрабатывает запрос для получения деталей фото по нашему внутреннему ID
// GET /photos/{id}
func (h *PhotoHandler) GetPhotoDetailsFromDB(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		respondWithError(w, http.StatusBadRequest, "ID фото не указан")
		return
	}

	photoID, err := uuid.Parse(idStr)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Некорректный формат ID фото")
		return
	}

	log.Printf("handler: Запрос на получение деталей фото по ID: %s", photoID)
	photo, err := h.photoUseCase.GetPhotoDetailsFromDB(r.Context(), photoID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "Фотография не найдена")
			return
		}
		log.Printf("handler: Ошибка в GetPhotoDetailsFromDB: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Ошибка при получении деталей фото")
		return
	}

	respondWithJSON(w, http.StatusOK, photo)
}
