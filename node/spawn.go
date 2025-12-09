package node

import (
	"encoding/json"
	"errors"
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
	pid, accid string, item goarSchema.BundleItem,
	proc hymxSchema.Process,
) (err error) {
	// check if scheduler is not node accid
	if proc.Scheduler != n.bundler.Address {
		err = schema.ErrSpawnRedirect
		log.Warn("handle process failed", "pid", pid, "scheduler", proc.Scheduler, "nodeAccId", n.bundler.Address, "err", err)
		return
	}

	// check if process already register
	_, nodes, err := n.IsRedirect(pid)
	if err != nil {
		return
	}
	if len(nodes) != 0 {
		err = schema.ErrProcessAlreadyExists
		log.Error("handle process failed", "pid", pid, "err", err)
		return
	}

	// check if the process already exists before assignment
	// assigning to an invalid (already spawned) process may result in a non-contiguous nonce sequence.
	if n.vmm.IsExists(pid) {
		err = schema.ErrProcessAlreadyExists
		log.Error("handle process failed", "pid", pid, "err", err)
		return
	}

	n.assignProcChan <- schema.AssignProcess{
		Pid:     pid,
		AccId:   accid,
		Process: proc,
		Item:    item,
	}
	return nil
}

func (n *Node) applyProcess(
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
	// try load from local first
	module, err = n.loadModuleByLocal(itemId)
	if err == nil {
		return
	}
	// try load from chainkit if not found in local
	return n.loadModuleByChainkit(itemId)
}

func (n *Node) loadModuleByLocal(itemId string) (module hymxSchema.Module, err error) {
	filename := filepath.Join("mod", fmt.Sprintf("mod-%s.json", itemId))

	_, err = os.Stat(filename)
	if os.IsNotExist(err) {
		log.Info("load module from local failed", "id", itemId)
		err = schema.ErrNotFoundMod
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

func (n *Node) loadModuleByChainkit(itemId string) (module hymxSchema.Module, err error) {
	if n.sdk == nil {
		return module, errors.New("sdk not initialized")
	}

	bundleItem, err := n.sdk.DownloadModuleFromArweave(itemId)
	if err != nil {
		return module, err
	}

	return utils.TagsToModule(bundleItem.Tags)
}
