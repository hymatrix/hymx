package tagcrypto

import (
	"strings"
	"testing"

	"github.com/everFinance/goether"
	goar "github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestEthereumEncryptedTagRoundTrip(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	tags := []goarSchema.Tag{{Name: EncryptedTagPrefix + "Secret", Value: "private-value"}}
	encrypted, changed, err := EncryptTags(tags, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)
	require.True(t, changed)
	require.Len(t, encrypted, 1)
	require.Equal(t, EncryptedTagPrefix+"Secret", encrypted[0].Name)
	require.NotContains(t, encrypted[0].Value, "private-value")
	require.True(t, strings.HasPrefix(encrypted[0].Value, CipherValuePrefix+":"+KeyTypeEthereumECIES+":"))

	decrypted, changed, err := DecryptTags(encrypted, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []goarSchema.Tag{{Name: "Secret", Value: "private-value"}}, decrypted)
}

func TestArweaveEncryptedTagRoundTrip(t *testing.T) {
	nodeSigner, err := goar.NewSignerFromPath("../../examples/test_keyfile.json")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	tags := []goarSchema.Tag{{Name: EncryptedTagPrefix + "Secret", Value: "private-value"}}
	encrypted, changed, err := EncryptTags(tags, nodeBundler.Owner, KeyTypeArweaveRSAOAEP)
	require.NoError(t, err)
	require.True(t, changed)
	require.Len(t, encrypted, 1)
	require.Equal(t, EncryptedTagPrefix+"Secret", encrypted[0].Name)
	require.NotContains(t, encrypted[0].Value, "private-value")
	require.True(t, strings.HasPrefix(encrypted[0].Value, CipherValuePrefix+":"+KeyTypeArweaveRSAOAEP+":"))

	decrypted, changed, err := DecryptTags(encrypted, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []goarSchema.Tag{{Name: "Secret", Value: "private-value"}}, decrypted)
}

func TestEncryptedReservedTagRejected(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	_, _, err = EncryptTags(
		[]goarSchema.Tag{{Name: EncryptedTagPrefix + "Type", Value: "Message"}},
		nodeBundler.Owner,
		KeyTypeEthereumECIES,
	)
	require.Error(t, err)
}
