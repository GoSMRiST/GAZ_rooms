package core

import "time"

type Room struct {
	RoomID            int       `json:"room_id"`
	UserID            int       `json:"user_id"`
	RoomName          string    `json:"room_name"`
	RoomType          string    `json:"room_type"`    // название типа, напр. "bars"
	RoomTypeID        int       `json:"room_type_id"` // id из таблицы room_types
	RoomDesc          string    `json:"room_desc"`
	PeopleLimit       int       `json:"people_limit"`
	City              string    `json:"city"`
	ParticipantsCount int       `json:"participants_count"`
	CreatedAt         time.Time `json:"created_at"`
}

type RoomType struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}
