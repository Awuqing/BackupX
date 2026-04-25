package notify

import "testing"

func TestValidateSMSWebhookURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "valid https endpoint", raw: "https://sms.example.com/send?channel=otp"},
		{name: "reject empty", raw: "", wantErr: true},
		{name: "reject http", raw: "http://sms.example.com/send", wantErr: true},
		{name: "reject user info", raw: "https://user:pass@sms.example.com/send", wantErr: true},
		{name: "reject localhost", raw: "https://localhost/send", wantErr: true},
		{name: "reject private ipv4", raw: "https://192.168.1.10/send", wantErr: true},
		{name: "reject loopback ipv6", raw: "https://[::1]/send", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := validateSMSWebhookURL(tt.raw)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
