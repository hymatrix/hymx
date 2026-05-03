package node

import (
	"encoding/json"
	"testing"

	"github.com/everFinance/goether"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestNodeDecodeKeepsEncryptedTagsRaw(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	encryptedTags, _, err := tagcrypto.EncryptTags([]goarSchema.Tag{
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"},
	}, nodeBundler.Owner, tagcrypto.KeyTypeEthereumECIES)
	require.NoError(t, err)

	rawTags := utils.MergeTags([]goarSchema.Tag{
		{Name: "Data-Protocol", Value: hymxSchema.DataProtocol},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Type", Value: hymxSchema.TypeMessage},
	}, encryptedTags)
	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "", rawTags)
	require.NoError(t, err)

	_, _, _, instance, err := utils.Decode(rawItem)
	require.NoError(t, err)
	msg := instance.(hymxSchema.Message)
	require.Empty(t, utils.GetTagsValue("Secret", msg.Tags))
	require.NotEmpty(t, utils.GetTagsValue(tagcrypto.EncryptedTagPrefix+"Secret", msg.Tags))
}

func TestCheckpointSnapshotKeepsRawEncryptedEnv(t *testing.T) {
	snap := vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid: "process-id",
				Params: map[string]string{
					"Encrypted-Secret": "hymxenc:v1:ethereum-ecies:ciphertext",
				},
				DecryptedParams: map[string]string{"Encrypted-Secret": "private-value"},
			},
			Process: hymxSchema.Process{
				Tags: []goarSchema.Tag{{Name: "Encrypted-Secret", Value: "hymxenc:v1:ethereum-ecies:ciphertext"}},
			},
		},
		Data: "vm-state",
	}
	by, err := json.Marshal(snap)
	require.NoError(t, err)
	require.Contains(t, string(by), "Encrypted-Secret")
	require.Contains(t, string(by), "ciphertext")
	require.NotContains(t, string(by), "private-value")
}
