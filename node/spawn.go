package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hymatrix/hymx/node/schema"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (n *Node) handleProcess(
	pid, accid string,
	item goarSchema.BundleItem, proc hymxSchema.Process,
	dryRun bool, maxNonce int64,
) (err error) {
	params, err := utils.TagsToParams(proc.Tags)
	if err != nil {
		return err
	}
	meta := vmmSchema.Meta{
		ItemId:           item.Id,
		Pid:              pid,
		AccId:            accid,
		FromProcess:      proc.FromProcess,
		PushedFor:        proc.PushedFor,
		Nonce:            0,
		Timestamp:        0, // ues 0 now. todo: use assignment timestamp
		Params:           params,
		Data:             item.Data,
		DryRun:           dryRun,
		RecoveryMaxNonce: maxNonce,
	}

	module, err := n.LoadModule(proc.Module)
	if err != nil {
		log.Error("load module failed", "module", proc.Module)
		return err
	}

	return n.vmm.Spawn(meta, proc, module)
}

func (n *Node) LoadModule(itemId string) (module hymxSchema.Module, err error) {
	// todo: download from arweave network
	filename := filepath.Join("mod", fmt.Sprintf("mod-%s.json", itemId))

	_, err = os.Stat(filename)
	if os.IsNotExist(err) {
		err = schema.ErrNotFoundMod
		log.Error("load module failed, ", "id", itemId)
		return
	} else if err != nil {
		return
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	var item goarSchema.BundleItem
	if err = json.Unmarshal(data, &item); err != nil {
		return
	}

	return utils.TagsToModule(item.Tags)
}
