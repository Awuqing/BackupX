package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestForwardedHeadersMiddlewareRejectsUntrustedHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(ForwardedHeadersMiddleware([]string{"127.0.0.1", "10.0.0.0/8"}))
	engine.GET("/master-url", func(c *gin.Context) {
		c.String(stdhttp.StatusOK, resolveMasterURL(c, ""))
	})

	request := httptest.NewRequest(stdhttp.MethodGet, "http://master.example.com/master-url", nil)
	request.RemoteAddr = "203.0.113.10:54321"
	request.Header.Set("X-Forwarded-Host", "attacker.example.com")
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Body.String() != "http://master.example.com" {
		t.Fatalf("untrusted forwarding headers changed URL: %q", recorder.Body.String())
	}
}

func TestForwardedHeadersMiddlewareAcceptsTrustedProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(ForwardedHeadersMiddleware([]string{"10.0.0.0/8"}))
	engine.GET("/master-url", func(c *gin.Context) {
		c.String(stdhttp.StatusOK, resolveMasterURL(c, ""))
	})

	request := httptest.NewRequest(stdhttp.MethodGet, "http://backupx:8340/master-url", nil)
	request.RemoteAddr = "10.10.0.5:43210"
	request.Header.Set("X-Forwarded-Host", "backup.example.com")
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)

	if recorder.Body.String() != "https://backup.example.com" {
		t.Fatalf("trusted forwarding headers were ignored: %q", recorder.Body.String())
	}
}
