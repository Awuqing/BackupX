package security

import (
	"testing"
	"time"

	"backupx/server/internal/model"
)

func TestJWTManagerGenerateAndParse(t *testing.T) {
	manager := NewJWTManager("test-secret", time.Hour)
	user := &model.User{ID: 7, Username: "admin", Role: "admin"}

	token, err := manager.Generate(user)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if claims.Subject != "7" {
		t.Fatalf("expected subject 7, got %s", claims.Subject)
	}
	if claims.Username != "admin" {
		t.Fatalf("expected username admin, got %s", claims.Username)
	}
}
