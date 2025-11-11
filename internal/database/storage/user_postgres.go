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

const systemUsername = "system_user"

// GormUserStorage реализует интерфейс ports.UserStorage с использованием GORM
type UserStorage struct {
	db     *sqlx.DB
	logger *slog.Logger
}

// NewGormUserStorage создает новый экземпляр GormUserStorage
func NewUserStorage(db *sqlx.DB, logger *slog.Logger) *UserStorage {
	return &UserStorage{db: db, logger: logger}
}

// GetOrCreateSystemUser получает или создает системного пользователя в БД.
func (s *UserStorage) GetOrCreateSystemUser(ctx context.Context) (uuid.UUID, error) {
	start := time.Now()

	var user domain.User
	err := s.db.GetContext(ctx, &user, `SELECT * FROM users WHERE username = $1`, systemUsername)

	if errors.Is(err, sql.ErrNoRows) {
		s.logger.Warn("system user not found, creating new one", "username", systemUsername)

		newUser := domain.User{
			ID:           uuid.New(),
			Username:     systemUsername,
			Email:        "system@example.com",
			PasswordHash: "dummy_hash",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		_, err = s.db.NamedExecContext(ctx, `
            INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
            VALUES (:id, :username, :email, :password_hash, :created_at, :updated_at)
        `, &newUser)
		if err != nil {
			s.logger.Error("failed to insert system user", "error", err)
			return uuid.Nil, fmt.Errorf("insert system user: %w", err)
		}

		s.logger.Info("system user created successfully",
			"user_id", newUser.ID,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return newUser.ID, nil
	}

	if err != nil {
		s.logger.Error("failed to select system user", "error", err)
		return uuid.Nil, fmt.Errorf("select system user: %w", err)
	}

	s.logger.Info("system user found",
		"user_id", user.ID,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return user.ID, nil
}
