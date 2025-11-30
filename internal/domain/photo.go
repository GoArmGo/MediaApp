package domain

import (
	"time"

	"github.com/google/uuid"
)

// Photo представляет модель фотографии в системе,
// соответствует таблице photos в бд
type Photo struct {
	ID             uuid.UUID `json:"id"`
	UnsplashID     string    `json:"unsplash_id"`
	UserID         uuid.UUID `json:"user_id"`
	S3URL          string    `json:"s3_url"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	AuthorName     string    `json:"author_name"`
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	LikesCount     int       `json:"likes_count"`
	OriginalURL    string    `json:"original_url"`
	UploadedAt     time.Time `json:"uploaded_at"`
	ViewsCount     int64     `json:"views_count"`
	DownloadsCount int64     `json:"downloads_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Tags           []Tag     `json:"tags,omitempty"`
}

func (Photo) TableName() string {
	return "photos"
}

// Tag представляет модель тега,
// соответствует таблице tags в бд
type Tag struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (Tag) TableName() string {
	return "tags"
}

// PhotoTag представляет связующую модель для отношения Many-to-Many между Photo и Tag,
// соответствует таблице photo_tags в бд
type PhotoTag struct {
	PhotoID uuid.UUID `json:"photo_id"`
	TagID   uuid.UUID `json:"tag_id"`
}

func (PhotoTag) TableName() string {
	return "photo_tags"
}
