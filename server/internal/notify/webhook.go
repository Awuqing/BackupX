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

type WebhookNotifier struct {
	client *http.Client
}

func NewWebhookNotifier() *WebhookNotifier {
	return &WebhookNotifier{client: &http.Client{Timeout: 10 * time.Second}}
}
func (n *WebhookNotifier) Type() string              { return "webhook" }
func (n *WebhookNotifier) SensitiveFields() []string { return []string{"secret"} }

func (n *WebhookNotifier) Validate(config map[string]any) error {
	if strings.TrimSpace(asString(config["url"])) == "" {
		return fmt.Errorf("webhook url is required")
	}
	return nil
}

func (n *WebhookNotifier) Send(ctx context.Context, config map[string]any, message Message) error {
	if err := n.Validate(config); err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{"title": message.Title, "body": message.Body, "fields": message.Fields})
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(asString(config["url"])), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if secret := strings.TrimSpace(asString(config["secret"])); secret != "" {
		request.Header.Set("X-BackupX-Secret", secret)
	}
	response, err := n.client.Do(request)
	if err != nil {
		return fmt.Errorf("send webhook request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("webhook response status: %s", response.Status)
	}
	return nil
}
