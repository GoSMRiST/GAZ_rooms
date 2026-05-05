CREATE TABLE IF NOT EXISTS rooms (
    id SERIAL PRIMARY KEY,

    name TEXT NOT NULL,

    description TEXT ,

    max_people INT NOT NULL CHECK (max_people > 0),

    leader_id INT NOT NULL,

    city TEXT NOT NULL,

    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rooms_city ON rooms(city);
CREATE INDEX IF NOT EXISTS idx_rooms_leader_id ON rooms(leader_id);
