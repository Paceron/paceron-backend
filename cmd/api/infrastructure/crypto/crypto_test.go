package crypto

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESGCMEncryptor_RoundTrip(t *testing.T) {
	enc := NewAESGCMEncryptor("0123456789abcdef")
	require.NotNil(t, enc)

	plaintext := "token-secreto-de-mercado-pago"
	ciphertext, err := enc.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, ciphertext)
	assert.NotEmpty(t, ciphertext)

	decrypted, err := enc.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestAESGCMEncryptor_RoundTrip_DifferentNonces(t *testing.T) {
	enc := NewAESGCMEncryptor("1234567890abcdef")

	c1, err := enc.Encrypt("mismo-plaintext")
	require.NoError(t, err)
	c2, err := enc.Encrypt("mismo-plaintext")
	require.NoError(t, err)

	assert.NotEqual(t, c1, c2)

	d1, err := enc.Decrypt(c1)
	require.NoError(t, err)
	d2, err := enc.Decrypt(c2)
	require.NoError(t, err)
	assert.Equal(t, "mismo-plaintext", d1)
	assert.Equal(t, "mismo-plaintext", d2)
}

func TestAESGCMEncryptor_KeySizes(t *testing.T) {
	keys := []string{"1234567890123456", "123456789012345678901234", "12345678901234567890123456789012"}
	for _, k := range keys {
		enc := NewAESGCMEncryptor(k)
		ciphertext, err := enc.Encrypt("secret")
		require.NoError(t, err, "key len %d", len(k))
		plain, err := enc.Decrypt(ciphertext)
		require.NoError(t, err)
		assert.Equal(t, "secret", plain)
	}
}

func TestAESGCMEncryptor_InvalidKeySize(t *testing.T) {
	enc := NewAESGCMEncryptor("short")
	_, err := enc.Encrypt("secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key size")

	_, err = enc.Decrypt("dGVzdA==")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key size")
}

func TestAESGCMEncryptor_DecryptInvalidBase64(t *testing.T) {
	enc := NewAESGCMEncryptor("0123456789abcdef")
	_, err := enc.Decrypt("!!!!not-base64!!!!")
	require.Error(t, err)
}

func TestAESGCMEncryptor_DecryptTooShort(t *testing.T) {
	enc := NewAESGCMEncryptor("0123456789abcdef")
	_, err := enc.Decrypt("aGk=")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ciphertext too short")
}

func TestAESGCMEncryptor_DecryptTampered(t *testing.T) {
	enc := NewAESGCMEncryptor("0123456789abcdef")
	ciphertext, err := enc.Encrypt("secret-value")
	require.NoError(t, err)

	_, err = enc.Decrypt(ciphertext + "cambiada")
	require.Error(t, err)
}

func TestAESGCMEncryptor_DecryptWrongKey(t *testing.T) {
	encA := NewAESGCMEncryptor("0123456789abcdef")
	encB := NewAESGCMEncryptor("fedcba9876543210")

	ciphertext, err := encA.Encrypt("between-tokens")
	require.NoError(t, err)

	_, err = encB.Decrypt(ciphertext)
	require.Error(t, err)
}

func TestLegacyEncrypt_Decrypt_RoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef")

	ciphertext, err := Encrypt(key, "legacy-secret")
	require.NoError(t, err)
	assert.NotEqual(t, "legacy-secret", ciphertext)

	plain, err := Decrypt(key, ciphertext)
	require.NoError(t, err)
	assert.Equal(t, "legacy-secret", plain)
}

func TestLegacyEncrypt_InvalidKeySize(t *testing.T) {
	_, err := Encrypt([]byte("bad"), "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key size")

	_, err = Decrypt([]byte("bad"), "aGk=")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid key size")
}

func TestLegacyEncrypt_InvalidBase64(t *testing.T) {
	key := []byte("0123456789abcdef")
	_, err := Decrypt(key, "no-base64!!!")
	require.Error(t, err)
}

func TestLegacyDecrypt_CiphertextTooShort(t *testing.T) {
	key := []byte("0123456789abcdef")
	_, err := Decrypt(key, "YWJj")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "ciphertext too short"))
}
