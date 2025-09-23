package vmm

import (
	"fmt"

	hySchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (v *Vmm) Spawn(meta schema.Meta, process hySchema.Process, module hySchema.Module) (err error) {
	pid := meta.Pid

	if v.IsExists(pid) {
		return schema.ErrProcessAlreadyExists
	}

	env := &schema.Env{
		Meta:        meta,
		Id:          pid,
		AccId:       meta.AccId,
		Process:     process,
		Module:      module,
		Nonce:       0,
		Sequence:    -1,
		ReceivedSeq: map[string]int64{},
	}

	vm, err := v.spawn(*env)
	if err != nil {
		return
	}
	v.addVm(vm, env)

	result := v.genSpawnResult(env)
	result.DryRun = meta.DryRun
	// send to outbox
	v.outbox(env, result)
	if meta.DryRun && meta.Nonce == meta.RecoveryMaxNonce {
		v.RecoveryUnlock(meta.Pid)
	}

	return
}

func (v *Vmm) spawn(env schema.Env) (vm schema.Vm, err error) {
	v.vmsLockMu.RLock()

	vmFunc, ok := v.vmFactors[env.Module.ModuleFormat]
	if !ok {
		v.vmsLockMu.RUnlock()
		return nil, schema.ErrInvalidModuleFormat
	}
	v.vmsLockMu.RUnlock()

	return vmFunc(env)
}

func (v *Vmm) genSpawnResult(env *schema.Env) (result *schema.Result) {
	result = &schema.Result{
		Nonce:       fmt.Sprintf("%d", env.Nonce),
		ItemId:      env.Meta.ItemId,
		FromProcess: env.Meta.Pid,
		PushedFor:   env.Meta.ItemId,
		Messages:    []*schema.ResMessage{},
		Data:        "",
		Error:       "",
	}
	if env.Meta.PushedFor != "" {
		result.PushedFor = env.Meta.PushedFor
	}

	// registry process
	if v.registry != nil {
		registerMsg := &schema.ResMessage{
			Target: v.registry.GetId(),
			Tags: []goarSchema.Tag{
				{Name: "Action", Value: "RegisterProcess"},
				{Name: "Pid", Value: env.Id},
				{Name: "Acc-Id", Value: env.Process.Scheduler},
			},
		}
		result.Messages = append(result.Messages, registerMsg)
	}

	// if spawn form process, send 'Spawned 'msg to it
	// Reference tag from ao
	if env.Meta.FromProcess != "" {
		ref := utils.GetTagsValueByDefault("Reference", env.Process.Tags, "0")
		spawnedMsg := &schema.ResMessage{
			Target: env.Meta.FromProcess,
			Tags: []goarSchema.Tag{
				{Name: "Action", Value: "Spawned"},
				{Name: "Process", Value: env.Id},
				{Name: "Reference", Value: ref},
			},
		}
		result.Messages = append(result.Messages, spawnedMsg)
	}
	return
}
