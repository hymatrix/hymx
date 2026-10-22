package utils

import (
	"testing"

	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyBundleItemChecksReadTags(t *testing.T) {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	item, err := bundler.CreateAndSignItem([]byte("data"), "", "", []goarSchema.Tag{
		{Name: "Type", Value: "Process"}, {Name: "Scheduler", Value: "A"},
	})
	require.NoError(t, err)
	require.NoError(t, VerifyBundleItem(item))
	for _, tags := range [][]goarSchema.Tag{
		{{Name: "Type", Value: "Process"}, {Name: "Scheduler", Value: "B"}},
		item.Tags[:1], append(append([]goarSchema.Tag{}, item.Tags...), item.Tags[0]),
		{item.Tags[1], item.Tags[0]},
	} {
		changed := item
		changed.Tags = tags
		require.NoError(t, goarUtils.VerifyBundleItem(changed))
		assert.ErrorIs(t, VerifyBundleItem(changed), nodeSchema.ErrTagsMismatch)
	}
	withoutTagsBy := item
	withoutTagsBy.TagsBy = ""
	assert.NoError(t, VerifyBundleItem(withoutTagsBy))
	for _, tagsBy := range []string{"!", goarUtils.Base64Encode([]byte{2})} {
		changed := item
		changed.TagsBy = tagsBy
		assert.Error(t, VerifyBundleItem(changed))
	}
	// An Avro array may encode each record in its own block.
	first, err := goarUtils.SerializeTags(item.Tags[:1])
	require.NoError(t, err)
	second, err := goarUtils.SerializeTags(item.Tags[1:])
	require.NoError(t, err)
	item.TagsBy = goarUtils.Base64Encode(append(first[:len(first)-1], second...))
	require.NoError(t, bundler.Sign(&item))
	assert.NoError(t, VerifyBundleItem(item))
}

func TestDecodeBundleItemRejectsTruncationAndTampering(t *testing.T) {
	signer, err := goar.NewSignerFromPath("../cmd/test_keyfile.json")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	item, err := bundler.CreateAndSignItem([]byte("data"), "", "", nil)
	require.NoError(t, err)
	got, err := DecodeBundleItem(item.Binary)
	require.NoError(t, err)
	assert.Equal(t, item.Id, got.Id)
	for i := 0; i < len(item.Binary); i++ {
		got, err := DecodeBundleItem(item.Binary[:i])
		require.Error(t, err, "prefix length %d", i)
		assert.Empty(t, got.Id)
	}
	changed := append([]byte{}, item.Binary...)
	changed[len(changed)-1] ^= 1
	_, err = DecodeBundleItem(changed)
	assert.Error(t, err)
}
