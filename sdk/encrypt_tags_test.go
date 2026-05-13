package sdk

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hymatrix/hymx/cryptor"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptTags(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	privateCryptor, err := cryptor.NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)
	publicKey, err := privateCryptor.PublicKey()
	require.NoError(t, err)

	publicCryptor, err := cryptor.NewFromPublicKey(publicKey)
	require.NoError(t, err)

	s := &SDK{Client: NewClient("")}
	s.Client.SetCryptor(publicCryptor)
	tags, err := s.EncryptTags([]goarSchema.Tag{{Name: "Secret", Value: "top-secret"}})
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, vmmSchema.EncryptedTagPrefix+"Secret", tags[0].Name)

	decryptedValue, err := privateCryptor.Decrypt(tags[0].Value)
	require.NoError(t, err)
	assert.Equal(t, "top-secret", decryptedValue)
}

func TestSetCryptorFromInfo(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	privateCryptor, err := cryptor.NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)
	publicKey, err := privateCryptor.PublicKey()
	require.NoError(t, err)

	c := NewClient("")
	require.NoError(t, c.SetCryptorFromInfo(publicKey))

	publicCryptor, err := c.GetCryptor()
	require.NoError(t, err)

	encryptedValue, err := publicCryptor.Encrypt("payload")
	require.NoError(t, err)

	decryptedValue, err := privateCryptor.Decrypt(encryptedValue)
	require.NoError(t, err)
	assert.Equal(t, "payload", decryptedValue)
}
