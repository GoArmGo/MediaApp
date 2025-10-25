package logger

import (
	"log/slog"
	"os"
	"time"
)

// Config описывает параметры логгера (можно расширить позже)
type SlogConfig struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "json" или "text"
}

// New создаёт и настраивает slog.Logger
func NewSlog(cfg SlogConfig) *slog.Logger {
	var lvl slog.Level

	switch cfg.Level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	var handler slog.Handler

	// Выбираем формат вывода
	if cfg.Format == "text" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: lvl,
			// Добавляем timestamp в человекочитаемом виде
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					a.Value = slog.StringValue(time.Now().Format(time.RFC3339))
				}
				return a
			},
		})
	}

	return slog.New(handler)
}
