package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
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

// --- Field-level helpers -----------------------------------------------------
//
// EncryptField/DecryptField store the nonce alongside the ciphertext in a single
// self-contained base64 string, so a column can be encrypted without adding a
// companion nonce column. Layout: base64(nonce || ciphertext).
//
// Use these for at-rest field encryption (username, email, password, chat blobs).
// Use Encrypt/Decrypt when the caller wants ciphertext and nonce kept apart.

// fieldPrefix marks a value as produced by EncryptField, so legacy plaintext or
// bcrypt values can be detected and migrated transparently on read.
const fieldPrefix = "enc:v1:"

// EncryptField encrypts a value into a single self-contained, prefixed string
func (e *Encryptor) EncryptField(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Seal appends the ciphertext to the nonce, giving us nonce||ciphertext
	sealed := aesgcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return fieldPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// IsEncryptedField reports whether a stored value was written by EncryptField
func IsEncryptedField(value string) bool {
	return strings.HasPrefix(value, fieldPrefix)
}

// DecryptField reverses EncryptField
func (e *Encryptor) DecryptField(value string) (string, error) {
	if !IsEncryptedField(value) {
		return "", errors.New("value is not an encrypted field")
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, fieldPrefix))
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted field: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(raw) < aesgcm.NonceSize() {
		return "", errors.New("encrypted field is too short")
	}

	nonce, ciphertext := raw[:aesgcm.NonceSize()], raw[aesgcm.NonceSize():]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	return string(plaintext), nil
}

// BlindIndex derives a deterministic, non-reversible lookup token for a value.
//
// AES-GCM uses a fresh nonce per encryption, so an encrypted column cannot be
// queried by equality. The blind index gives us a stable value to index and
// match on (e.g. finding a user by username) without storing it in the clear.
func (e *Encryptor) BlindIndex(value string) string {
	mac := hmac.New(sha256.New, e.key)
	mac.Write([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyPassword compares a candidate password against a stored AES-GCM value in
// constant time. Returns false rather than an error on any mismatch.
func (e *Encryptor) VerifyPassword(stored, candidate string) bool {
	plaintext, err := e.DecryptField(stored)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(plaintext), []byte(candidate)) == 1
}
