CREATE TABLE IF NOT EXISTS room_participants (
    room_id INT NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id INT NOT NULL,

    role TEXT NOT NULL DEFAULT 'member' CHECK (
        role IN ('leader', 'member')
    ),

    joined_at TIMESTAMP DEFAULT NOW(),

    PRIMARY KEY (room_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_room_participants_user_id
    ON room_participants(user_id);
