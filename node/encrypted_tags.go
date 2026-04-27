package node

import (
	"fmt"

	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/utils/tagcrypto"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (n *Node) decryptInternalItem(item goarSchema.BundleItem) (goarSchema.BundleItem, interface{}, error) {
	if !tagcrypto.HasEncryptedTags(item.Tags) {
		_, _, _, instance, err := utils.Decode(item)
		return item, instance, err
	}

	internalItem := item
	tags, _, err := tagcrypto.DecryptTags(item.Tags, n.signer)
	if err != nil {
		return goarSchema.BundleItem{}, nil, err
	}
	internalItem.Tags = tags

	_, _, _, instance, err := utils.Decode(internalItem)
	if err != nil {
		return goarSchema.BundleItem{}, nil, err
	}
	return internalItem, instance, nil
}

func (n *Node) decodeInternalItem(item goarSchema.BundleItem) (string, string, string, interface{}, error) {
	if !tagcrypto.HasEncryptedTags(item.Tags) {
		return utils.Decode(item)
	}

	internalItem := item
	tags, _, err := tagcrypto.DecryptTags(item.Tags, n.signer)
	if err != nil {
		return "", "", "", nil, err
	}
	internalItem.Tags = tags

	return utils.Decode(internalItem)
}

func (n *Node) sanitizeCheckpointSnapshot(snap vmmSchema.Snapshot) (vmmSchema.Snapshot, error) {
	rawItem, err := n.getRawSpawnItem(snap.Env.Meta.Pid)
	if err != nil {
		return snap, err
	}
	if rawItem == nil {
		if len(snap.Env.Meta.EncryptedParams) != 0 {
			return snap, fmt.Errorf("raw encrypted spawn item not found for checkpoint sanitization: %s", snap.Env.Meta.Pid)
		}
		return snap, nil
	}
	if !tagcrypto.HasEncryptedTags(rawItem.Tags) {
		if len(snap.Env.Meta.EncryptedParams) != 0 {
			return snap, fmt.Errorf("raw spawn item has no encrypted tags for encrypted checkpoint state: %s", snap.Env.Meta.Pid)
		}
		return snap, nil
	}

	proc, err := utils.DecodeItemToProcess(*rawItem)
	if err != nil {
		return snap, err
	}
	params, err := utils.TagsToParams(proc.Tags)
	if err != nil {
		return snap, err
	}

	snap.Env.Process = proc
	snap.Env.Meta.Params = params
	snap.Env.Meta.EncryptedParams, err = tagcrypto.EncryptedPlainTagNames(proc.Tags)
	if err != nil {
		return snap, err
	}
	return snap, nil
}

func (n *Node) decryptSnapshotEnv(snap vmmSchema.Snapshot) (vmmSchema.Snapshot, error) {
	if !tagcrypto.HasEncryptedTags(snap.Env.Process.Tags) {
		return snap, nil
	}

	encryptedParams, err := tagcrypto.EncryptedPlainTagNames(snap.Env.Process.Tags)
	if err != nil {
		return snap, err
	}
	tags, _, err := tagcrypto.DecryptTags(snap.Env.Process.Tags, n.signer)
	if err != nil {
		return snap, err
	}
	params, err := utils.TagsToParams(tags)
	if err != nil {
		return snap, err
	}

	snap.Env.Process.Tags = tags
	snap.Env.Meta.Params = params
	snap.Env.Meta.EncryptedParams = encryptedParams
	return snap, nil
}

func (n *Node) getRawSpawnItem(pid string) (*goarSchema.BundleItem, error) {
	if item, ok := n.rawSpawnItem(pid); ok {
		return &item, nil
	}
	if n.db == nil {
		return nil, nil
	}
	item, err := n.db.GetMessage(pid)
	if err != nil || item == nil {
		return item, err
	}
	if utils.GetType(*item) != hymxSchema.TypeProcess {
		return nil, fmt.Errorf("checkpoint process item has invalid type: %s", utils.GetType(*item))
	}
	return item, nil
}

func (n *Node) rememberRawSpawnItem(pid string, item goarSchema.BundleItem) {
	if pid == "" || item.Id == "" || utils.GetType(item) != hymxSchema.TypeProcess {
		return
	}
	n.rawSpawnItemsMu.Lock()
	defer n.rawSpawnItemsMu.Unlock()
	if n.rawSpawnItems == nil {
		n.rawSpawnItems = map[string]goarSchema.BundleItem{}
	}
	n.rawSpawnItems[pid] = item
}

func (n *Node) rawSpawnItem(pid string) (goarSchema.BundleItem, bool) {
	n.rawSpawnItemsMu.RLock()
	defer n.rawSpawnItemsMu.RUnlock()
	if n.rawSpawnItems == nil {
		return goarSchema.BundleItem{}, false
	}
	item, ok := n.rawSpawnItems[pid]
	return item, ok
}
