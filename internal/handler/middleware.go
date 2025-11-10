package handler

import (
	"log/slog"
	"net/http"
	"time"
)

// RequestLogger — middleware для логирования HTTP-запросов.
func RequestLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Оборачиваем ResponseWriter, чтобы знать статус
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.statusCode,
				"duration_ms", duration.Milliseconds(),
			)
		})
	}
}

// responseWriter нужен, чтобы перехватывать код ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
