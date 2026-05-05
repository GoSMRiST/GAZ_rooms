package rest

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"rooms/internal/core"
	"rooms/internal/middleware"
)

type RoomService interface {
	GetRoomTypes(ctx context.Context) ([]core.RoomType, error)
	GetCities(ctx context.Context) ([]string, error)
	CreateRoom(ctx context.Context, room *core.Room) (*core.Room, error)
	GetRoom(ctx context.Context, roomID int) (*core.Room, error)
	ListRooms(ctx context.Context, filters *core.Filters) ([]core.Room, error)
	GetUserRooms(ctx context.Context, userID int) ([]core.Room, error)
	GetRoomParticipants(ctx context.Context, roomID int) ([]core.Participant, error)
	JoinRoom(ctx context.Context, roomID int, userID int) error
	LeaveRoom(ctx context.Context, roomID int, userID int) error
	DeleteRoom(ctx context.Context, roomID int, userID int) error
	UpdateRoom(ctx context.Context, room *core.Room) (*core.Room, error)
}

type RoomHandler struct {
	log     *slog.Logger
	service RoomService
}

func NewRoomHandler(log *slog.Logger, service RoomService) *RoomHandler {
	return &RoomHandler{log: log, service: service}
}

func (h *RoomHandler) RegisterRoutes(engine *gin.Engine, auth gin.HandlerFunc) {
	engine.GET("/room-types", h.GetRoomTypes)
	engine.GET("/cities", h.GetCities)

	r := engine.Group("/rooms")

	r.GET("", h.ListRooms)
	r.GET("/:id", h.GetRoom)
	r.GET("/:id/participants", h.GetRoomParticipants)

	protected := r.Group("", auth)
	{
		protected.POST("", h.CreateRoom)
		protected.PUT("/:id", h.UpdateRoom)
		protected.DELETE("/:id", h.DeleteRoom)
		protected.POST("/:id/join", h.JoinRoom)
		protected.POST("/:id/leave", h.LeaveRoom)
	}

	users := engine.Group("/users", auth)
	{
		users.GET("/me/rooms", h.GetUserRooms)
	}
}

type createRoomRequest struct {
	RoomName    string `json:"room_name"    binding:"required"`
	RoomType    string `json:"room_type"    binding:"required"` // название: "bars", "sport" и т.д.
	RoomDesc    string `json:"room_desc"`
	PeopleLimit int    `json:"people_limit" binding:"required,min=2"`
	City        string `json:"city"         binding:"required"`
}

type updateRoomRequest struct {
	RoomName    string `json:"room_name"    binding:"required"`
	RoomType    string `json:"room_type"    binding:"required"`
	RoomDesc    string `json:"room_desc"`
	PeopleLimit int    `json:"people_limit" binding:"required,min=2"`
	City        string `json:"city"         binding:"required"`
}

// GET /room-types
func (h *RoomHandler) GetRoomTypes(c *gin.Context) {
	types, err := h.service.GetRoomTypes(c.Request.Context())
	if err != nil {
		h.handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": types})
}

// POST /rooms
func (h *RoomHandler) CreateRoom(c *gin.Context) {
	userID, ok := middleware.GetUserID(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	room, err := h.service.CreateRoom(c.Request.Context(), &core.Room{
		UserID:      userID,
		RoomName:    req.RoomName,
		RoomType:    req.RoomType,
		RoomDesc:    req.RoomDesc,
		PeopleLimit: req.PeopleLimit,
		City:        req.City,
	})
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "success", "data": room})
}

// GET /rooms
func (h *RoomHandler) ListRooms(c *gin.Context) {
	filters := &core.Filters{}

	if city := c.Query("city"); city != "" {
		filters.City = &city
	}
	if roomType := c.Query("type"); roomType != "" {
		filters.RoomType = &roomType
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filters.Limit = limit
		}
	}
	// Cursor-based пагинация: ?last_id=<id последней комнаты с предыдущей страницы>
	if lastIDStr := c.Query("last_id"); lastIDStr != "" {
		if lastID, err := strconv.Atoi(lastIDStr); err == nil && lastID > 0 {
			filters.LastID = lastID
		}
	}

	rooms, err := h.service.ListRooms(c.Request.Context(), filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	// Возвращаем next_cursor — id последней комнаты в наборе.
	// Клиент передаёт его как ?last_id= для следующей страницы.
	var nextCursor *int
	if len(rooms) > 0 {
		last := rooms[len(rooms)-1].RoomID
		nextCursor = &last
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"data":        rooms,
		"next_cursor": nextCursor,
	})
}

// GET /rooms/:id
func (h *RoomHandler) GetRoom(c *gin.Context) {
	roomID, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	room, err := h.service.GetRoom(c.Request.Context(), roomID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": room})
}

// GET /rooms/:id/participants
func (h *RoomHandler) GetRoomParticipants(c *gin.Context) {
	roomID, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	participants, err := h.service.GetRoomParticipants(c.Request.Context(), roomID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": participants})
}

// GET /rooms/me
func (h *RoomHandler) GetUserRooms(c *gin.Context) {
	userID, ok := middleware.GetUserID(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	rooms, err := h.service.GetUserRooms(c.Request.Context(), userID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": rooms})
}

// POST /rooms/:id/join
func (h *RoomHandler) JoinRoom(c *gin.Context) {
	userID, ok := middleware.GetUserID(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	roomID, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	if err := h.service.JoinRoom(c.Request.Context(), roomID, userID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "result": "joined room"})
}

// POST /rooms/:id/leave
func (h *RoomHandler) LeaveRoom(c *gin.Context) {
	userID, ok := middleware.GetUserID(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	roomID, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	if err := h.service.LeaveRoom(c.Request.Context(), roomID, userID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "result": "left room"})
}

// PUT /rooms/:id
func (h *RoomHandler) UpdateRoom(c *gin.Context) {
	userID, ok := middleware.GetUserID(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	roomID, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	var req updateRoomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	updated, err := h.service.UpdateRoom(c.Request.Context(), &core.Room{
		RoomID:      roomID,
		UserID:      userID,
		RoomName:    req.RoomName,
		RoomType:    req.RoomType,
		RoomDesc:    req.RoomDesc,
		PeopleLimit: req.PeopleLimit,
		City:        req.City,
	})
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": updated})
}

// DELETE /rooms/:id
func (h *RoomHandler) DeleteRoom(c *gin.Context) {
	userID, ok := middleware.GetUserID(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	roomID, err := parseIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room id"})
		return
	}

	if err := h.service.DeleteRoom(c.Request.Context(), roomID, userID); err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "result": "room deleted"})
}

func parseIDParam(c *gin.Context, param string) (int, error) {
	return strconv.Atoi(c.Param(param))
}

func (h *RoomHandler) handleServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, core.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, core.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, core.ErrRoomFull):
		c.JSON(http.StatusConflict, gin.H{"error": "room is full"})
	case errors.Is(err, core.ErrAlreadyInRoom):
		c.JSON(http.StatusConflict, gin.H{"error": "already in room"})
	case errors.Is(err, core.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid input"})
	case errors.Is(err, core.ErrInvalidRoomType):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid room type, check /room-types for valid options"})
	case errors.Is(err, core.ErrNameTooLong):
		c.JSON(http.StatusBadRequest, gin.H{"error": "room name must be 30 characters or less"})
	case errors.Is(err, core.ErrDescriptionLimitExceeded):
		c.JSON(http.StatusBadRequest, gin.H{"error": "description must be 100 characters or less"})
	case errors.Is(err, core.ErrPeopleLimitExceeded):
		c.JSON(http.StatusBadRequest, gin.H{"error": "people limit exceeds maximum on your sub"})
	case errors.Is(err, core.ErrRoomLimitReached):
		c.JSON(http.StatusForbidden, gin.H{"error": "you have reached your room creation limit"})
	case errors.Is(err, core.ErrEmailNotVerified):
		c.JSON(http.StatusForbidden, gin.H{"error": "email not verified"})
	case errors.Is(err, core.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	default:
		h.log.Error("internal error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

// GET /cities — список уникальных городов из существующих комнат
func (h *RoomHandler) GetCities(c *gin.Context) {
	cities, err := h.service.GetCities(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": cities})
}
