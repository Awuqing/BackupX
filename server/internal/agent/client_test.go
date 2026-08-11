package agent

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMasterClientKeepsEnvironmentProxySupport(t *testing.T) {
	client := NewMasterClient("https://master.example.com", "token", false)
	transport := client.httpClient.Transport.(*http.Transport)
	if transport.Proxy == nil {
		t.Fatal("default transport proxy function must be preserved")
	}
}

func TestMasterClientConfiguresExplicitProxy(t *testing.T) {
	client := NewMasterClient("https://master.example.com", "token", false)
	if err := client.ConfigureTransport("socks5h://127.0.0.1:1080", ""); err != nil {
		t.Fatal(err)
	}
	transport := client.httpClient.Transport.(*http.Transport)
	requestURL, _ := url.Parse("https://master.example.com")
	proxyURL, err := transport.Proxy(&http.Request{URL: requestURL})
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL == nil || proxyURL.String() != "socks5h://127.0.0.1:1080" {
		t.Fatalf("proxy URL = %v", proxyURL)
	}
}

func TestMasterClientRejectsInvalidCACertificate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := NewMasterClient("https://master.example.com", "token", false)
	if err := client.ConfigureTransport("", path); err == nil {
		t.Fatal("expected invalid CA certificate error")
	}
}

func TestMasterClientDoesNotForwardTokenThroughRedirects(t *testing.T) {
	receivedToken := ""
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("X-Agent-Token")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()

	client := NewMasterClient(redirector.URL, "secret-agent-token", false)
	if _, err := client.Heartbeat(context.Background(), HeartbeatRequest{}); err == nil {
		t.Fatal("redirect response should not be accepted as a Master API response")
	}
	if receivedToken != "" {
		t.Fatalf("Agent token leaked through redirect: %q", receivedToken)
	}
}

func TestHeartbeatSendsTokenOnlyInAuthenticationHeader(t *testing.T) {
	requestBody := ""
	receivedHeader := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requestBody = string(body)
		receivedHeader = r.Header.Get("X-Agent-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"status":"ok","nodeId":1,"name":"node"}}`))
	}))
	defer server.Close()

	client := NewMasterClient(server.URL, "secret-agent-token", false)
	if _, err := client.Heartbeat(context.Background(), HeartbeatRequest{Hostname: "node"}); err != nil {
		t.Fatal(err)
	}
	if receivedHeader != "secret-agent-token" {
		t.Fatalf("authentication header = %q", receivedHeader)
	}
	if strings.Contains(requestBody, "secret-agent-token") || strings.Contains(requestBody, `"token"`) {
		t.Fatalf("heartbeat body exposed the Agent token: %s", requestBody)
	}
}
