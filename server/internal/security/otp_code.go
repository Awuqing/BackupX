package security

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

const LoginOTPDigits = 6

func GenerateNumericOTP() (string, error) {
	limit := big.NewInt(1_000_000)
	value, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", LoginOTPDigits, value.Int64()), nil
}

func NormalizeNumericOTP(code string) string {
	return strings.TrimSpace(code)
}
