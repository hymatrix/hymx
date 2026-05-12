package vmm

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/hymatrix/hymx/cryptor"
	"github.com/hymatrix/hymx/vmm/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecryptParams(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	c := cryptor.NewRSAFromPrivateKey(privateKey)
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

func TestDecryptParamsSkipsExistingPlaintextParam(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	c := cryptor.NewRSAFromPrivateKey(privateKey)
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

func TestDecryptParamsMissingDecryptor(t *testing.T) {
	v := &Vmm{}
	meta := schema.Meta{
		Params: map[string]string{
			"Encrypted-Bar": "secret",
		},
	}

	v.decryptParams(&meta)
	assert.Equal(t, schema.ErrDecryptParamFailed.Error(), meta.Params["Bar"])
}
