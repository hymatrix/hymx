package utils

import (
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/everFinance/goether"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyBundleItemChecksReadTags(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	signer, err := goether.NewSigner(hexutil.Encode(crypto.FromECDSA(key)))
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	tags, err := ProcessToTags(schema.Process{Base: schema.DefaultBaseProcess, Module: "module", Scheduler: "0xA"})
	require.NoError(t, err)
	item, err := bundler.CreateAndSignItem(nil, "", "", tags)
	require.NoError(t, err)
	require.NotEmpty(t, item.TagsBy)

	assert.NoError(t, VerifyBundleItem(item))

	withoutTagsBy := item
	withoutTagsBy.TagsBy = ""
	assert.NoError(t, VerifyBundleItem(withoutTagsBy))

	binary, err := goarUtils.DecodeBundleItem(item.Binary)
	require.NoError(t, err)
	assert.NoError(t, VerifyBundleItem(binary))

	tampered := item
	tampered.Tags = append([]goarSchema.Tag{}, item.Tags...)
	tampered.Tags[len(tampered.Tags)-1].Value = "0xB"
	require.NoError(t, goarUtils.VerifyBundleItem(tampered), "goar alone accepts tampered Tags")
	assert.ErrorIs(t, VerifyBundleItem(tampered), nodeSchema.ErrTagsMismatch)

	tampered.TagsBy = ""
	assert.Error(t, VerifyBundleItem(tampered))
}
