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

	decryptedParams, changed, err := DecryptParams(encrypted, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, map[string]string{"Encrypted-Secret": "private-value"}, decryptedParams)
}

func TestDecryptParamsKeepsEncryptedTagKeys(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	tags := []goarSchema.Tag{{Name: EncryptedTagPrefix + "Secret", Value: "private-value"}}
	encrypted, changed, err := EncryptTags(tags, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "Encrypted-Secret", encrypted[0].Name)

	params, changed, err := DecryptParams(encrypted, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, map[string]string{"Encrypted-Secret": "private-value"}, params)
}

func TestDecryptParamMapKeepsEncryptedTagKeys(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	encrypted, _, err := EncryptTags([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)

	decrypted, changed, err := DecryptParamMap(map[string]string{
		"Public":          "public-value",
		encrypted[0].Name: encrypted[0].Value,
	}, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, map[string]string{"Encrypted-Secret": "private-value"}, decrypted)
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

	decryptedParams, changed, err := DecryptParams(encrypted, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, map[string]string{"Encrypted-Secret": "private-value"}, decryptedParams)
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

func TestInvalidEncryptedTagNameRejected(t *testing.T) {
	tags := []goarSchema.Tag{{Name: EncryptedTagPrefix, Value: "private-value"}}

	err := ValidateEncryptedTagNames(tags)

	require.Error(t, err)
}

func TestEncryptTagsLeavesPlainTagsUnchanged(t *testing.T) {
	tags := []goarSchema.Tag{{Name: "Plain", Value: "public-value"}}

	encrypted, changed, err := EncryptTags(tags, "", KeyTypeEthereumECIES)

	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, tags, encrypted)
}

func TestMixedPlainAndEncryptedTagsRoundTrip(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	tags := []goarSchema.Tag{
		{Name: "Plain", Value: "public-value"},
		{Name: EncryptedTagPrefix + "Secret", Value: "private-value"},
	}
	encrypted, changed, err := EncryptTags(tags, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "public-value", encrypted[0].Value)
	require.NotContains(t, encrypted[1].Value, "private-value")

	names, err := EncryptedPlainTagNames(encrypted)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"Secret": true}, names)

	decryptedParams, changed, err := DecryptParams(encrypted, nodeSigner)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, map[string]string{"Encrypted-Secret": "private-value"}, decryptedParams)
}

func TestDecryptParamsRejectsMalformedCipherValue(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)

	_, _, err = DecryptParams([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: "not-a-cipher"},
	}, nodeSigner)

	require.Error(t, err)
}

func TestEncryptTagsRejectsUnsupportedKeyType(t *testing.T) {
	_, _, err := EncryptTags([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, "public-key", "unsupported-key-type")

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported encryption key type")
}

func TestDecryptParamsRejectsUnsupportedCipherKeyType(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)

	_, _, err = DecryptParams([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: CipherValuePrefix + ":unsupported-key-type:Y2lwaGVy"},
	}, nodeSigner)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported encryption key type")
}

func TestDecryptParamsRejectsWrongSignerType(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	arweaveSigner, err := goar.NewSignerFromPath("../../examples/test_keyfile.json")
	require.NoError(t, err)

	encrypted, _, err := EncryptTags([]goarSchema.Tag{
		{Name: EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.NoError(t, err)

	_, _, err = DecryptParams(encrypted, arweaveSigner)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ethereum encrypted tag requires ethereum signer")
}

func TestEncryptedReservedNameRejectedConsistently(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	tags := []goarSchema.Tag{{Name: EncryptedTagPrefix + "Type", Value: "Message"}}

	require.Error(t, ValidateEncryptedTagNames(tags))
	_, err = EncryptedPlainTagNames(tags)
	require.Error(t, err)
	_, _, err = EncryptTags(tags, nodeBundler.Owner, KeyTypeEthereumECIES)
	require.Error(t, err)
}
