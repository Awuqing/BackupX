package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

const (
	WebAuthnChallengeBytes = 32
)

type WebAuthnCredentialMaterial struct {
	CredentialID string
	PublicKeyX   string
	PublicKeyY   string
	SignCount    uint32
}

type WebAuthnParsedCredential struct {
	CredentialID string
	PublicKeyX   string
	PublicKeyY   string
	SignCount    uint32
}

type WebAuthnClientData struct {
	Type      string `json:"type"`
	Challenge string `json:"challenge"`
	Origin    string `json:"origin"`
}

type WebAuthnAttestationResponse struct {
	ClientDataJSON    string `json:"clientDataJSON"`
	AttestationObject string `json:"attestationObject"`
}

type WebAuthnRegistrationResponse struct {
	ID       string                      `json:"id"`
	RawID    string                      `json:"rawId"`
	Type     string                      `json:"type"`
	Response WebAuthnAttestationResponse `json:"response"`
}

type WebAuthnAssertionResponse struct {
	ClientDataJSON    string `json:"clientDataJSON"`
	AuthenticatorData string `json:"authenticatorData"`
	Signature         string `json:"signature"`
	UserHandle        string `json:"userHandle,omitempty"`
}

type WebAuthnLoginAssertion struct {
	ID       string                    `json:"id"`
	RawID    string                    `json:"rawId"`
	Type     string                    `json:"type"`
	Response WebAuthnAssertionResponse `json:"response"`
}

func GenerateWebAuthnChallenge() (string, error) {
	buf := make([]byte, WebAuthnChallengeBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return EncodeBase64URL(buf), nil
}

func EncodeBase64URL(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func DecodeBase64URL(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, errors.New("empty base64url value")
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(trimmed); err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(trimmed)
}

func VerifyWebAuthnRegistration(input WebAuthnRegistrationResponse, challenge string, rpID string, expectedOrigin string) (*WebAuthnParsedCredential, error) {
	if input.Type != "public-key" {
		return nil, fmt.Errorf("unexpected credential type: %s", input.Type)
	}
	clientDataRaw, err := DecodeBase64URL(input.Response.ClientDataJSON)
	if err != nil {
		return nil, fmt.Errorf("decode client data: %w", err)
	}
	if err := validateWebAuthnClientData(clientDataRaw, "webauthn.create", challenge, expectedOrigin); err != nil {
		return nil, err
	}
	attestationObject, err := DecodeBase64URL(input.Response.AttestationObject)
	if err != nil {
		return nil, fmt.Errorf("decode attestation object: %w", err)
	}
	parsed, err := parseCBORExact(attestationObject)
	if err != nil {
		return nil, fmt.Errorf("parse attestation object: %w", err)
	}
	attestationMap, ok := parsed.(map[any]any)
	if !ok {
		return nil, errors.New("attestation object is not a map")
	}
	authData, ok := attestationMap["authData"].([]byte)
	if !ok {
		return nil, errors.New("attestation authData is missing")
	}
	credential, err := parseAttestedCredentialData(authData, rpID)
	if err != nil {
		return nil, err
	}
	rawID := strings.TrimSpace(input.RawID)
	if rawID == "" {
		rawID = strings.TrimSpace(input.ID)
	}
	if rawID != "" && rawID != credential.CredentialID {
		return nil, errors.New("credential raw id does not match attested credential id")
	}
	return credential, nil
}

func VerifyWebAuthnAssertion(input WebAuthnLoginAssertion, challenge string, rpID string, expectedOrigin string, credential WebAuthnCredentialMaterial) (uint32, error) {
	if input.Type != "public-key" {
		return 0, fmt.Errorf("unexpected credential type: %s", input.Type)
	}
	rawID := strings.TrimSpace(input.RawID)
	if rawID == "" {
		rawID = strings.TrimSpace(input.ID)
	}
	if rawID != credential.CredentialID {
		return 0, errors.New("credential id does not match")
	}
	clientDataRaw, err := DecodeBase64URL(input.Response.ClientDataJSON)
	if err != nil {
		return 0, fmt.Errorf("decode client data: %w", err)
	}
	if err := validateWebAuthnClientData(clientDataRaw, "webauthn.get", challenge, expectedOrigin); err != nil {
		return 0, err
	}
	authData, err := DecodeBase64URL(input.Response.AuthenticatorData)
	if err != nil {
		return 0, fmt.Errorf("decode authenticator data: %w", err)
	}
	signature, err := DecodeBase64URL(input.Response.Signature)
	if err != nil {
		return 0, fmt.Errorf("decode signature: %w", err)
	}
	signCount, err := parseAssertionAuthenticatorData(authData, rpID, credential.SignCount)
	if err != nil {
		return 0, err
	}
	xBytes, err := DecodeBase64URL(credential.PublicKeyX)
	if err != nil {
		return 0, fmt.Errorf("decode public key x: %w", err)
	}
	yBytes, err := DecodeBase64URL(credential.PublicKeyY)
	if err != nil {
		return 0, fmt.Errorf("decode public key y: %w", err)
	}
	publicKey := ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xBytes), Y: new(big.Int).SetBytes(yBytes)}
	if !publicKey.Curve.IsOnCurve(publicKey.X, publicKey.Y) {
		return 0, errors.New("webauthn public key is not on P-256 curve")
	}
	clientDataHash := sha256.Sum256(clientDataRaw)
	verifyData := append(append([]byte{}, authData...), clientDataHash[:]...)
	digest := sha256.Sum256(verifyData)
	if !ecdsa.VerifyASN1(&publicKey, digest[:], signature) {
		return 0, errors.New("invalid webauthn signature")
	}
	return signCount, nil
}

func validateWebAuthnClientData(raw []byte, expectedType string, challenge string, expectedOrigin string) error {
	var clientData WebAuthnClientData
	if err := json.Unmarshal(raw, &clientData); err != nil {
		return fmt.Errorf("parse client data: %w", err)
	}
	if clientData.Type != expectedType {
		return fmt.Errorf("unexpected webauthn client data type: %s", clientData.Type)
	}
	if clientData.Challenge != challenge {
		return errors.New("webauthn challenge mismatch")
	}
	if expectedOrigin != "" && clientData.Origin != expectedOrigin {
		return fmt.Errorf("webauthn origin mismatch: %s", clientData.Origin)
	}
	return nil
}

func parseAttestedCredentialData(authData []byte, rpID string) (*WebAuthnParsedCredential, error) {
	signCount, credentialData, err := parseAuthenticatorDataHeader(authData, rpID, true, 0)
	if err != nil {
		return nil, err
	}
	if len(credentialData) < 18 {
		return nil, errors.New("attested credential data is too short")
	}
	offset := 16
	credentialIDLength := int(binary.BigEndian.Uint16(credentialData[offset : offset+2]))
	offset += 2
	if credentialIDLength <= 0 || len(credentialData) < offset+credentialIDLength {
		return nil, errors.New("invalid credential id length")
	}
	credentialID := credentialData[offset : offset+credentialIDLength]
	offset += credentialIDLength
	publicKeyRaw := credentialData[offset:]
	publicKey, err := parseCBOR(publicKeyRaw)
	if err != nil {
		return nil, fmt.Errorf("parse credential public key: %w", err)
	}
	publicKeyMap, ok := publicKey.(map[any]any)
	if !ok {
		return nil, errors.New("credential public key is not a map")
	}
	kty, err := coseInt(publicKeyMap, 1)
	if err != nil {
		return nil, err
	}
	alg, err := coseInt(publicKeyMap, 3)
	if err != nil {
		return nil, err
	}
	crv, err := coseInt(publicKeyMap, -1)
	if err != nil {
		return nil, err
	}
	if kty != 2 || alg != -7 || crv != 1 {
		return nil, fmt.Errorf("unsupported COSE key: kty=%d alg=%d crv=%d", kty, alg, crv)
	}
	x, err := coseBytes(publicKeyMap, -2)
	if err != nil {
		return nil, err
	}
	y, err := coseBytes(publicKeyMap, -3)
	if err != nil {
		return nil, err
	}
	if !elliptic.P256().IsOnCurve(new(big.Int).SetBytes(x), new(big.Int).SetBytes(y)) {
		return nil, errors.New("credential public key is not on P-256 curve")
	}
	return &WebAuthnParsedCredential{
		CredentialID: EncodeBase64URL(credentialID),
		PublicKeyX:   EncodeBase64URL(x),
		PublicKeyY:   EncodeBase64URL(y),
		SignCount:    signCount,
	}, nil
}

func parseAssertionAuthenticatorData(authData []byte, rpID string, previousSignCount uint32) (uint32, error) {
	signCount, _, err := parseAuthenticatorDataHeader(authData, rpID, false, previousSignCount)
	if err != nil {
		return 0, err
	}
	return signCount, nil
}

func parseAuthenticatorDataHeader(authData []byte, rpID string, requireAttestedData bool, previousSignCount uint32) (uint32, []byte, error) {
	if len(authData) < 37 {
		return 0, nil, errors.New("authenticator data is too short")
	}
	expectedRPIDHash := sha256.Sum256([]byte(rpID))
	if string(authData[:32]) != string(expectedRPIDHash[:]) {
		return 0, nil, errors.New("rp id hash mismatch")
	}
	flags := authData[32]
	if flags&0x01 == 0 {
		return 0, nil, errors.New("user presence flag is missing")
	}
	signCount := binary.BigEndian.Uint32(authData[33:37])
	if previousSignCount > 0 && signCount > 0 && signCount <= previousSignCount {
		return 0, nil, errors.New("authenticator sign count did not increase")
	}
	if requireAttestedData && flags&0x40 == 0 {
		return 0, nil, errors.New("attested credential data flag is missing")
	}
	return signCount, authData[37:], nil
}

func coseInt(m map[any]any, key int64) (int64, error) {
	value, ok := m[key]
	if !ok {
		return 0, fmt.Errorf("missing COSE key %d", key)
	}
	intValue, ok := value.(int64)
	if !ok {
		return 0, fmt.Errorf("invalid COSE key %d", key)
	}
	return intValue, nil
}

func coseBytes(m map[any]any, key int64) ([]byte, error) {
	value, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("missing COSE key %d", key)
	}
	bytesValue, ok := value.([]byte)
	if !ok || len(bytesValue) == 0 {
		return nil, fmt.Errorf("invalid COSE key %d", key)
	}
	return bytesValue, nil
}

func parseCBOR(data []byte) (any, error) {
	reader := cborReader{data: data}
	value, err := reader.read()
	if err != nil {
		return nil, err
	}
	return value, nil
}

func parseCBORExact(data []byte) (any, error) {
	reader := cborReader{data: data}
	value, err := reader.read()
	if err != nil {
		return nil, err
	}
	if reader.pos != len(data) {
		return nil, errors.New("trailing cbor data")
	}
	return value, nil
}

type cborReader struct {
	data []byte
	pos  int
}

func (r *cborReader) read() (any, error) {
	if r.pos >= len(r.data) {
		return nil, errors.New("unexpected cbor eof")
	}
	initial := r.data[r.pos]
	r.pos++
	major := initial >> 5
	additional := initial & 0x1f
	length, err := r.readLength(additional)
	if err != nil {
		return nil, err
	}
	switch major {
	case 0:
		return int64(length), nil
	case 1:
		return -1 - int64(length), nil
	case 2:
		return r.readBytes(length)
	case 3:
		raw, err := r.readBytes(length)
		if err != nil {
			return nil, err
		}
		return string(raw), nil
	case 4:
		out := make([]any, 0, length)
		for i := uint64(0); i < length; i++ {
			item, err := r.read()
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
		return out, nil
	case 5:
		out := make(map[any]any, length)
		for i := uint64(0); i < length; i++ {
			key, err := r.read()
			if err != nil {
				return nil, err
			}
			value, err := r.read()
			if err != nil {
				return nil, err
			}
			out[key] = value
		}
		return out, nil
	case 7:
		switch additional {
		case 20:
			return false, nil
		case 21:
			return true, nil
		case 22, 23:
			return nil, nil
		default:
			return nil, fmt.Errorf("unsupported cbor simple value: %d", additional)
		}
	default:
		return nil, fmt.Errorf("unsupported cbor major type: %d", major)
	}
}

func (r *cborReader) readLength(additional byte) (uint64, error) {
	switch {
	case additional < 24:
		return uint64(additional), nil
	case additional == 24:
		if r.pos+1 > len(r.data) {
			return 0, errors.New("unexpected cbor eof")
		}
		value := r.data[r.pos]
		r.pos++
		return uint64(value), nil
	case additional == 25:
		if r.pos+2 > len(r.data) {
			return 0, errors.New("unexpected cbor eof")
		}
		value := binary.BigEndian.Uint16(r.data[r.pos : r.pos+2])
		r.pos += 2
		return uint64(value), nil
	case additional == 26:
		if r.pos+4 > len(r.data) {
			return 0, errors.New("unexpected cbor eof")
		}
		value := binary.BigEndian.Uint32(r.data[r.pos : r.pos+4])
		r.pos += 4
		return uint64(value), nil
	case additional == 27:
		if r.pos+8 > len(r.data) {
			return 0, errors.New("unexpected cbor eof")
		}
		value := binary.BigEndian.Uint64(r.data[r.pos : r.pos+8])
		r.pos += 8
		return value, nil
	default:
		return 0, fmt.Errorf("unsupported cbor additional info: %d", additional)
	}
}

func (r *cborReader) readBytes(length uint64) ([]byte, error) {
	if length > uint64(len(r.data)-r.pos) {
		return nil, errors.New("unexpected cbor eof")
	}
	out := r.data[r.pos : r.pos+int(length)]
	r.pos += int(length)
	return out, nil
}
