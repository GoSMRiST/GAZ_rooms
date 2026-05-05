package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"rooms/internal/core"
)

type DataBase struct {
	log  *slog.Logger
	pool *pgxpool.Pool
}

func InitDataBase(log *slog.Logger, ctx context.Context, connString string) (*DataBase, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return &DataBase{
		log:  log,
		pool: pool,
	}, nil
}

func (db *DataBase) Pool() *pgxpool.Pool {
	return db.pool
}

func (db *DataBase) Close() {
	db.pool.Close()
}

func (db *DataBase) GetRoomTypes(ctx context.Context) ([]core.RoomType, error) {
	rows, err := db.pool.Query(ctx, `SELECT id, name FROM room_types ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("GetRoomTypes: %w", err)
	}
	defer rows.Close()

	types := make([]core.RoomType, 0)
	for rows.Next() {
		var t core.RoomType
		if err := rows.Scan(&t.ID, &t.Name); err != nil {
			return nil, fmt.Errorf("GetRoomTypes: %w", err)
		}
		types = append(types, t)
	}

	return types, rows.Err()
}

func (db *DataBase) GetRoomTypeByName(ctx context.Context, name string) (*core.RoomType, error) {
	var t core.RoomType
	err := db.pool.QueryRow(ctx, `SELECT id, name FROM room_types WHERE name = $1`, name).
		Scan(&t.ID, &t.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrInvalidRoomType
		}
		return nil, fmt.Errorf("GetRoomTypeByName: %w", err)
	}
	return &t, nil
}

func (db *DataBase) GetCities(ctx context.Context) ([]string, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT DISTINCT city FROM rooms
		WHERE city != '' ORDER BY city
	`)
	if err != nil {
		return nil, fmt.Errorf("GetCities: %w", err)
	}
	defer rows.Close()
	var cities []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cities = append(cities, c)
	}
	return cities, rows.Err()
}

func (db *DataBase) CreateRoom(ctx context.Context, room *core.Room) (*core.Room, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	resp := &core.Room{}

	err = tx.QueryRow(ctx, `
		INSERT INTO rooms (name, type_id, description, max_people, leader_id, city)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, type_id, description, max_people, leader_id, city, created_at;
	`,
		room.RoomName,
		room.RoomTypeID,
		room.RoomDesc,
		room.PeopleLimit,
		room.UserID,
		room.City,
	).Scan(
		&resp.RoomID,
		&resp.RoomName,
		&resp.RoomTypeID,
		&resp.RoomDesc,
		&resp.PeopleLimit,
		&resp.UserID,
		&resp.City,
		&resp.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateRoom: %w", err)
	}

	resp.RoomType = room.RoomType // название уже резолвнуто в сервисе
	resp.ParticipantsCount = 1

	_, err = tx.Exec(ctx, `
		INSERT INTO room_participants (room_id, user_id, role)
		VALUES ($1, $2, 'leader');
	`, resp.RoomID, room.UserID)
	if err != nil {
		return nil, fmt.Errorf("CreateRoom insert participant: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("CreateRoom commit: %w", err)
	}

	return resp, nil
}

func (db *DataBase) GetRoom(ctx context.Context, roomId int) (*core.Room, error) {
	resp := &core.Room{}

	err := db.pool.QueryRow(ctx, `
		SELECT
			r.id,
			r.name,
			rt.name,
			rt.id,
			r.description,
			r.max_people,
			r.leader_id,
			r.city,
			r.created_at,
			COUNT(rp.user_id) AS participants_count
		FROM rooms r
		JOIN room_types rt ON rt.id = r.type_id
		LEFT JOIN room_participants rp ON rp.room_id = r.id
		WHERE r.id = $1
		GROUP BY r.id, rt.id, rt.name;
	`, roomId).Scan(
		&resp.RoomID,
		&resp.RoomName,
		&resp.RoomType,
		&resp.RoomTypeID,
		&resp.RoomDesc,
		&resp.PeopleLimit,
		&resp.UserID,
		&resp.City,
		&resp.CreatedAt,
		&resp.ParticipantsCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, fmt.Errorf("GetRoom: %w", err)
	}

	return resp, nil
}

func (db *DataBase) ListRooms(ctx context.Context, filters *core.Filters) ([]core.Room, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT
			r.id,
			r.name,
			rt.name,
			rt.id,
			r.description,
			r.max_people,
			r.leader_id,
			r.city,
			r.created_at,
			COUNT(rp.user_id) AS participants_count
		FROM rooms r
		JOIN room_types rt ON rt.id = r.type_id
		LEFT JOIN room_participants rp ON rp.room_id = r.id
		WHERE ($1::text IS NULL OR r.city = $1)
		  AND ($2::text IS NULL OR rt.name = $2)
		  AND ($3::int = 0 OR r.id < $3)
		GROUP BY r.id, rt.id, rt.name
		ORDER BY r.id DESC
		LIMIT $4;
	`,
		filters.City,
		filters.RoomType,
		filters.LastID,
		filters.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("ListRooms: %w", err)
	}
	defer rows.Close()

	rooms := make([]core.Room, 0)
	for rows.Next() {
		var r core.Room
		if err := rows.Scan(
			&r.RoomID,
			&r.RoomName,
			&r.RoomType,
			&r.RoomTypeID,
			&r.RoomDesc,
			&r.PeopleLimit,
			&r.UserID,
			&r.City,
			&r.CreatedAt,
			&r.ParticipantsCount,
		); err != nil {
			return nil, fmt.Errorf("ListRooms scan: %w", err)
		}
		rooms = append(rooms, r)
	}

	return rooms, rows.Err()
}

func (db *DataBase) GetUserRooms(ctx context.Context, userId int) ([]core.Room, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT
			r.id,
			r.name,
			rt.name,
			rt.id,
			r.description,
			r.max_people,
			r.leader_id,
			r.city,
			r.created_at,
			COUNT(rp2.user_id) AS participants_count
		FROM room_participants rp
		JOIN rooms r ON r.id = rp.room_id
		JOIN room_types rt ON rt.id = r.type_id
		LEFT JOIN room_participants rp2 ON rp2.room_id = r.id
		WHERE rp.user_id = $1
		GROUP BY r.id, rt.id, rt.name
		ORDER BY r.created_at DESC;
	`, userId)
	if err != nil {
		return nil, fmt.Errorf("GetUserRooms: %w", err)
	}
	defer rows.Close()

	rooms := make([]core.Room, 0)
	for rows.Next() {
		var r core.Room
		if err := rows.Scan(
			&r.RoomID,
			&r.RoomName,
			&r.RoomType,
			&r.RoomTypeID,
			&r.RoomDesc,
			&r.PeopleLimit,
			&r.UserID,
			&r.City,
			&r.CreatedAt,
			&r.ParticipantsCount,
		); err != nil {
			return nil, fmt.Errorf("GetUserRooms scan: %w", err)
		}
		rooms = append(rooms, r)
	}

	return rooms, rows.Err()
}

func (db *DataBase) GetRoomParticipants(ctx context.Context, roomId int) ([]core.Participant, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT user_id, role, joined_at
		FROM room_participants
		WHERE room_id = $1;
	`, roomId)
	if err != nil {
		return nil, fmt.Errorf("GetRoomParticipants: %w", err)
	}
	defer rows.Close()

	participants := make([]core.Participant, 0)
	for rows.Next() {
		var p core.Participant
		if err := rows.Scan(&p.UserId, &p.Role, &p.JoinedAt); err != nil {
			return nil, fmt.Errorf("GetRoomParticipants scan: %w", err)
		}
		participants = append(participants, p)
	}

	return participants, rows.Err()
}

func (db *DataBase) JoinRoom(ctx context.Context, roomID int, userID int) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var maxPeople int
	err = tx.QueryRow(ctx, `
		SELECT max_people FROM rooms WHERE id = $1 FOR UPDATE;
	`, roomID).Scan(&maxPeople)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrNotFound
		}
		return fmt.Errorf("JoinRoom get room: %w", err)
	}

	var count int
	if err = tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM room_participants WHERE room_id = $1;
	`, roomID).Scan(&count); err != nil {
		return fmt.Errorf("JoinRoom count: %w", err)
	}

	if count >= maxPeople {
		return core.ErrRoomFull
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO room_participants (room_id, user_id, role)
		VALUES ($1, $2, 'member');
	`, roomID, userID)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return core.ErrAlreadyInRoom
		}
		return fmt.Errorf("JoinRoom insert: %w", err)
	}

	return tx.Commit(ctx)
}

func (db *DataBase) LeaveRoom(ctx context.Context, roomID int, userID int) error {
	res, err := db.pool.Exec(ctx, `
		DELETE FROM room_participants
		WHERE room_id = $1 AND user_id = $2
		  AND role != 'leader';
	`, roomID, userID)
	if err != nil {
		return fmt.Errorf("LeaveRoom: %w", err)
	}

	if res.RowsAffected() == 0 {
		// Либо не в комнате, либо является лидером — возвращаем NotFound,
		// лидер должен использовать DELETE /rooms/:id
		return core.ErrNotFound
	}
	return nil
}

func (db *DataBase) DeleteRoom(ctx context.Context, roomID int, userID int) error {
	var leaderID int
	err := db.pool.QueryRow(ctx, `SELECT leader_id FROM rooms WHERE id = $1`, roomID).Scan(&leaderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrNotFound
		}
		return fmt.Errorf("DeleteRoom get leader: %w", err)
	}

	if leaderID != userID {
		return core.ErrForbidden
	}

	_, err = db.pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, roomID)
	if err != nil {
		return fmt.Errorf("DeleteRoom: %w", err)
	}

	return nil
}

func (db *DataBase) UpdateRoom(ctx context.Context, room *core.Room) (*core.Room, error) {
	var leaderID int
	err := db.pool.QueryRow(ctx, `SELECT leader_id FROM rooms WHERE id = $1`, room.RoomID).Scan(&leaderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, fmt.Errorf("UpdateRoom get leader: %w", err)
	}
	if leaderID != room.UserID {
		return nil, core.ErrForbidden
	}

	resp := &core.Room{}

	err = db.pool.QueryRow(ctx, `
		WITH updated AS (
			UPDATE rooms
			SET name        = $1,
			    type_id     = $2,
			    description = $3,
			    max_people  = $4,
			    city        = $5
			WHERE id = $6
			RETURNING id, name, type_id, description, max_people, leader_id, city, created_at
		)
		SELECT
			u.id,
			u.name,
			rt.name,
			rt.id,
			u.description,
			u.max_people,
			u.leader_id,
			u.city,
			u.created_at,
			COUNT(rp.user_id) AS participants_count
		FROM updated u
		JOIN room_types rt ON rt.id = u.type_id
		LEFT JOIN room_participants rp ON rp.room_id = u.id
		GROUP BY u.id, u.name, rt.name, rt.id, u.description, u.max_people, u.leader_id, u.city, u.created_at;
	`,
		room.RoomName,
		room.RoomTypeID,
		room.RoomDesc,
		room.PeopleLimit,
		room.City,
		room.RoomID,
	).Scan(
		&resp.RoomID,
		&resp.RoomName,
		&resp.RoomType,
		&resp.RoomTypeID,
		&resp.RoomDesc,
		&resp.PeopleLimit,
		&resp.UserID,
		&resp.City,
		&resp.CreatedAt,
		&resp.ParticipantsCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, fmt.Errorf("UpdateRoom: %w", err)
	}

	resp.RoomType = room.RoomType

	return resp, nil
}

func (db *DataBase) GetCountParticipants(ctx context.Context, roomID int) (int, error) {
	var count int

	err := db.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM room_participants WHERE room_id = $1;
	`, roomID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("GetCountParticipants: %w", err)
	}

	return count, nil
}

func (db *DataBase) DeleteRoomsByUser(ctx context.Context, userID int) (int64, error) {
	result, err := db.pool.Exec(ctx, `
		DELETE FROM rooms WHERE leader_id = $1;
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("DeleteRoomsByUser: %w", err)
	}

	return result.RowsAffected(), nil
}

func (db *DataBase) IsUserInRoom(ctx context.Context, roomID, userID int) (bool, error) {
	var exists bool
	err := db.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM room_participants
			WHERE room_id = $1 AND user_id = $2
		)
	`, roomID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("IsUserInRoom: %w", err)
	}

	return exists, nil
}

func (db *DataBase) CountRoomsByLeader(ctx context.Context, userID int) (int, error) {
	var count int
	err := db.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM rooms WHERE leader_id = $1
	`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("CountRoomsByLeader: %w", err)
	}

	return count, nil
}
