package unsplash

import "time"

// Отдельная структура для URL-ов
type UnsplashPhotoURLs struct {
	Raw     string `json:"raw"`
	Full    string `json:"full"`
	Regular string `json:"regular"`
	Small   string `json:"small"`
	Thumb   string `json:"thumb"`
}

// Отдельная структура для пользователя
type UnsplashUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

// Теперь UnsplashPhotoResponse использует эти именованные структуры
type UnsplashPhotoResponse struct {
	ID             string `json:"id"`
	Description    string `json:"description"`
	AltDescription string `json:"alt_description"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Likes          int    `json:"likes"`

	URLs UnsplashPhotoURLs `json:"urls"`
	User UnsplashUser      `json:"user"`

	Views     int64     `json:"views,omitempty"`
	Downloads int64     `json:"downloads,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// UnsplashSearchResponse для ответа
type UnsplashSearchResponse struct {
	Total      int                     `json:"total"`
	TotalPages int                     `json:"total_pages"`
	Results    []UnsplashPhotoResponse `json:"results"`
}
