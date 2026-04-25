package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SMSWebhookNotifier struct {
	client *http.Client
}

func NewSMSWebhookNotifier() *SMSWebhookNotifier {
	return &SMSWebhookNotifier{client: &http.Client{
		Timeout:       10 * time.Second,
		Transport:     newSMSWebhookTransport(),
		CheckRedirect: validateSMSWebhookRedirect,
	}}
}

func (n *SMSWebhookNotifier) Type() string              { return "sms" }
func (n *SMSWebhookNotifier) SensitiveFields() []string { return []string{"secret"} }

func (n *SMSWebhookNotifier) Validate(config map[string]any) error {
	if _, err := validateSMSWebhookURL(asString(config["url"])); err != nil {
		return err
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
	endpoint, err := validateSMSWebhookURL(asString(config["url"]))
	if err != nil {
		return err
	}

	// codeql[go/request-forgery]: SMS webhook URLs are admin-configured and validated by validateSMSWebhookURL before use.
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
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

func validateSMSWebhookRedirect(request *http.Request, _ []*http.Request) error {
	_, err := validateSMSWebhookURL(request.URL.String())
	return err
}

func newSMSWebhookTransport() *http.Transport {
	return &http.Transport{
		DialContext:           dialSMSWebhookContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
	}
}

func dialSMSWebhookContext(ctx context.Context, network string, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ip, err := resolvePublicSMSWebhookIP(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
}

func resolvePublicSMSWebhookIP(ctx context.Context, hostname string) (net.IP, error) {
	host := strings.TrimSpace(hostname)
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicSMSWebhookIP(ip) {
			return nil, fmt.Errorf("sms webhook host must resolve to a public address")
		}
		return ip, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve sms webhook host: %w", err)
	}
	for _, address := range addresses {
		if !isPublicSMSWebhookIP(address.IP) {
			return nil, fmt.Errorf("sms webhook host must resolve to a public address")
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("sms webhook host did not resolve")
	}
	return addresses[0].IP, nil
}

func validateSMSWebhookURL(raw string) (string, error) {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return "", fmt.Errorf("sms webhook url is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("sms webhook url is invalid: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", fmt.Errorf("sms webhook url must use https")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("sms webhook url must not include user info")
	}
	if parsed.Hostname() == "" {
		return "", fmt.Errorf("sms webhook host is required")
	}
	if err := validateSMSWebhookHost(parsed.Hostname()); err != nil {
		return "", err
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}

func validateSMSWebhookHost(hostname string) error {
	host := strings.Trim(strings.ToLower(strings.TrimSpace(hostname)), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("sms webhook host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && !isPublicSMSWebhookIP(ip) {
		return fmt.Errorf("sms webhook host must resolve to a public address")
	}
	return nil
}

func isPublicSMSWebhookIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() &&
		!ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}
