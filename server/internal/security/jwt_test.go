//go:build ignore

package security

import (
	"testing"
	"time"

	"backupx/server/internal/model"
)

func TestJWTManagerIssueAndParse(t *testing.T) {
	manager := NewJWTManager("test-secret", time.Hour)
	token, err := manager.IssueToken(&model.User{ID: 7, Username: "admin", Role: "admin"})
	if err != nil {
		t.Fatalf("IssueToken() error = %v", err)
	}
	claims, err := manager.Parse(token)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.UserID != 7 || claims.Username != "admin" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}
