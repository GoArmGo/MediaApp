// internal/domain/user.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// User представляет модель пользователя в системе,
// соответствует таблице users в бд
type User struct {
	ID           uuid.UUID `json:"id" db:"id" gorm:"type:uuid;primaryKey"`
	Username     string    `json:"username" db:"username" gorm:"unique"`
	Email        string    `json:"email" db:"email" gorm:"unique"`
	PasswordHash string    `json:"-" db:"password_hash"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

func (User) TableName() string {
	return "users"
}
