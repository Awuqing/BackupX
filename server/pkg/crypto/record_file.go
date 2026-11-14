package backupcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const recordFileMagic = "BXENC2"

// RecordKey isolates each backup without distributing the Master encryption key.
func RecordKey(masterKey []byte, recordID uint) []byte {
	mac := hmac.New(sha256.New, masterKey)
	fmt.Fprintf(mac, "backupx:record:v2:%d", recordID)
	return mac.Sum(nil)
}

func EncryptRecordFile(key []byte, recordID uint, sourcePath string) (string, error) {
	return transformRecordFile(key, recordID, sourcePath, true)
}

func DecryptRecordFile(key []byte, recordID uint, sourcePath string) (string, error) {
	return transformRecordFile(key, recordID, sourcePath, false)
}

// V2 authenticates ordering, record identity and an explicit end marker, so
// truncation and chunk reordering fail even without an external checksum.
func transformRecordFile(key []byte, recordID uint, sourcePath string, encrypt bool) (result string, err error) {
	if recordID == 0 || len(key) != 32 {
		return "", errors.New("invalid record encryption key or record ID")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	header := make([]byte, len(recordFileMagic)+8)
	copy(header, recordFileMagic)
	binary.BigEndian.PutUint64(header[len(recordFileMagic):], uint64(recordID))
	if !encrypt {
		actual := make([]byte, len(header))
		if _, err := io.ReadFull(source, actual); err != nil {
			return "", err
		}
		if !hmac.Equal(actual, header) {
			return "", errors.New("encrypted backup record mismatch or unsupported format")
		}
	}
	targetPath := sourcePath + ".enc"
	if !encrypt {
		targetPath = strings.TrimSuffix(sourcePath, ".enc")
		if targetPath == sourcePath {
			targetPath += ".plain"
		}
	}
	target, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, target.Close())
		if err != nil {
			err = errors.Join(err, os.Remove(targetPath))
			result = ""
		}
	}()
	if encrypt {
		if _, err := target.Write(header); err != nil {
			return "", err
		}
	}
	buffer := make([]byte, chunkSize)
	aad := append(header, make([]byte, 8)...)
	for sequence := uint64(0); ; sequence++ {
		binary.BigEndian.PutUint64(aad[len(header):], sequence)
		nonce := make([]byte, gcm.NonceSize())
		if encrypt {
			n, readErr := io.ReadFull(source, buffer)
			if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
				return "", readErr
			}
			if _, err := rand.Read(nonce); err != nil {
				return "", err
			}
			sealed := gcm.Seal(nil, nonce, buffer[:n], aad)
			if _, err := target.Write(nonce); err != nil {
				return "", err
			}
			if err := binary.Write(target, binary.BigEndian, uint32(len(sealed))); err != nil {
				return "", err
			}
			if _, err := target.Write(sealed); err != nil {
				return "", err
			}
			if n == 0 {
				return targetPath, nil
			}
		} else {
			if _, err := io.ReadFull(source, nonce); err != nil {
				return "", fmt.Errorf("missing encrypted frame: %w", err)
			}
			var length uint32
			if err := binary.Read(source, binary.BigEndian, &length); err != nil {
				return "", err
			}
			if length < uint32(gcm.Overhead()) || length > uint32(chunkSize+gcm.Overhead()) {
				return "", errors.New("invalid encrypted frame length")
			}
			sealed := make([]byte, length)
			if _, err := io.ReadFull(source, sealed); err != nil {
				return "", err
			}
			plain, err := gcm.Open(nil, nonce, sealed, aad)
			if err != nil {
				return "", fmt.Errorf("decrypt backup frame: %w", err)
			}
			if len(plain) == 0 {
				if n, err := source.Read(buffer[:1]); n != 0 || err != io.EOF {
					return "", errors.New("data after encrypted end marker")
				}
				return targetPath, nil
			}
			if _, err := target.Write(plain); err != nil {
				return "", err
			}
		}
	}
}
