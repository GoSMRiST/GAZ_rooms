package service

import (
	"context"
	"errors"
	"log/slog"
	"rooms/internal/core"
)

type RepositoryInterface interface {
	GetRoomTypes(ctx context.Context) ([]core.RoomType, error)
	GetRoomTypeByName(ctx context.Context, name string) (*core.RoomType, error)
	GetCities(ctx context.Context) ([]string, error)

	CreateRoom(ctx context.Context, room *core.Room) (*core.Room, error)
	GetRoom(ctx context.Context, roomId int) (*core.Room, error)
	ListRooms(ctx context.Context, filters *core.Filters) ([]core.Room, error)
	GetUserRooms(ctx context.Context, userId int) ([]core.Room, error)
	GetRoomParticipants(ctx context.Context, roomId int) ([]core.Participant, error)
	JoinRoom(ctx context.Context, roomID int, userID int) error
	LeaveRoom(ctx context.Context, roomID int, userID int) error
	DeleteRoom(ctx context.Context, roomID int, userID int) error
	UpdateRoom(ctx context.Context, room *core.Room) (*core.Room, error)
	GetCountParticipants(ctx context.Context, roomID int) (int, error)
	DeleteRoomsByUser(ctx context.Context, userID int) (int64, error)
	IsUserInRoom(ctx context.Context, roomID, userID int) (bool, error)
	CountRoomsByLeader(ctx context.Context, userID int) (int, error)
}

type UserVerifier interface {
	GetUserInfo(ctx context.Context, userID int) (verified bool, subscription core.Subscription, err error)
}

type MessengerNotifier interface {
	OnRoomCreated(ctx context.Context, roomID, leaderID int) error
	OnRoomDeleted(ctx context.Context, roomID int) error
	OnMemberJoined(ctx context.Context, roomID, userID int) error
	OnMemberLeft(ctx context.Context, roomID, userID int) error
	OnLeaderDeleted(ctx context.Context, leaderID int) error
}

type noopMessenger struct{}

func (noopMessenger) OnRoomCreated(_ context.Context, _, _ int) error  { return nil }
func (noopMessenger) OnRoomDeleted(_ context.Context, _ int) error     { return nil }
func (noopMessenger) OnMemberJoined(_ context.Context, _, _ int) error { return nil }
func (noopMessenger) OnMemberLeft(_ context.Context, _, _ int) error   { return nil }
func (noopMessenger) OnLeaderDeleted(_ context.Context, _ int) error   { return nil }

type RoomService struct {
	log       *slog.Logger
	repo      RepositoryInterface
	verifier  UserVerifier
	messenger MessengerNotifier
}

func NewRoomService(log *slog.Logger, repo RepositoryInterface, verifier UserVerifier, notifier MessengerNotifier) *RoomService {
	if notifier == nil {
		notifier = noopMessenger{}
	}
	return &RoomService{log: log, repo: repo, verifier: verifier, messenger: notifier}
}

func (s *RoomService) notifyMessenger(ctx context.Context, action string, fn func() error) {
	if err := fn(); err != nil {
		s.log.Warn("messenger notification failed", "action", action, "err", err)
	}
}

func (s *RoomService) GetRoomTypes(ctx context.Context) ([]core.RoomType, error) {
	return s.repo.GetRoomTypes(ctx)
}

func (s *RoomService) GetCities(ctx context.Context) ([]string, error) {
	return s.repo.GetCities(ctx)
}

func (s *RoomService) resolveRoomType(ctx context.Context, typeName string) (*core.RoomType, error) {
	return s.repo.GetRoomTypeByName(ctx, typeName)
}

func (s *RoomService) CreateRoom(ctx context.Context, room *core.Room) (*core.Room, error) {
	if room.RoomName == "" || room.RoomType == "" || room.City == "" || room.PeopleLimit <= 0 {
		return nil, core.ErrInvalidInput
	}

	if len([]rune(room.RoomName)) > 30 {
		return nil, core.ErrNameTooLong
	}

	if len([]rune(room.RoomDesc)) > 100 {
		return nil, core.ErrDescriptionLimitExceeded
	}

	// Получаем верификацию и подписку одним gRPC вызовом
	verified, sub, err := s.verifier.GetUserInfo(ctx, room.UserID)
	if err != nil {
		s.log.Error("CreateRoom: failed to get user info", "error", err)
		return nil, err
	}
	if !verified {
		return nil, core.ErrEmailNotVerified
	}

	// Лимит людей по подписке
	peopleLimit, ok := core.PeopleLimits[sub]
	if !ok {
		peopleLimit = core.PeopleLimits[core.SubscriptionDefault]
	}
	if room.PeopleLimit > peopleLimit {
		return nil, core.ErrPeopleLimitExceeded
	}

	// Лимит создания комнат по подписке
	roomLimit, ok := core.RoomCreationLimits[sub]
	if !ok {
		roomLimit = core.RoomCreationLimits[core.SubscriptionDefault]
	}
	count, err := s.repo.CountRoomsByLeader(ctx, room.UserID)
	if err != nil {
		s.log.Error("CreateRoom: failed to count rooms", "error", err)
		return nil, err
	}
	if count >= roomLimit {
		return nil, core.ErrRoomLimitReached
	}

	rt, err := s.resolveRoomType(ctx, room.RoomType)
	if err != nil {
		return nil, err
	}
	room.RoomTypeID = rt.ID

	created, err := s.repo.CreateRoom(ctx, room)
	if err != nil {
		s.log.Error("CreateRoom: repo error", "error", err)
		return nil, err
	}

	s.notifyMessenger(ctx, "OnRoomCreated", func() error {
		return s.messenger.OnRoomCreated(ctx, created.RoomID, created.UserID)
	})

	return created, nil
}

func (s *RoomService) GetRoom(ctx context.Context, roomID int) (*core.Room, error) {
	room, err := s.repo.GetRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return nil, core.ErrNotFound
		}
		s.log.Error("GetRoom: repo error", "error", err)
		return nil, err
	}

	return room, nil
}

func (s *RoomService) ListRooms(ctx context.Context, filters *core.Filters) ([]core.Room, error) {
	if filters.Limit <= 0 {
		filters.Limit = 20
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}
	if filters.RoomType != nil {
		if _, err := s.repo.GetRoomTypeByName(ctx, *filters.RoomType); err != nil {
			return nil, core.ErrInvalidRoomType
		}
	}
	rooms, err := s.repo.ListRooms(ctx, filters)
	if err != nil {
		s.log.Error("ListRooms: repo error", "error", err)
		return nil, err
	}

	return rooms, nil
}

func (s *RoomService) GetUserRooms(ctx context.Context, userID int) ([]core.Room, error) {
	rooms, err := s.repo.GetUserRooms(ctx, userID)
	if err != nil {
		s.log.Error("GetUserRooms: repo error", "error", err)
		return nil, err
	}

	return rooms, nil
}

func (s *RoomService) GetRoomParticipants(ctx context.Context, roomID int) ([]core.Participant, error) {
	participants, err := s.repo.GetRoomParticipants(ctx, roomID)
	if err != nil {
		s.log.Error("GetRoomParticipants: repo error", "error", err)
		return nil, err
	}

	return participants, nil
}

func (s *RoomService) JoinRoom(ctx context.Context, roomID int, userID int) error {
	err := s.repo.JoinRoom(ctx, roomID, userID)
	if err != nil {
		if errors.Is(err, core.ErrRoomFull) || errors.Is(err, core.ErrAlreadyInRoom) || errors.Is(err, core.ErrNotFound) {
			return err
		}
		s.log.Error("JoinRoom: repo error", "error", err)
		return err
	}

	s.notifyMessenger(ctx, "OnMemberJoined", func() error {
		return s.messenger.OnMemberJoined(ctx, roomID, userID)
	})

	return nil
}

func (s *RoomService) LeaveRoom(ctx context.Context, roomID int, userID int) error {
	err := s.repo.LeaveRoom(ctx, roomID, userID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return core.ErrNotFound
		}
		s.log.Error("LeaveRoom: repo error", "error", err)
		return err
	}

	s.notifyMessenger(ctx, "OnMemberLeft", func() error {
		return s.messenger.OnMemberLeft(ctx, roomID, userID)
	})

	return nil
}

func (s *RoomService) DeleteRoom(ctx context.Context, roomID int, userID int) error {
	err := s.repo.DeleteRoom(ctx, roomID, userID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrForbidden) {
			return err
		}
		s.log.Error("DeleteRoom: repo error", "error", err)
		return err
	}

	s.notifyMessenger(ctx, "OnRoomDeleted", func() error {
		return s.messenger.OnRoomDeleted(ctx, roomID)
	})

	return nil
}

func (s *RoomService) UpdateRoom(ctx context.Context, room *core.Room) (*core.Room, error) {
	if room.RoomName == "" || room.RoomType == "" || room.City == "" || room.PeopleLimit <= 0 {
		return nil, core.ErrInvalidInput
	}

	if len([]rune(room.RoomName)) > 30 {
		return nil, core.ErrNameTooLong
	}

	if len([]rune(room.RoomDesc)) > 100 {
		return nil, core.ErrDescriptionLimitExceeded
	}

	_, sub, err := s.verifier.GetUserInfo(ctx, room.UserID)
	if err != nil {
		s.log.Error("UpdateRoom: failed to get user info", "error", err)
		return nil, err
	}

	peopleLimit, ok := core.PeopleLimits[sub]
	if !ok {
		peopleLimit = core.PeopleLimits[core.SubscriptionDefault]
	}

	if room.PeopleLimit > peopleLimit {
		return nil, core.ErrPeopleLimitExceeded
	}

	rt, err := s.resolveRoomType(ctx, room.RoomType)
	if err != nil {
		return nil, err
	}

	room.RoomTypeID = rt.ID

	count, err := s.repo.GetCountParticipants(ctx, room.RoomID)
	if err != nil {
		return nil, err
	}
	if room.PeopleLimit < count {
		return nil, core.ErrInvalidInput
	}

	updated, err := s.repo.UpdateRoom(ctx, room)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrForbidden) {
			return nil, err
		}
		s.log.Error("UpdateRoom: repo error", "error", err)
		return nil, err
	}

	return updated, nil
}

func (s *RoomService) DeleteRoomsByUser(ctx context.Context, userID int) (int64, error) {
	count, err := s.repo.DeleteRoomsByUser(ctx, userID)
	if err != nil {
		s.log.Error("DeleteRoomsByUser: repo error", "error", err)
		return 0, err
	}
	s.log.Info("DeleteRoomsByUser: completed", "user_id", userID, "deleted", count)

	if count > 0 {
		s.notifyMessenger(ctx, "OnLeaderDeleted", func() error {
			return s.messenger.OnLeaderDeleted(ctx, userID)
		})
	}

	return count, nil
}

func (s *RoomService) IsUserInRoom(ctx context.Context, roomID, userID int) (bool, error) {
	ok, err := s.repo.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		s.log.Error("IsUserInRoom: repo error", "error", err)
		return false, err
	}
	return ok, nil
}
