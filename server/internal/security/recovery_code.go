package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

const RecoveryCodeCount = 10

func GenerateRecoveryCodes(count int) ([]string, error) {
	if count <= 0 {
		count = RecoveryCodeCount
	}
	codes := make([]string, 0, count)
	for i := 0; i < count; i++ {
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			return nil, fmt.Errorf("generate recovery code: %w", err)
		}
		encoded := strings.ToUpper(hex.EncodeToString(raw))
		codes = append(codes, encoded[0:4]+"-"+encoded[4:8]+"-"+encoded[8:12]+"-"+encoded[12:16])
	}
	return codes, nil
}

func NormalizeRecoveryCode(code string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return unicode.ToUpper(r)
	}, strings.TrimSpace(code))
}

func IsRecoveryCodeCandidate(code string) bool {
	normalized := NormalizeRecoveryCode(code)
	if len(normalized) != 16 {
		return false
	}
	for _, r := range normalized {
		if !('0' <= r && r <= '9') && !('A' <= r && r <= 'F') {
			return false
		}
	}
	return true
}
