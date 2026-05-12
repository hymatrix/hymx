package cryptor

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hymatrix/hymx/cryptor/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRSAKeyfileEncryptDecrypt(t *testing.T) {
	c, err := NewRSAFromKeyfile("../cmd/test_keyfile.json")
	require.NoError(t, err)

	plaintext := "hymx rsa payload"
	ciphertext, err := c.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := c.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestRSAEncryptWithPublicKeyDecryptWithPrivateKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateCryptor := NewRSAFromPrivateKey(privateKey)
	publicKey, algorithm, err := privateCryptor.PublicKey()
	require.NoError(t, err)

	publicCryptor, err := NewFromPublicKey(algorithm, publicKey)
	require.NoError(t, err)

	plaintext := "public rsa payload"
	ciphertext, err := publicCryptor.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := privateCryptor.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestECCEncryptDecrypt(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	privateCryptor, err := NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)

	publicKey, algorithm, err := privateCryptor.PublicKey()
	require.NoError(t, err)

	publicCryptor, err := NewFromPublicKey(algorithm, publicKey)
	require.NoError(t, err)

	plaintext := "hymx ecc payload"
	ciphertext, err := publicCryptor.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := privateCryptor.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestECCEncryptWithHexPublicKey(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	privateCryptor, err := NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)

	publicCryptor, err := NewFromPublicKey(schema.AlgorithmECC, hexutil.Encode(crypto.FromECDSAPub(&privateKey.PublicKey)))
	require.NoError(t, err)

	plaintext := "hex public key payload"
	ciphertext, err := publicCryptor.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := privateCryptor.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestRSAKeyfileAlgorithm(t *testing.T) {
	rsaCryptor, err := NewRSAFromKeyfile("../cmd/test_keyfile.json")
	require.NoError(t, err)
	assert.Equal(t, schema.AlgorithmRSA, rsaCryptor.Algorithm)
}
