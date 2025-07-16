-- 001_create_photos_table.up.sql

-- Создаем таблицу 'users'
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Создаем таблицу 'photos' со всеми необходимыми полями
CREATE TABLE IF NOT EXISTS photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unsplash_id VARCHAR(50) UNIQUE NOT NULL,      -- Теперь UNIQUE NOT NULL, так как это ключ из Unsplash
    user_id UUID NOT NULL,                        -- ID пользователя, который сохранил фото (из вашей БД)
    s3_url TEXT NOT NULL,                         -- Полный URL изображения в AWS S3
    title TEXT,                                   -- Название или краткое описание
    description TEXT,                             -- Более подробное описание
    author_name VARCHAR(255) NOT NULL,            -- Имя автора фото с Unsplash
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    likes_count INTEGER DEFAULT 0,                -- Количество лайков на Unsplash
    original_url TEXT NOT NULL,                   -- Оригинальный URL Unsplash фото
    uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, -- Дата и время сохранения в вашей БД (изначально)
    views_count BIGINT DEFAULT 0,
    downloads_count BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, -- <--- ДОБАВЛЕНО
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP, -- <--- ДОБАВЛЕНО
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Создаем индексы для ускорения запросов
CREATE INDEX IF NOT EXISTS idx_photos_user_id ON photos (user_id);
CREATE INDEX IF NOT EXISTS idx_photos_unsplash_id ON photos (unsplash_id); -- Добавляем индекс для unsplash_id
CREATE INDEX IF NOT EXISTS idx_photos_uploaded_at ON photos (uploaded_at DESC);

-- Создаем таблицу 'tags'
CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL
);

-- Создаем связующую таблицу 'photo_tags'
CREATE TABLE IF NOT EXISTS photo_tags (
    photo_id UUID NOT NULL,
    tag_id UUID NOT NULL,
    PRIMARY KEY (photo_id, tag_id),
    FOREIGN KEY (photo_id) REFERENCES photos(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

-- Создаем индексы для ускорения запросов для tags
CREATE INDEX IF NOT EXISTS idx_tags_name ON tags (name);
