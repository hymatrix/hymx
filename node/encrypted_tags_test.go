package node

import (
	"encoding/json"
	"testing"

	"github.com/everFinance/goether"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	"github.com/hymatrix/hymx/vmm"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestDecryptInternalItemDecryptsCustomTagsWithoutMutatingRawItem(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	encryptedTags, changed, err := tagcrypto.EncryptTags(
		[]goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}},
		nodeBundler.Owner,
		tagcrypto.KeyTypeEthereumECIES,
	)
	require.NoError(t, err)
	require.True(t, changed)

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
	rawCiphertext := utils.GetTagsValue(tagcrypto.EncryptedTagPrefix+"Secret", rawItem.Tags)
	require.NotEmpty(t, rawCiphertext)

	n := &Node{
		signer: nodeSigner,
		db:     checkpointTestDB{items: map[string]goarSchema.BundleItem{rawItem.Id: rawItem}},
	}
	internalItem, instance, err := n.decryptInternalItem(rawItem)
	require.NoError(t, err)

	require.Equal(t, rawCiphertext, utils.GetTagsValue(tagcrypto.EncryptedTagPrefix+"Secret", rawItem.Tags))
	require.Empty(t, utils.GetTagsValue("Secret", rawItem.Tags))
	require.Equal(t, "private-value", utils.GetTagsValue("Secret", internalItem.Tags))

	msg, ok := instance.(hymxSchema.Message)
	require.True(t, ok)
	require.Equal(t, "private-value", utils.GetTagsValue("Secret", msg.Tags))
}

func TestDecryptInternalItemRejectsEncryptedReservedTag(t *testing.T) {
	rawTags := []goarSchema.Tag{
		{Name: "Data-Protocol", Value: hymxSchema.DataProtocol},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Type", Value: hymxSchema.TypeMessage},
		{Name: tagcrypto.EncryptedTagPrefix + "Type", Value: tagcrypto.CipherValuePrefix + ":" + tagcrypto.KeyTypeEthereumECIES + ":bad"},
	}

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "", rawTags)
	require.NoError(t, err)

	n := &Node{signer: userSigner}
	_, _, err = n.decryptInternalItem(rawItem)
	require.Error(t, err)
}

func TestDecodeInternalItemDecryptsReplayMessage(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	encryptedTags, _, err := tagcrypto.EncryptTags(
		[]goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}},
		nodeBundler.Owner,
		tagcrypto.KeyTypeEthereumECIES,
	)
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

	n := &Node{
		signer: nodeSigner,
		db:     checkpointTestDB{items: map[string]goarSchema.BundleItem{rawItem.Id: rawItem}},
	}
	_, _, _, instance, err := n.decodeInternalItem(rawItem)
	require.NoError(t, err)
	msg, ok := instance.(hymxSchema.Message)
	require.True(t, ok)
	require.Equal(t, "private-value", utils.GetTagsValue("Secret", msg.Tags))
}

func TestHandleRoutesEncryptedSpawnBeforeDecrypting(t *testing.T) {
	localSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	localBundler, err := goar.NewBundler(localSigner)
	require.NoError(t, err)
	schedulerSigner, err := goether.NewSigner("0x1111111111111111111111111111111111111111111111111111111111111111")
	require.NoError(t, err)
	schedulerBundler, err := goar.NewBundler(schedulerSigner)
	require.NoError(t, err)

	encryptedTags, _, err := tagcrypto.EncryptTags(
		[]goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}},
		schedulerBundler.Owner,
		tagcrypto.KeyTypeEthereumECIES,
	)
	require.NoError(t, err)
	rawProc := hymxSchema.Process{
		Base:      hymxSchema.DefaultBaseProcess,
		Module:    "module-id",
		Scheduler: schedulerBundler.Address,
		Tags:      encryptedTags,
	}
	rawTags, err := utils.ProcessToTags(rawProc)
	require.NoError(t, err)
	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "", "", rawTags)
	require.NoError(t, err)

	n := &Node{
		signer:  localSigner,
		bundler: localBundler,
		vmm:     vmm.New(&nodeSchema.Info{}, make(chan vmmSchema.VmmResult), make(chan vmmSchema.Outbox), make(chan struct{}, 1)),
	}

	err = n.Handle(rawItem)
	require.ErrorIs(t, err, nodeSchema.ErrSpawnRedirect)
}

func TestCheckpointSnapshotStoresEncryptedSpawnTags(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)

	encryptedTags, _, err := tagcrypto.EncryptTags(
		[]goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}},
		nodeBundler.Owner,
		tagcrypto.KeyTypeEthereumECIES,
	)
	require.NoError(t, err)

	rawProc := hymxSchema.Process{
		Base:      hymxSchema.DefaultBaseProcess,
		Module:    "module-id",
		Scheduler: nodeBundler.Address,
		Tags:      encryptedTags,
	}
	rawTags, err := utils.ProcessToTags(rawProc)
	require.NoError(t, err)
	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "", "", rawTags)
	require.NoError(t, err)

	n := &Node{
		signer: nodeSigner,
		db:     checkpointTestDB{items: map[string]goarSchema.BundleItem{rawItem.Id: rawItem}},
	}
	internalItem, instance, err := n.decryptInternalItem(rawItem)
	require.NoError(t, err)
	proc := instance.(hymxSchema.Process)
	params, err := utils.TagsToParams(proc.Tags)
	require.NoError(t, err)

	snap := vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				ItemId: internalItem.Id,
				Pid:    internalItem.Id,
				Params: params,
			},
			Process: proc,
		},
	}
	sanitized, err := n.sanitizeCheckpointSnapshot(snap)
	require.NoError(t, err)
	by, err := json.Marshal(sanitized)
	require.NoError(t, err)
	require.NotContains(t, string(by), "private-value")
	require.Contains(t, string(by), tagcrypto.EncryptedTagPrefix+"Secret")

	restored, err := n.decryptSnapshotEnv(sanitized)
	require.NoError(t, err)
	require.Equal(t, "private-value", utils.GetTagsValue("Secret", restored.Env.Process.Tags))
	require.Equal(t, "private-value", restored.Env.Meta.Params["Secret"])
}

func TestCheckpointSnapshotFailsWhenEncryptedRawSpawnIsMissing(t *testing.T) {
	n := &Node{db: checkpointTestDB{items: map[string]goarSchema.BundleItem{}}}
	snap := vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid:             "process-id",
				Params:          map[string]string{"Secret": "private-value"},
				EncryptedParams: map[string]bool{"Secret": true},
			},
			Process: hymxSchema.Process{
				Tags: []goarSchema.Tag{{Name: "Secret", Value: "private-value"}},
			},
		},
	}

	_, err := n.sanitizeCheckpointSnapshot(snap)
	require.Error(t, err)
}

func TestDecryptInternalItemRejectsMalformedEncryptedValue(t *testing.T) {
	rawTags := []goarSchema.Tag{
		{Name: "Data-Protocol", Value: hymxSchema.DataProtocol},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Type", Value: hymxSchema.TypeMessage},
		{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "not-a-cipher"},
	}

	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "", rawTags)
	require.NoError(t, err)

	n := &Node{signer: userSigner}
	internalItem, instance, err := n.decryptInternalItem(rawItem)

	require.Error(t, err)
	require.Empty(t, internalItem.Id)
	require.Nil(t, instance)
}

func TestDecryptInternalItemRejectsWrongSignerType(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	nodeBundler, err := goar.NewBundler(nodeSigner)
	require.NoError(t, err)
	encryptedTags, _, err := tagcrypto.EncryptTags(
		[]goarSchema.Tag{{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "private-value"}},
		nodeBundler.Owner,
		tagcrypto.KeyTypeEthereumECIES,
	)
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
	arweaveSigner, err := goar.NewSignerFromPath("../examples/test_keyfile.json")
	require.NoError(t, err)

	n := &Node{signer: arweaveSigner}
	_, _, err = n.decryptInternalItem(rawItem)

	require.Error(t, err)
	require.Contains(t, err.Error(), "ethereum encrypted tag requires ethereum signer")
}

func TestSanitizeCheckpointSnapshotRejectsNonProcessRawItem(t *testing.T) {
	userSigner, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	userBundler, err := goar.NewBundler(userSigner)
	require.NoError(t, err)
	rawTags := []goarSchema.Tag{
		{Name: "Data-Protocol", Value: hymxSchema.DataProtocol},
		{Name: "Variant", Value: hymxSchema.Variant},
		{Name: "Type", Value: hymxSchema.TypeMessage},
	}
	rawItem, err := userBundler.CreateAndSignItem([]byte("payload"), "lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY", "", rawTags)
	require.NoError(t, err)

	n := &Node{db: checkpointTestDB{items: map[string]goarSchema.BundleItem{"process-id": rawItem}}}
	snap := vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Meta: vmmSchema.Meta{
				Pid: "process-id",
			},
		},
	}

	_, err = n.sanitizeCheckpointSnapshot(snap)

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid type")
}

func TestDecryptSnapshotEnvRejectsMalformedEncryptedTag(t *testing.T) {
	nodeSigner, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	n := &Node{signer: nodeSigner}
	snap := vmmSchema.Snapshot{
		Env: vmmSchema.Env{
			Process: hymxSchema.Process{
				Tags: []goarSchema.Tag{
					{Name: tagcrypto.EncryptedTagPrefix + "Secret", Value: "not-a-cipher"},
				},
			},
			Meta: vmmSchema.Meta{
				Params: map[string]string{"Plain": "public-value"},
			},
		},
	}

	restored, err := n.decryptSnapshotEnv(snap)

	require.Error(t, err)
	require.Equal(t, snap.Env.Meta.Params, restored.Env.Meta.Params)
	require.Equal(t, snap.Env.Process.Tags, restored.Env.Process.Tags)
}

type checkpointTestDB struct {
	items map[string]goarSchema.BundleItem
}

func (db checkpointTestDB) SaveResult(vmmSchema.VmmResult) error { return nil }

func (db checkpointTestDB) GetResult(string) (*vmmSchema.VmmResult, error) { return nil, nil }

func (db checkpointTestDB) GetResults(string, int64) ([]vmmSchema.VmmResult, error) { return nil, nil }

func (db checkpointTestDB) IsExist(string) (bool, error) { return false, nil }

func (db checkpointTestDB) GetNonce(string) (int64, error) { return 0, nil }

func (db checkpointTestDB) Commit(string, int64, goarSchema.BundleItem, goarSchema.BundleItem) error {
	return nil
}

func (db checkpointTestDB) GetAllProcess() ([]string, []int64, error) { return nil, nil, nil }

func (db checkpointTestDB) GetMessage(msgid string) (*goarSchema.BundleItem, error) {
	item, ok := db.items[msgid]
	if !ok {
		return nil, nil
	}
	return &item, nil
}

func (db checkpointTestDB) GetMessageByNonce(string, int64) (*goarSchema.BundleItem, error) {
	return nil, nil
}

func (db checkpointTestDB) GetAssignByNonce(string, int64) (*goarSchema.BundleItem, error) {
	return nil, nil
}

func (db checkpointTestDB) GetCheckpointIndex(string) (string, error) { return "", nil }

func (db checkpointTestDB) SaveCheckpointIndex(string, string) error { return nil }

func (db checkpointTestDB) GetCache(string, string) (string, error) { return "", nil }

func (db checkpointTestDB) SaveCache(string, string, string) error { return nil }
