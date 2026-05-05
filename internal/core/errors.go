package core

import "errors"

var (
	// General errors
	ErrNotFound       = errors.New("resource not found")
	ErrDuplicateEmail = errors.New("email already exists")
	ErrInvalidInput   = errors.New("invalid input data")

	// Token errors
	ErrTokenExpired = errors.New("token expired")
	ErrInvalidToken = errors.New("invalid token")
	ErrUnauthorized = errors.New("unauthorized")

	// Users errors
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidBirthday    = errors.New("invalid birthday")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidGender      = errors.New("invalid gender")
	ErrEmailNotVerified   = errors.New("email not verified")

	// Update errors
	ErrNicknameIsEmpty = errors.New("nickname is empty")
	ErrUrlIsEmpty      = errors.New("url is empty")
	ErrPasswordIsEmpty = errors.New("password is empty")

	// Rooms errors
	ErrRoomFull                 = errors.New("room is full")
	ErrAlreadyInRoom            = errors.New("already in room")
	ErrInvalidRoomType          = errors.New("invalid room type")
	ErrDescriptionLimitExceeded = errors.New("description limit exceeded")
	ErrNameTooLong              = errors.New("room name must be 30 characters or less")
	ErrRoomLimitReached         = errors.New("room creation limit reached for your subscription")
	ErrPeopleLimitExceeded      = errors.New("people limit exceeds your subscription maximum")

	// ErrForbidden — пользователь существует, но не имеет прав на операцию
	ErrForbidden = errors.New("forbidden")
)
