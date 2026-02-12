package vmm

import (
	"fmt"
	"strings"

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
	if v.registry == nil && module.ModuleFormat != schema.ModuleFormatRegistry && module.ModuleFormat != schema.ModuleFormatToken {
		if err = v.waitRegistrySpawned(pid); err != nil {
			return err
		}
	}

	env := &schema.Env{
		Meta:        meta,
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
	result.Mode = meta.Mode
	// send to outbox
	v.outbox(env, result)
	if meta.Mode != schema.ExecModeApply && meta.Nonce == meta.RecoveryMaxNonce {
		v.RecoveryUnlock(meta.Pid)
	}

	return
}

func (v *Vmm) waitRegistrySpawned(pid string) error {
	if v.registry != nil {
		return nil
	}
	log.Debug("wait for registry spawned", "pid", pid)
	select {
	case <-v.ctx.Done():
		return schema.ErrRegistryNotFound
	case <-v.registrySpawned:
		log.Debug("registry spawned! go on", "pid", pid)
		return nil
	}
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

func (v *Vmm) genSpawnResult(env *schema.Env) (result *schema.VmmResult) {
	result = &schema.VmmResult{
		Nonce:       fmt.Sprintf("%d", env.Nonce),
		ItemId:      env.Meta.ItemId,
		FromProcess: env.Meta.Pid,
		PushedFor:   env.Meta.ItemId,
		Messages:    []*schema.ResMessage{},
		Data:        "",
		Timestamp:   fmt.Sprintf("%d", env.Meta.Timestamp),
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
				{Name: "Pid", Value: env.Meta.Pid},
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
				{Name: "Process", Value: env.Meta.Pid},
				{Name: "Reference", Value: ref},
			},
		}
		// Forward X- prefixed tags to message
		for key, value := range env.Meta.Params {
			if strings.HasPrefix(key, "X-") {
				spawnedMsg.Tags = append(spawnedMsg.Tags, goarSchema.Tag{Name: key, Value: value})
			}
		}

		result.Messages = append(result.Messages, spawnedMsg)
	}
	return
}
