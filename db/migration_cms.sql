-- Migration for existing databases: adds CMS tables and long_desc to videa
-- Run this if you already have an existing database and don't want to recreate it.

ALTER TABLE videa ADD COLUMN IF NOT EXISTS long_desc_hr TEXT;
ALTER TABLE videa ADD COLUMN IF NOT EXISTS long_desc_en TEXT;
ALTER TABLE videa ADD COLUMN IF NOT EXISTS long_desc_it TEXT;
ALTER TABLE videa ADD COLUMN IF NOT EXISTS long_desc_de TEXT;

CREATE TABLE IF NOT EXISTS cms_users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cms_sessions (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES cms_users(id) ON DELETE CASCADE,
    token TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
