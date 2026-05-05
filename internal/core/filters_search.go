package core

type Filters struct {
	City     *string `json:"city"`
	RoomType *string `json:"roomType"`
	Limit    int     `json:"limit"`
	LastID   int     `json:"last_id"`
}
