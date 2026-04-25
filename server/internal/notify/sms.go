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

type SMSWebhookNotifier struct {
	client *http.Client
}

func NewSMSWebhookNotifier() *SMSWebhookNotifier {
	return &SMSWebhookNotifier{client: &http.Client{Timeout: 10 * time.Second}}
}

func (n *SMSWebhookNotifier) Type() string              { return "sms" }
func (n *SMSWebhookNotifier) SensitiveFields() []string { return []string{"secret"} }

func (n *SMSWebhookNotifier) Validate(config map[string]any) error {
	if strings.TrimSpace(asString(config["url"])) == "" {
		return fmt.Errorf("sms webhook url is required")
	}
	return nil
}

func (n *SMSWebhookNotifier) Send(ctx context.Context, config map[string]any, message Message) error {
	if err := n.Validate(config); err != nil {
		return err
	}
	payload := map[string]any{
		"title":   message.Title,
		"body":    message.Body,
		"fields":  message.Fields,
		"phone":   message.Fields["phone"],
		"code":    message.Fields["code"],
		"purpose": message.Fields["purpose"],
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sms webhook payload: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSpace(asString(config["url"])), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create sms webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if secret := strings.TrimSpace(asString(config["secret"])); secret != "" {
		request.Header.Set("X-BackupX-Secret", secret)
	}
	response, err := n.client.Do(request)
	if err != nil {
		return fmt.Errorf("send sms webhook request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("sms webhook response status: %s", response.Status)
	}
	return nil
}
