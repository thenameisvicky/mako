package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Ciphertext represents base64 encoded encrypted string
type Ciphertext string

// Nonce represents base64 encoded GCM nonce
type Nonce string

type EncryptedValue struct {
	Ciphertext Ciphertext
	Nonce      Nonce
}

type Encryptor struct {
	key []byte
}

// New creates a new Encryptor instance. It accepts a 32-byte key (either raw or base64 encoded).
func New(keyStr string) (*Encryptor, error) {
	// Try base64 decoding first
	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		// Fallback to raw bytes
		key = []byte(keyStr)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be exactly 32 bytes, got %d bytes", len(key))
	}

	return &Encryptor{key: key}, nil
}

// Encrypt encrypts a plaintext string and returns EncryptedValue containing base64 encoded ciphertext and nonce
func (e *Encryptor) Encrypt(plaintext string) (*EncryptedValue, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)

	return &EncryptedValue{
		Ciphertext: Ciphertext(base64.StdEncoding.EncodeToString(ciphertext)),
		Nonce:      Nonce(base64.StdEncoding.EncodeToString(nonce)),
	}, nil
}

// Decrypt decrypts ciphertext using the nonce and returns the original plaintext
func (e *Encryptor) Decrypt(encValue *EncryptedValue) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(string(encValue.Ciphertext))
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	nonce, err := base64.StdEncoding.DecodeString(string(encValue.Nonce))
	if err != nil {
		return "", fmt.Errorf("failed to decode nonce: %w", err)
	}

	if len(nonce) != aesgcm.NonceSize() {
		return "", errors.New("invalid nonce size")
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}
