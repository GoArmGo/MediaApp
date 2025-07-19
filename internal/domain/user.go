// internal/domain/user.go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// User представляет модель пользователя в системе,
// соответствует таблице users в бд
type User struct {
	ID           uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Username     string    `json:"username" gorm:"unique"`
	Email        string    `json:"email" gorm:"unique"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}
