package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type TelegramNotifier struct {
	client *http.Client
}

func NewTelegramNotifier() *TelegramNotifier {
	return &TelegramNotifier{client: &http.Client{Timeout: 10 * time.Second}}
}
func (n *TelegramNotifier) Type() string              { return "telegram" }
func (n *TelegramNotifier) SensitiveFields() []string { return []string{"botToken"} }

func (n *TelegramNotifier) Validate(config map[string]any) error {
	if strings.TrimSpace(asString(config["botToken"])) == "" || strings.TrimSpace(asString(config["chatId"])) == "" {
		return fmt.Errorf("telegram botToken/chatId are required")
	}
	return nil
}

func (n *TelegramNotifier) Send(ctx context.Context, config map[string]any, message Message) error {
	if err := n.Validate(config); err != nil {
		return err
	}
	botToken := strings.TrimSpace(asString(config["botToken"]))
	chatID := strings.TrimSpace(asString(config["chatId"]))
	payload, err := json.Marshal(map[string]any{"chat_id": chatID, "text": message.Title + "\n\n" + message.Body})
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+botToken+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create telegram request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := n.client.Do(request)
	if err != nil {
		return fmt.Errorf("send telegram request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("telegram response status: %s", response.Status)
	}
	return nil
}
