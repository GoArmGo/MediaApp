// internal/domain/user.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// User представляет модель пользователя в системе.
// Соответствует таблице 'users' в базе данных.
// (Если у вас уже есть этот файл, добавьте теги туда)
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
