package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thenameisvicky/tokenly-gateway/internal/encryption"
)

func TestEncryptDecrypt(t *testing.T) {
	// Base64 encoded 32-byte key
	key := "lQZlZWbckfgey+Zmcoa4lQ1epLALwHomyLNGCIJPPIo="

	encryptor, err := encryption.New(key)
	require.NoError(t, err)

	secretMessage := "sk-ant-123456789"

	encVal, err := encryptor.Encrypt(secretMessage)
	require.NoError(t, err)
	assert.NotEmpty(t, encVal.Ciphertext)
	assert.NotEmpty(t, encVal.Nonce)

	decrypted, err := encryptor.Decrypt(encVal)
	require.NoError(t, err)
	assert.Equal(t, secretMessage, decrypted)

	// Test incorrect key validation
	_, err = encryption.New("too-short-key")
	assert.Error(t, err)
}
