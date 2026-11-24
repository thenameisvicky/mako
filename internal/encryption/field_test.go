package encryption

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// testKey is a throwaway AES-256 key generated for this test file only.
const testKey = "I2QRtnVxxxy57fKK8UiFFxvh5XCuIy1bekqplo3NX3M="

func TestEncryptFieldRoundTrip(t *testing.T) {
	e, err := New(testKey)
	require.NoError(t, err)

	ciphertext, err := e.EncryptField("hunter2")
	require.NoError(t, err)
	require.True(t, IsEncryptedField(ciphertext))
	require.NotContains(t, ciphertext, "hunter2")

	plaintext, err := e.DecryptField(ciphertext)
	require.NoError(t, err)
	require.Equal(t, "hunter2", plaintext)
}

func TestEncryptFieldUsesFreshNonce(t *testing.T) {
	e, err := New(testKey)
	require.NoError(t, err)

	first, err := e.EncryptField("same-value")
	require.NoError(t, err)
	second, err := e.EncryptField("same-value")
	require.NoError(t, err)

	require.NotEqual(t, first, second, "a repeated value must not produce identical ciphertext")
}

func TestBlindIndexIsDeterministicAndNormalised(t *testing.T) {
	e, err := New(testKey)
	require.NoError(t, err)

	require.Equal(t, e.BlindIndex("vicky"), e.BlindIndex("vicky"))
	require.Equal(t, e.BlindIndex("vicky"), e.BlindIndex("  VICKY "))
	require.NotEqual(t, e.BlindIndex("vicky"), e.BlindIndex("vicky2"))
	require.NotContains(t, e.BlindIndex("vicky"), "vicky")
}

func TestVerifyPassword(t *testing.T) {
	e, err := New(testKey)
	require.NoError(t, err)

	stored, err := e.EncryptField("correct-horse")
	require.NoError(t, err)

	require.True(t, e.VerifyPassword(stored, "correct-horse"))
	require.False(t, e.VerifyPassword(stored, "wrong"))
	require.False(t, e.VerifyPassword("not-encrypted", "correct-horse"))
}

func TestDecryptFieldRejectsTamperedValue(t *testing.T) {
	e, err := New(testKey)
	require.NoError(t, err)

	ciphertext, err := e.EncryptField("secret")
	require.NoError(t, err)

	// Flip a character in the base64 payload; GCM's tag must reject it.
	tampered := []byte(ciphertext)
	tampered[len(tampered)-2] ^= 'A' ^ 'B'

	_, err = e.DecryptField(string(tampered))
	require.Error(t, err)
}

func TestDecryptFieldRejectsNonField(t *testing.T) {
	e, err := New(testKey)
	require.NoError(t, err)

	_, err = e.DecryptField("$2a$10$legacybcrypthash")
	require.Error(t, err)
}
