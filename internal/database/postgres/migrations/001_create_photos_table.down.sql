-- 001_create_photos_table.down.sql

-- Удаляем связующую таблицу 'photo_tags'
DROP TABLE IF EXISTS photo_tags;

-- Удаляем таблицу 'tags'
DROP TABLE IF EXISTS tags;

-- Удаляем таблицу 'photos'
DROP TABLE IF EXISTS photos;

-- Удаляем таблицу 'users'
DROP TABLE IF EXISTS users;