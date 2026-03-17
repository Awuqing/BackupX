package backupcrypto

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptAndDecryptFile(t *testing.T) {
	hash := sha256.Sum256([]byte("backup-secret"))
	sourcePath := filepath.Join(t.TempDir(), "payload.bin")
	if err := os.WriteFile(sourcePath, []byte("backup-payload"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	encryptedPath, err := EncryptFile(hash[:], sourcePath)
	if err != nil {
		t.Fatalf("EncryptFile returned error: %v", err)
	}
	decryptedPath, err := DecryptFile(hash[:], encryptedPath)
	if err != nil {
		t.Fatalf("DecryptFile returned error: %v", err)
	}
	content, err := os.ReadFile(decryptedPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if string(content) != "backup-payload" {
		t.Fatalf("unexpected decrypted content: %s", string(content))
	}
}
