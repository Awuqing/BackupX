package codec

import (
	"bytes"
	"testing"
)

func TestCipherEncryptAndDecrypt(t *testing.T) {
	cipher := New("encryption-secret")
	input := map[string]any{
		"endpoint": "https://example.com",
		"secret":   "top-secret",
	}
	encoded, err := cipher.EncryptValue(input)
	if err != nil {
		t.Fatalf("EncryptValue returned error: %v", err)
	}
	var output map[string]any
	if err := cipher.DecryptValue(encoded, &output); err != nil {
		t.Fatalf("DecryptValue returned error: %v", err)
	}
	if output["secret"] != "top-secret" {
		t.Fatalf("expected decrypted secret, got %#v", output["secret"])
	}
}

func TestConfigCipherEncryptAndDecryptBytes(t *testing.T) {
	cipher := NewConfigCipher("encryption-secret")
	encoded, err := cipher.Encrypt([]byte(`{"bucket":"demo"}`))
	if err != nil {
		t.Fatalf("Encrypt returned error: %v", err)
	}
	decoded, err := cipher.Decrypt(encoded)
	if err != nil {
		t.Fatalf("Decrypt returned error: %v", err)
	}
	if !bytes.Equal(decoded, []byte(`{"bucket":"demo"}`)) {
		t.Fatalf("expected decrypted payload to match, got %s", string(decoded))
	}
}

func TestMaskConfig(t *testing.T) {
	masked := MaskConfig(map[string]any{"secret": "abc", "bucket": "demo"}, []string{"secret"})
	if masked["secret"] != "********" {
		t.Fatalf("expected masked secret, got %#v", masked["secret"])
	}
	if masked["bucket"] != "demo" {
		t.Fatalf("expected bucket to remain unchanged")
	}
}

func TestMergeMaskedConfig(t *testing.T) {
	merged := MergeMaskedConfig(
		map[string]any{"bucket": "changed", "secret": "********"},
		map[string]any{"bucket": "demo", "secret": "top-secret"},
		[]string{"secret"},
	)
	if merged["bucket"] != "changed" {
		t.Fatalf("expected bucket to use new value, got %#v", merged["bucket"])
	}
	if merged["secret"] != "top-secret" {
		t.Fatalf("expected masked secret to reuse stored value, got %#v", merged["secret"])
	}
}
