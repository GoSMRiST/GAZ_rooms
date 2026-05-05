CREATE TABLE IF NOT EXISTS room_types (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO room_types (name) VALUES
    ('бары'),
    ('клубы'),
    ('настолки'),
    ('прогулка'),
    ('посидеть'),
    ('спорт'),
    ('работа'),
    ('квесты');

-- Добавляем внешний ключ в rooms
ALTER TABLE rooms
    ADD COLUMN type_id INT NOT NULL DEFAULT 1 REFERENCES room_types(id);

CREATE INDEX IF NOT EXISTS idx_rooms_type_id ON rooms(type_id);
