package postgres

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GormUserStorage реализует интерфейс ports.UserStorage с использованием GORM
type GormUserStorage struct {
	db *gorm.DB
}

// NewGormUserStorage создает новый экземпляр GormUserStorage
func NewGormUserStorage(db *gorm.DB) *GormUserStorage {
	return &GormUserStorage{db: db}
}

// GetOrCreateSystemUser получает или создает системного пользователя в бд
func (s *GormUserStorage) GetOrCreateSystemUser(ctx context.Context) (uuid.UUID, error) {
	var user domain.User
	result := s.db.WithContext(ctx).Where("username = ?", "system_user").First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		log.Println("Системный пользователь не найден, создаем нового...")
		newUser := domain.User{
			ID:           uuid.New(),
			Username:     "system_user",
			Email:        "system@example.com",
			PasswordHash: "dummy_hash",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		createResult := s.db.WithContext(ctx).Create(&newUser)
		if createResult.Error != nil {
			return uuid.Nil, fmt.Errorf("ошибка при создании системного пользователя с GORM: %w", createResult.Error)
		}
		log.Printf("Системный пользователь %s успешно создан.", newUser.ID)
		return newUser.ID, nil
	} else if result.Error != nil {
		return uuid.Nil, fmt.Errorf("ошибка при поиске системного пользователя с GORM: %w", result.Error)
	}

	log.Printf("Системный пользователь %s найден.", user.ID)
	return user.ID, nil
}
