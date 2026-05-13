package cryptor

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestECCEncryptDecrypt(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	privateCryptor, err := NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)

	publicKey, err := privateCryptor.PublicKey()
	require.NoError(t, err)

	publicCryptor, err := NewFromPublicKey(publicKey)
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

	publicCryptor, err := NewFromPublicKey(hexutil.Encode(crypto.FromECDSAPub(&privateKey.PublicKey)))
	require.NoError(t, err)

	plaintext := "hex public key payload"
	ciphertext, err := publicCryptor.Encrypt(plaintext)
	require.NoError(t, err)

	decrypted, err := privateCryptor.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}
