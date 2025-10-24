package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/core/ports"
	"github.com/GoArmGo/MediaApp/internal/handler"
	"github.com/GoArmGo/MediaApp/internal/usecase"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// runServer запускает HTTP сервер и логику публикации сообщений
func runServer(
	ctx context.Context,
	cfg *config.Config,
	photoUseCase usecase.PhotoUseCase,
	photoSearchPublisher ports.PhotoSearchPublisher,
	uploadLimiter chan struct{},
) error {
	photoHandler := handler.NewPhotoHandler(photoUseCase, photoSearchPublisher, uploadLimiter)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(cfg.RequestTimeout))

	r.Get("/photos/{unsplashID}", photoHandler.GetOrCreatePhotoByUnsplashID)
	r.Get("/photos/search", photoHandler.SearchAndSavePhotos)
	r.Get("/photos/recent", photoHandler.GetRecentPhotosFromDB)
	r.Get("/photos/{id}", photoHandler.GetPhotoDetailsFromDB)

	serverAddr := fmt.Sprintf(":%s", cfg.ServerPort)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: r,
	}

	go func() {
		log.Printf("Сервер запущен на %s", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка при запуске сервера: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("Получен сигнал завершения. Завершаем работу сервера...")

	ctxServer, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctxServer); err != nil {
		//log.Fatalf("Ошибка при завершении работы сервера: %v", err)
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	log.Println("Сервер успешно завершил работу.")
	return nil
}
