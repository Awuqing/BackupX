package notify

import "context"

type Message struct {
	Title  string         `json:"title"`
	Body   string         `json:"body"`
	Fields map[string]any `json:"fields,omitempty"`
}

type Notifier interface {
	Type() string
	SensitiveFields() []string
	Validate(config map[string]any) error
	Send(ctx context.Context, config map[string]any, message Message) error
}
