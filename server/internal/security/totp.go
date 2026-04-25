package security

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strings"
	"time"
	"unicode"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const TOTPIssuer = "BackupX"

type TOTPEnrollment struct {
	Secret        string
	OTPAuthURL    string
	QRCodeDataURL string
}

func GenerateTOTPEnrollment(accountName string) (*TOTPEnrollment, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TOTPIssuer,
		AccountName: accountName,
		Period:      30,
		SecretSize:  20,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, err
	}

	image, err := key.Image(220, 220)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image); err != nil {
		return nil, err
	}

	return &TOTPEnrollment{
		Secret:        key.Secret(),
		OTPAuthURL:    key.URL(),
		QRCodeDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}

func ValidateTOTPCode(secret string, code string) (bool, error) {
	return totp.ValidateCustom(NormalizeTOTPCode(code), secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
}

func NormalizeTOTPCode(code string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(code))
}
