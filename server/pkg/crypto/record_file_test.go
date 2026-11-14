package backupcrypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordEncryption(t *testing.T) {
	master := bytes.Repeat([]byte{7}, 32)
	key := RecordKey(master, 42)
	if bytes.Equal(key, master) || bytes.Equal(key, RecordKey(master, 43)) {
		t.Fatal("keys are not isolated")
	}
	for _, size := range []int{0, 17, chunkSize + 29} {
		payload := bytes.Repeat([]byte{9}, size)
		src := filepath.Join(t.TempDir(), "backup.tar")
		if err := os.WriteFile(src, payload, 0600); err != nil {
			t.Fatal(err)
		}
		enc, err := EncryptRecordFile(key, 42, src)
		if err != nil {
			t.Fatal(err)
		}
		encrypted, err := os.ReadFile(enc)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecryptRecordFile(key, 43, enc); err == nil {
			t.Fatal("accepted another record")
		}
		if _, err := DecryptRecordFile(RecordKey(master, 43), 42, enc); err == nil {
			t.Fatal("accepted wrong key")
		}
		plain, err := DecryptFile(master, enc)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(plain)
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("round trip failed: %v", err)
		}
		for _, corrupt := range [][]byte{encrypted[:len(encrypted)-28], append(append([]byte{}, encrypted...), 0), append([]byte{}, encrypted...)} {
			if len(corrupt) == len(encrypted) {
				corrupt[len(corrupt)-1] ^= 1
			}
			if err := os.WriteFile(enc, corrupt, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := DecryptRecordFile(key, 42, enc); err == nil {
				t.Fatal("accepted corrupt/truncated ciphertext")
			}
			if _, err := os.Stat(src); !os.IsNotExist(err) {
				t.Fatal("failed decryption retained plaintext")
			}
		}
	}
}
