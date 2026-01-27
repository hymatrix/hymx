package vmm

import (
	"fmt"

	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

// outbox, manage out msg, sequence
func (v *Vmm) outbox(env *schema.Env, result *schema.VmmResult) {
	for _, msg := range result.Messages {
		if _, err := utils.TagsToMessage(msg.Tags); err != nil {
			log.Error("invalid msg tags", "err", err)
			continue
		}

		v.vmsLockMu.Lock()
		env.Sequence += 1
		msg.Sequence = fmt.Sprintf("%d", env.Sequence)
		v.vmsLockMu.Unlock()

		if result.Mode != schema.ExecModeApply {
			continue
		}

		tags, _ := utils.MessageToTags(hymxSchema.Message{
			Base: hymxSchema.DefaultBaseMessage,
		})
		tags = utils.MergeTags([]goarSchema.Tag{
			{Name: "From-Process", Value: result.FromProcess},
			{Name: "Sequence", Value: msg.Sequence},
			{Name: "Pushed-For", Value: result.PushedFor},
		}, tags)
		tags = utils.MergeTags(tags, msg.Tags)

		v.outboxChan <- schema.Outbox{
			Type: hymxSchema.TypeMessage,
			To:   msg.Target,
			From: result.FromProcess,
			Data: msg.Data,
			Tags: tags,
		}
	}

	for _, spawn := range result.Spawns {
		proc, err := utils.TagsToProcess(spawn.Tags)
		if err != nil {
			log.Error("invalid process tags", "err", err)
			continue
		}

		v.vmsLockMu.Lock()
		env.Sequence += 1
		spawn.Sequence = fmt.Sprintf("%d", env.Sequence)
		v.vmsLockMu.Unlock()

		if result.Mode != schema.ExecModeApply {
			continue
		}

		tags, _ := utils.ProcessToTags(hymxSchema.Process{
			Base:      hymxSchema.DefaultBaseProcess,
			Module:    proc.Module,
			Scheduler: proc.Scheduler,
		})
		tags = utils.MergeTags([]goarSchema.Tag{
			{Name: "From-Process", Value: result.FromProcess},
			{Name: "Sequence", Value: spawn.Sequence},
			{Name: "Pushed-For", Value: result.PushedFor},
		}, tags)
		tags = utils.MergeTags(tags, proc.Tags)

		v.outboxChan <- schema.Outbox{
			Type: hymxSchema.TypeProcess,
			To:   proc.Scheduler,
			From: result.FromProcess,
			Data: spawn.Data,
			Tags: tags,
		}
	}

	// send to result chan
	v.resultChan <- *result
}
