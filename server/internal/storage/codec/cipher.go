package codec

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

const maskedValue = "********"

type ConfigCipher struct {
	key []byte
}

type Cipher = ConfigCipher

func New(secret string) *ConfigCipher {
	hash := sha256.Sum256([]byte(secret))
	return &ConfigCipher{key: hash[:]}
}

func NewConfigCipher(secret string) *ConfigCipher {
	return New(secret)
}

func (c *ConfigCipher) Key() []byte {
	copyKey := make([]byte, len(c.key))
	copy(copyKey, c.key)
	return copyKey
}

func (c *ConfigCipher) Encrypt(raw []byte) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, raw, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *ConfigCipher) Decrypt(encoded string) ([]byte, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	if len(payload) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt ciphertext: %w", err)
	}
	return plain, nil
}

func (c *ConfigCipher) EncryptJSON(value any) (string, error) {
	plain, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal plaintext: %w", err)
	}
	return c.Encrypt(plain)
}

func (c *ConfigCipher) DecryptJSON(encoded string, out any) error {
	plain, err := c.Decrypt(encoded)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(plain, out); err != nil {
		return fmt.Errorf("unmarshal plaintext: %w", err)
	}
	return nil
}

func (c *ConfigCipher) EncryptValue(value any) (string, error) {
	return c.EncryptJSON(value)
}

func (c *ConfigCipher) DecryptValue(encoded string, out any) error {
	return c.DecryptJSON(encoded, out)
}

func MaskConfig(raw map[string]any, sensitiveFields []string) map[string]any {
	masked := cloneMap(raw)
	for _, field := range sensitiveFields {
		value, ok := masked[field]
		if !ok {
			continue
		}
		switch actual := value.(type) {
		case string:
			if actual != "" {
				masked[field] = maskedValue
			}
		default:
			masked[field] = maskedValue
		}
	}
	return masked
}

func MergeMaskedConfig(next map[string]any, existing map[string]any, sensitiveFields []string) map[string]any {
	merged := cloneMap(existing)
	for key, value := range next {
		merged[key] = value
	}
	for _, field := range sensitiveFields {
		value, ok := merged[field]
		if !ok {
			continue
		}
		switch actual := value.(type) {
		case string:
			if actual == "" || actual == maskedValue {
				merged[field] = existing[field]
			}
		}
	}
	return merged
}

func IsMaskedString(value string) bool {
	return bytes.Equal([]byte(value), []byte(maskedValue))
}

func cloneMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
