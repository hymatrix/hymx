package vmm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hymatrix/hymx/cryptor"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecryptParams(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	c, err := cryptor.NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)
	encryptedValue, err := c.Encrypt("secret")
	require.NoError(t, err)

	v := &Vmm{cryptor: c}
	meta := schema.Meta{
		Params: map[string]string{
			"Foo":           "plain",
			"Encrypted-Bar": encryptedValue,
		},
	}

	v.decryptParams(&meta)
	assert.Equal(t, "plain", meta.Params["Foo"])
	assert.Equal(t, "secret", meta.Params["Bar"])
	assert.Equal(t, encryptedValue, meta.Params["Encrypted-Bar"])
}

func TestDecryptParamsNilMeta(t *testing.T) {
	v := &Vmm{}

	assert.NotPanics(t, func() {
		v.decryptParams(nil)
	})
}

func TestDecryptParamsSkipsExistingPlaintextParam(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	c, err := cryptor.NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)
	encryptedValue, err := c.Encrypt("secret")
	require.NoError(t, err)

	v := &Vmm{cryptor: c}
	meta := schema.Meta{
		Params: map[string]string{
			"Bar":           "plain-secret",
			"Encrypted-Bar": encryptedValue,
		},
	}

	v.decryptParams(&meta)
	assert.Equal(t, "plain-secret", meta.Params["Bar"])
	assert.Equal(t, encryptedValue, meta.Params["Encrypted-Bar"])
}

func TestDecryptParamsUnsupportedNodeKeepsParams(t *testing.T) {
	v := &Vmm{}
	meta := schema.Meta{
		Params: map[string]string{
			"Encrypted-Bar": "secret",
		},
	}

	v.decryptParams(&meta)
	assert.NotContains(t, meta.Params, "Bar")
	assert.Equal(t, "secret", meta.Params["Encrypted-Bar"])
}
