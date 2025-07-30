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
	goarUtils "github.com/permadao/goar/utils"
)

func (n *Node) runCheckpoint() {
	pids := n.vmm.GetVmPids()
	if len(pids) == 0 {
		return
	}

	for _, pid := range pids {
		ckpItem, err := n.Checkpoint(pid)
		if err != nil {
			log.Warn("generate checkpoint failed", "pid", pid, "err", err)
			continue
		}

		if err := SaveCheckpoint(ckpItem); err != nil {
			log.Error("save checkpoint failed", "pid", pid, "err", err)
			continue
		}

		if err := n.db.SaveCheckpointIndex(pid, ckpItem.Id); err != nil {
			log.Error("save checkpoint index to db failed", "pid", pid, "err", err)
			continue
		}
	}
}

func (n *Node) Restore(ckpId string) (nonce int64, err error) {
	ckpItem, err := LoadCheckpoint(ckpId)
	if err != nil {
		return -1, err
	}

	by, err := goarUtils.Base64Decode(ckpItem.Data)
	if err != nil {
		return -1, err
	}

	snap := vmmSchema.Snapshot{}
	if err = json.Unmarshal(by, &snap); err != nil {
		return -1, err
	}

	if err = n.vmm.Restore(snap); err != nil {
		return -1, err
	}

	return snap.Env.Nonce, n.outboxDB.Restore(snap.Outbox)
}

func (n *Node) Checkpoint(pid string) (ckpItem goarSchema.BundleItem, err error) {
	snap, err := n.vmm.Checkpoint(pid)
	if err != nil {
		return
	}

	outSnap, err := n.outboxDB.Checkpoint(pid)
	if err != nil {
		return
	}
	snap.Outbox = outSnap

	return n.signCheckpoint(snap)
}

func (n *Node) signCheckpoint(snap vmmSchema.Snapshot) (ckpItem goarSchema.BundleItem, err error) {
	ckp := hymxSchema.Checkpoint{
		Base:    hymxSchema.DefaultCheckpoint,
		Process: snap.Env.Id,
		Nonce:   fmt.Sprintf("%d", snap.Env.Nonce),
	}

	tags, err := utils.CheckpointToTags(ckp)
	if err != nil {
		return
	}

	by, err := json.Marshal(snap)
	if err != nil {
		return
	}

	return n.bundler.CreateAndSignItem(by, "", "", tags)
}

func SaveCheckpoint(ckpItem goarSchema.BundleItem) error {
	if err := os.MkdirAll("./ckp", 0755); err != nil {
		return err
	}

	ckpBin, err := json.Marshal(ckpItem)
	if err != nil {
		return err
	}

	filename := filepath.Join("ckp", fmt.Sprintf("ckp-%s.json", ckpItem.Id))
	if err := os.WriteFile(filename, ckpBin, 0644); err != nil {
		return err
	}

	return nil
}

func LoadCheckpoint(itemId string) (ckpItem goarSchema.BundleItem, err error) {
	// todo: download from arweave network
	filename := filepath.Join("ckp", fmt.Sprintf("ckp-%s.json", itemId))

	_, err = os.Stat(filename)
	if os.IsNotExist(err) {
		err = schema.ErrNotFoundCkp
		return
	} else if err != nil {
		return
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return
	}

	err = json.Unmarshal(data, &ckpItem)
	return
}
