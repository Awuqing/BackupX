package security

import "testing"

func TestHashAndComparePassword(t *testing.T) {
	hash, err := HashPassword("super-secret-password")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "super-secret-password" {
		t.Fatalf("expected hashed password to differ from plain text")
	}
	if err := ComparePassword(hash, "super-secret-password"); err != nil {
		t.Fatalf("ComparePassword returned error: %v", err)
	}
}
