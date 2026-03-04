package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client defines the contract for the Telegram Bot API.
type Client interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
	SetWebhook(ctx context.Context, url string) error
}

// Update represents an incoming Telegram webhook update.
type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

// Message represents a Telegram message.
type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
}

// User represents a Telegram user.
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
	IsBot     bool   `json:"is_bot"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // "private", "group", etc.
}

type botClient struct {
	token      string
	httpClient *http.Client
}

// NewBotClient creates a new Telegram Bot API client.
func NewBotClient(token string) Client {
	return &botClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *botClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	payload := map[string]any{
		"chat_id": chatID,
		"text":    text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling sendMessage: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating sendMessage request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram sendMessage returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *botClient) SetWebhook(ctx context.Context, webhookURL string) error {
	payload := map[string]any{"url": webhookURL}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling setWebhook: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/setWebhook", c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating setWebhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("setting telegram webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram setWebhook returned status %d", resp.StatusCode)
	}
	return nil
}

var _ Client = (*botClient)(nil)
