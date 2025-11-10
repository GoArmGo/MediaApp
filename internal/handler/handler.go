package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/GoArmGo/MediaApp/internal/core/ports"
	"github.com/GoArmGo/MediaApp/internal/usecase"
	"github.com/google/uuid"
)

// PhotoHandler — обработчик HTTP-запросов для работы с фотографиями.
type PhotoHandler struct {
	photoUseCase         usecase.PhotoUseCase
	photoSearchPublisher ports.PhotoSearchPublisher
	uploadLimiter        chan struct{}
	logger               *slog.Logger
}

// NewPhotoHandler создаёт новый экземпляр PhotoHandler.
func NewPhotoHandler(
	uc usecase.PhotoUseCase,
	publisher ports.PhotoSearchPublisher,
	limiter chan struct{},
	logger *slog.Logger,
) *PhotoHandler {
	return &PhotoHandler{
		photoUseCase:         uc,
		photoSearchPublisher: publisher,
		uploadLimiter:        limiter,
		logger:               logger,
	}
}

// respondWithJSON — отправляет JSON-ответ клиенту.
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}, logger *slog.Logger) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error("failed to marshal JSON response", "error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if _, err = w.Write(response); err != nil {
		logger.Error("failed to write HTTP response", "error", err)
	}
}

// respondWithError — отправляет JSON-ответ с ошибкой.
func respondWithError(w http.ResponseWriter, code int, message string, logger *slog.Logger) {
	respondWithJSON(w, code, map[string]string{"error": message}, logger)
}

// GetOrCreatePhotoByUnsplashID — получает фото по unsplash_id или создаёт новое.
func (h *PhotoHandler) GetOrCreatePhotoByUnsplashID(w http.ResponseWriter, r *http.Request) {
	unsplashID := r.URL.Query().Get("unsplash_id")
	if unsplashID == "" {
		h.logger.Warn("missing required parameter", "param", "unsplash_id")
		respondWithError(w, http.StatusBadRequest, "Не указан unsplash_id", h.logger)
		return
	}

	h.logger.Info("processing request", "endpoint", "GetOrCreatePhotoByUnsplashID", "unsplash_id", unsplashID)

	photo, err := h.photoUseCase.GetOrCreatePhotoByUnsplashID(r.Context(), unsplashID)
	if err != nil {
		h.logger.Error("failed to get or create photo", "unsplash_id", unsplashID, "error", err)
		respondWithError(w, http.StatusInternalServerError, "Ошибка при получении или создании фото", h.logger)
		return
	}

	h.logger.Info("photo processed successfully", "unsplash_id", unsplashID)
	respondWithJSON(w, http.StatusOK, photo, h.logger)
}

// SearchAndSavePhotos — выполняет поиск фото и сохраняет их.
func (h *PhotoHandler) SearchAndSavePhotos(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		h.logger.Warn("missing required parameter", "param", "query")
		respondWithError(w, http.StatusBadRequest, "Не указан параметр запроса", h.logger)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage <= 0 {
		perPage = 10
	}

	h.logger.Info("searching and saving photos",
		"endpoint", "SearchAndSavePhotos",
		"query", query,
		"page", page,
		"per_page", perPage,
	)

	_, err := h.photoUseCase.SearchAndSavePhotos(r.Context(), query, page, perPage)
	if err != nil {
		h.logger.Error("failed to search and save photos", "query", query, "error", err)
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Ошибка поиска фото: %v", err), h.logger)
		return
	}

	h.logger.Info("photos search and save completed", "query", query, "page", page)
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Фотографии успешно сохранены"}, h.logger)
}

// GetRecentPhotosFromDB — получает последние фото из БД.
func (h *PhotoHandler) GetRecentPhotosFromDB(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if perPage <= 0 {
		perPage = 10
	}

	h.logger.Info("fetching recent photos",
		"endpoint", "GetRecentPhotosFromDB",
		"page", page,
		"per_page", perPage,
	)

	photos, err := h.photoUseCase.GetRecentPhotosFromDB(r.Context(), page, perPage)
	if err != nil {
		h.logger.Error("failed to fetch recent photos", "error", err)
		respondWithError(w, http.StatusInternalServerError, "Ошибка получения последних фото", h.logger)
		return
	}

	h.logger.Info("recent photos fetched successfully", "count", len(photos))
	respondWithJSON(w, http.StatusOK, photos, h.logger)
}

// GetPhotoDetailsFromDB — получает детальную информацию о фото.
func (h *PhotoHandler) GetPhotoDetailsFromDB(w http.ResponseWriter, r *http.Request) {
	photoIDStr := r.URL.Query().Get("photo_id")
	if photoIDStr == "" {
		h.logger.Warn("missing required parameter", "param", "photo_id")
		respondWithError(w, http.StatusBadRequest, "Не указан photo_id", h.logger)
		return
	}

	photoUUID, err := uuid.Parse(photoIDStr)
	if err != nil {
		h.logger.Error("invalid photo_id parameter", "photo_id", photoIDStr, "error", err)
		respondWithError(w, http.StatusBadRequest, "Некорректный photo_id", h.logger)
		return
	}

	h.logger.Info("fetching photo details",
		"endpoint", "GetPhotoDetailsFromDB",
		"photo_id", photoUUID,
	)

	photo, err := h.photoUseCase.GetPhotoDetailsFromDB(r.Context(), photoUUID)
	if err != nil {
		h.logger.Error("failed to fetch photo details", "photo_id", photoUUID, "error", err)
		respondWithError(w, http.StatusInternalServerError, "Ошибка получения информации о фото", h.logger)
		return
	}

	h.logger.Info("photo details fetched successfully", "photo_id", photoUUID)
	respondWithJSON(w, http.StatusOK, photo, h.logger)
}
