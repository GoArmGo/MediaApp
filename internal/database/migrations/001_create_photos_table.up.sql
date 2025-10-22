CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    unsplash_id VARCHAR(50) UNIQUE NOT NULL,      
    user_id UUID NOT NULL,                         
    s3_url TEXT NOT NULL,                          
    title TEXT,                                   
    description TEXT,                             
    author_name VARCHAR(255) NOT NULL,            
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    likes_count INTEGER DEFAULT 0,                
    original_url TEXT NOT NULL,                   
    uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL, 
    views_count BIGINT DEFAULT 0,
    downloads_count BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,  
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,  
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_photos_user_id ON photos (user_id);
CREATE INDEX IF NOT EXISTS idx_photos_unsplash_id ON photos (unsplash_id);  
CREATE INDEX IF NOT EXISTS idx_photos_uploaded_at ON photos (uploaded_at DESC);

CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) UNIQUE NOT NULL
);

-- связующая таблица photo_tags
CREATE TABLE IF NOT EXISTS photo_tags (
    photo_id UUID NOT NULL,
    tag_id UUID NOT NULL,
    PRIMARY KEY (photo_id, tag_id),
    FOREIGN KEY (photo_id) REFERENCES photos(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_tags_name ON tags (name);
