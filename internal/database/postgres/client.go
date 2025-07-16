package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/GoArmGo/MediaApp/internal/config"
	"github.com/GoArmGo/MediaApp/internal/domain"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Client представляет клиент для взаимодействия с PostgreSQL.
type Client struct {
	DB *sqlx.DB
}

// NewClient инициализирует новое подключение к PostgreSQL и применяет миграции.
func NewClient(cfg *config.Config) (*Client, error) {
	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия соединения с БД: %w", err)
	}

	// Настройка пула соединений (опционально, но рекомендуется)
	db.SetMaxOpenConns(25)                 // Максимальное количество открытых соединений
	db.SetMaxIdleConns(10)                 // Максимальное количество неактивных соединений в пуле
	db.SetConnMaxLifetime(5 * time.Minute) // Максимальное время жизни соединения

	// Проверка соединения с базой данных
	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к базе данных: %w", err)
	}

	log.Println("Успешное подключение к базе данных PostgreSQL.")

	// Применение миграций
	if err := applyMigrations(cfg.DatabaseURL); err != nil {
		return nil, fmt.Errorf("ошибка при применении миграций: %w", err)
	}

	return &Client{DB: db}, nil
}

// applyMigrations применяет все доступные миграции к базе данных.
func applyMigrations(databaseURL string) error {
	m, err := migrate.New(
		"file://internal/database/postgres/migrations", // Путь к вашим SQL-файлам миграций
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("не удалось создать экземпляр мигратора: %w", err)
	}

	// Выполнить миграции вверх (Up)
	if err = m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("ошибка выполнения миграций: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("Миграции не требуются, база данных актуальна.")
	} else {
		log.Println("Миграции успешно применены.")
	}
	return nil
}

// Close закрывает соединение с базой данных.
func (c *Client) Close() error {
	return c.DB.Close()
}

// -----
// PostgresStorage реализует интерфейс PhotoStorage для PostgreSQL.
type PostgresStorage struct {
	db *sqlx.DB
}

// NewPostgresStorage создает новый экземпляр PostgresStorage.
func NewPostgresStorage(db *sqlx.DB) *PostgresStorage {
	return &PostgresStorage{db: db}
}

// SavePhoto сохраняет метаданные фотографии в базе данных.
func (s *PostgresStorage) SavePhoto(ctx context.Context, photo *domain.Photo) error {
	systemUserID, err := s.getOrCreateSystemUser()
	if err != nil {
		return fmt.Errorf("не удалось получить или создать системного пользователя: %w", err)
	}
	photo.UserID = systemUserID

	// Если photo.ID еще не установлен, генерируем его.
	if photo.ID == uuid.Nil {
		photo.ID = uuid.New()
	}
	// Если photo.UploadedAt еще не установлено, устанавливаем текущее время.
	if photo.UploadedAt.IsZero() {
		photo.UploadedAt = time.Now()
	}

	// SQLX здесь не сильно упрощает INSERT, но делает его более безопасным за счет NamedExec
	// Мы все равно перечисляем колонки и используем именованные параметры.
	// sqlx.NamedExec автоматически подставит значения из структуры по тегам `db`.
	query := `
		INSERT INTO photos (
			id, unsplash_id, user_id, s3_url, title, description,
			author_name, width, height, likes_count, original_url,
			uploaded_at, views_count, downloads_count
		) VALUES (
			:id, :unsplash_id, :user_id, :s3_url, :title, :description,
			:author_name, :width, :height, :likes_count, :original_url,
			:uploaded_at, :views_count, :downloads_count
		)
	`
	// Обратите внимание: RETURNING id здесь не нужен, так как photo.ID уже сгенерирован.
	// Если бы ID генерировался БД (например, SERIAL), мы бы использовали NamedQuery().StructScan()

	_, err = s.db.NamedExec(query, photo) // <-- Использование NamedExec с нашей структурой Photo
	if err != nil {
		return fmt.Errorf("ошибка при сохранении фото в БД с помощью sqlx: %w", err)
	}

	log.Printf("Фото %s (Unsplash ID: %s) успешно сохранено в БД.", photo.ID, photo.UnsplashID)
	return nil
}

// getOrCreateSystemUser() также можно оптимизировать с sqlx.Get() для удобства.
func (s *PostgresStorage) getOrCreateSystemUser() (uuid.UUID, error) {
	var userID uuid.UUID
	query := `SELECT id FROM users WHERE username = 'system_user'`

	// Используем QueryRowx (из sqlx) для выборки одной строки и Scan
	err := s.db.QueryRowx(query).Scan(&userID) // QueryRowx возвращает *Row, у которого есть Scan

	if err == sql.ErrNoRows {
		log.Println("Системный пользователь не найден, создаем нового...")
		newUser := domain.User{
			ID:           uuid.New(),
			Username:     "system_user",
			Email:        "system@example.com",
			PasswordHash: "dummy_hash",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		insertQuery := `
			INSERT INTO users (id, username, email, password_hash, created_at, updated_at)
			VALUES (:id, :username, :email, :password_hash, :created_at, :updated_at)
		`
		// NamedExec для INSERT запроса, используем структуру newUser
		_, err = s.db.NamedExec(insertQuery, newUser) // <-- Используем NamedExec
		if err != nil {
			return uuid.Nil, fmt.Errorf("ошибка при создании системного пользователя с sqlx: %w", err)
		}
		// После вставки, нам нужно получить ID. Если ID генерируется БД,
		// то нужно делать отдельный SELECT или использовать RETURNING id и Scan.
		// Поскольку у нас UUID генерируется в Go, ID уже в newUser.ID
		log.Printf("Системный пользователь %s успешно создан.", newUser.ID)
		return newUser.ID, nil
	} else if err != nil {
		return uuid.Nil, fmt.Errorf("ошибка при поиске системного пользователя: %w", err)
	}

	log.Printf("Системный пользователь %s найден.", userID)
	return userID, nil
}

// GetPhotoByIDFromDB теперь будет использовать sqlx.Get
func (s *PostgresStorage) GetPhotoByIDFromDB(ctx context.Context, id uuid.UUID) (*domain.Photo, error) {
	var photo domain.Photo
	query := `SELECT * FROM photos WHERE id = $1`
	// sqlx.Get умеет сканировать одну строку в структуру по db тегам
	err := s.db.Get(&photo, query, id) // <-- Использование sqlx.Get
	if err == sql.ErrNoRows {
		return nil, nil // Фото не найдено
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении фото по ID из БД: %w", err)
	}
	return &photo, nil
}

// GetPhotosByUnsplashIDFromDB также будет использовать sqlx.Get
func (s *PostgresStorage) GetPhotosByUnsplashIDFromDB(ctx context.Context, unsplashID string) (*domain.Photo, error) {
	var photo domain.Photo
	query := `SELECT * FROM photos WHERE unsplash_id = $1`
	err := s.db.Get(&photo, query, unsplashID)
	if err == sql.ErrNoRows {
		return nil, nil // Фото не найдено
	}
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении фото по Unsplash ID из БД: %w", err)
	}
	return &photo, nil
}

// SearchPhotosInDB будет использовать sqlx.Select
func (s *PostgresStorage) SearchPhotosInDB(ctx context.Context, query string, page, perPage int) ([]domain.Photo, error) {
	var photos []domain.Photo
	// SQL LIKE для поиска подстроки, LOWER для регистронезависимого поиска
	// Добавляем OFFSET и LIMIT для пагинации
	sqlQuery := `
		SELECT * FROM photos
		WHERE LOWER(title) LIKE LOWER($1) OR LOWER(description) LIKE LOWER($1) OR LOWER(author_name) LIKE LOWER($1)
		ORDER BY uploaded_at DESC
		LIMIT $2 OFFSET $3
	`
	// %s% в SQL для LIKE, здесь мы передаем query как %query%
	searchParam := fmt.Sprintf("%%%s%%", query)
	offset := (page - 1) * perPage

	// sqlx.Select умеет сканировать несколько строк в срез структур
	err := s.db.Select(&photos, sqlQuery, searchParam, perPage, offset)
	if err != nil {
		return nil, fmt.Errorf("ошибка при поиске фото в БД: %w", err)
	}
	return photos, nil
}

// ListAllPhotosInDB также будет использовать sqlx.Select
func (s *PostgresStorage) ListAllPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {
	var photos []domain.Photo
	sqlQuery := `
		SELECT * FROM photos
		ORDER BY uploaded_at DESC
		LIMIT $1 OFFSET $2
	`
	offset := (page - 1) * perPage

	err := s.db.Select(&photos, sqlQuery, perPage, offset)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении всех фото из БД: %w", err)
	}
	return photos, nil
}

// ListPhotosInDB получает список фотографий из БД с пагинацией.
func (s *PostgresStorage) ListPhotosInDB(ctx context.Context, page, perPage int) ([]domain.Photo, error) {
	var photos []domain.Photo
	offset := (page - 1) * perPage // Вычисляем смещение

	// SQL-запрос для получения списка фото с лимитом и смещением
	query := `
        SELECT id, unsplash_id, s3_url, title, description, author_name,
               width, height, likes_count, original_url, uploaded_at,
               views_count, downloads_count, created_at, updated_at
        FROM photos
        ORDER BY created_at DESC -- Например, по дате создания
        LIMIT $1 OFFSET $2
    `
	err := s.db.SelectContext(ctx, &photos, query, perPage, offset)
	if err != nil {
		return nil, fmt.Errorf("ошибка при получении списка фото из БД: %w", err)
	}
	return photos, nil
}
