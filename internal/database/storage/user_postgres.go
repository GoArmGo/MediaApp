package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const systemUsername = "system_user"

// GormUserStorage реализует интерфейс ports.UserStorage с использованием GORM
type UserStorage struct {
	db *sqlx.DB
}

// NewGormUserStorage создает новый экземпляр GormUserStorage
func NewUserStorage(db *sqlx.DB) *UserStorage {
	return &UserStorage{db: db}
}

// GetOrCreateSystemUser получает или создает системного пользователя в БД.
func (s *UserStorage) GetOrCreateSystemUser(ctx context.Context) (uuid.UUID, error) {

	var user domain.User
	err := s.db.GetContext(ctx, &user, `SELECT * FROM users WHERE username = $1`, systemUsername)

	if errors.Is(err, sql.ErrNoRows) {
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
			return uuid.Nil, fmt.Errorf("insert system user: %w", err)
		}

		log.Println("[bootstrap] System user created:", newUser.ID)
		return newUser.ID, nil
	}

	if err != nil {
		return uuid.Nil, fmt.Errorf("select system user: %w", err)
	}

	return user.ID, nil
}
