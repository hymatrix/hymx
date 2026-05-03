package vmm

import (
	"encoding/json"
	"testing"

	"github.com/everFinance/goether"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	"github.com/hymatrix/hymx/vmm/schema"
	goar "github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestDeriveMetaDecryptedParamsPreservesRawParams(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	tags, changed, err := tagcrypto.EncryptTags([]goarSchema.Tag{
		{Name: "Public", Value: "public-value"},
		{Name: "Encrypted-Secret", Value: "private-value"},
	}, nodeBundler.Owner, tagcrypto.KeyTypeEthereumECIES)
	require.NoError(t, err)
	require.True(t, changed)

	params := map[string]string{}
	for _, tag := range tags {
		params[tag.Name] = tag.Value
	}
	rawParams := map[string]string{}
	for key, value := range params {
		rawParams[key] = value
	}

	v := &Vmm{tagDecryptKey: nodeSigner}
	meta, err := v.withDecryptedParamsFromTags(schema.Meta{Params: params}, tags)
	require.NoError(t, err)

	require.Equal(t, rawParams, meta.Params)
	require.Equal(t, "private-value", meta.DecryptedParams["Encrypted-Secret"])
}

func TestDecryptedParamsAreNotSerializedInCheckpointEnv(t *testing.T) {
	env := schema.Env{
		Meta: schema.Meta{
			Params: map[string]string{
				"Encrypted-Secret": "ciphertext-value",
			},
			DecryptedParams: map[string]string{
				"Encrypted-Secret": "private-value",
			},
		},
		Process: hymxSchema.Process{},
	}

	data, err := json.Marshal(env)
	require.NoError(t, err)

	require.NotContains(t, string(data), "private-value")
	require.Contains(t, string(data), "Encrypted-Secret")
}
