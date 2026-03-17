package backupcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	chunkSize      = 1 << 20
	fileMagic      = "BXENC1"
	nonceSizeBytes = 12
)

func EncryptFile(key []byte, sourcePath string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open source file: %w", err)
	}
	defer source.Close()
	targetPath := sourcePath + ".enc"
	target, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("create encrypted file: %w", err)
	}
	defer target.Close()
	if _, err := target.WriteString(fileMagic); err != nil {
		return "", fmt.Errorf("write encryption header: %w", err)
	}
	buffer := make([]byte, chunkSize)
	for {
		readCount, readErr := source.Read(buffer)
		if readCount > 0 {
			nonce := make([]byte, gcm.NonceSize())
			if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
				return "", fmt.Errorf("generate nonce: %w", err)
			}
			sealed := gcm.Seal(nil, nonce, buffer[:readCount], nil)
			if _, err := target.Write(nonce); err != nil {
				return "", fmt.Errorf("write nonce: %w", err)
			}
			if err := binary.Write(target, binary.BigEndian, uint32(len(sealed))); err != nil {
				return "", fmt.Errorf("write ciphertext length: %w", err)
			}
			if _, err := target.Write(sealed); err != nil {
				return "", fmt.Errorf("write ciphertext: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("read source file: %w", readErr)
		}
	}
	return targetPath, nil
}

func DecryptFile(key []byte, sourcePath string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", fmt.Errorf("open encrypted file: %w", err)
	}
	defer source.Close()
	header := make([]byte, len(fileMagic))
	if _, err := io.ReadFull(source, header); err != nil {
		return "", fmt.Errorf("read encryption header: %w", err)
	}
	if string(header) != fileMagic {
		return "", fmt.Errorf("invalid encrypted file header")
	}
	targetPath := strings.TrimSuffix(sourcePath, ".enc")
	if targetPath == sourcePath {
		targetPath += ".plain"
	}
	target, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("create decrypted file: %w", err)
	}
	defer target.Close()
	for {
		nonce := make([]byte, nonceSizeBytes)
		_, err := io.ReadFull(source, nonce)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read nonce: %w", err)
		}
		var cipherLength uint32
		if err := binary.Read(source, binary.BigEndian, &cipherLength); err != nil {
			return "", fmt.Errorf("read ciphertext length: %w", err)
		}
		ciphertext := make([]byte, cipherLength)
		if _, err := io.ReadFull(source, ciphertext); err != nil {
			return "", fmt.Errorf("read ciphertext payload: %w", err)
		}
		plain, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return "", fmt.Errorf("decrypt chunk: %w", err)
		}
		if _, err := target.Write(plain); err != nil {
			return "", fmt.Errorf("write decrypted payload: %w", err)
		}
	}
	return targetPath, nil
}
