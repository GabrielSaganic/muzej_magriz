-- db/init.sql

DROP TABLE IF EXISTS linkovi;
DROP TABLE IF EXISTS slike;

CREATE TABLE slike (
    id SERIAL PRIMARY KEY,
    gallery_code TEXT DEFAULT 'main',
    image_url TEXT NOT NULL,
    order_index INT NOT NULL,
    desc_hr TEXT, desc_en TEXT, desc_it TEXT, desc_de TEXT,
    long_desc_hr TEXT, long_desc_en TEXT, long_desc_it TEXT, long_desc_de TEXT
);

CREATE TABLE linkovi (
    id SERIAL PRIMARY KEY,
    source_slika_id INT REFERENCES slike(id) ON DELETE CASCADE,
    target_gallery_code TEXT NOT NULL,
    name_hr TEXT, name_en TEXT, name_it TEXT, name_de TEXT
);

CREATE TABLE videa (
    id SERIAL PRIMARY KEY,
    gallery_code TEXT DEFAULT 'main',
    video_url TEXT NOT NULL,
    thumbnail_url TEXT,
    order_index INT NOT NULL,
    desc_hr TEXT, desc_en TEXT, desc_it TEXT, desc_de TEXT
);
