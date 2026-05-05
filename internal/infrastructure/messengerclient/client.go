package messengerclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	httpClient  *http.Client
	baseURL     string
	internalKey string
}

func New(baseURL, internalKey string) *Client {
	return &Client{
		httpClient:  &http.Client{Timeout: 2 * time.Second},
		baseURL:     baseURL,
		internalKey: internalKey,
	}
}

func (c *Client) post(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Key", c.internalKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("messenger returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("X-Internal-Key", c.internalKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	// 404 — не ошибка: комнаты/чата может уже не быть
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("messenger returned status %d", resp.StatusCode)
	}

	return nil
}

// OnRoomCreated создаёт чат-группу при создании комнаты.
func (c *Client) OnRoomCreated(ctx context.Context, roomID, leaderID int) error {
	return c.post(ctx, "/internal/chats", map[string]int{
		"room_id":   roomID,
		"leader_id": leaderID,
	})
}

// OnRoomDeleted удаляет чат-группу при удалении комнаты.
func (c *Client) OnRoomDeleted(ctx context.Context, roomID int) error {
	return c.delete(ctx, fmt.Sprintf("/internal/chats/%d", roomID))
}

// OnMemberJoined добавляет участника в чат при вступлении в комнату.
func (c *Client) OnMemberJoined(ctx context.Context, roomID, userID int) error {
	return c.post(ctx, fmt.Sprintf("/internal/chats/%d/members", roomID), map[string]int{
		"user_id": userID,
	})
}

// OnMemberLeft удаляет участника из чата при выходе из комнаты.
func (c *Client) OnMemberLeft(ctx context.Context, roomID, userID int) error {
	return c.delete(ctx, fmt.Sprintf("/internal/chats/%d/members/%d", roomID, userID))
}

// OnLeaderDeleted удаляет все чаты пользователя-лидера при удалении аккаунта.
func (c *Client) OnLeaderDeleted(ctx context.Context, leaderID int) error {
	return c.delete(ctx, fmt.Sprintf("/internal/leaders/%d/chats", leaderID))
}
