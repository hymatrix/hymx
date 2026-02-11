package vmm

import (
	"fmt"

	"github.com/hymatrix/hymx/vmm/schema"
)

func (v *Vmm) apply(meta schema.Meta) error {
	vm, env, err := v.GetVm(meta.Pid)
	if err != nil {
		return err
	}
	log.Debug("===> apply", "meta", meta, "env", env)

	vmmRes := schema.VmmResult{
		Nonce:       fmt.Sprintf("%d", meta.Nonce),
		Timestamp:   fmt.Sprintf("%d", meta.Timestamp),
		ItemId:      meta.ItemId,
		FromProcess: meta.Pid,
		PushedFor:   meta.ItemId,
		Mode:        meta.Mode,
	}
	if meta.PushedFor != "" {
		vmmRes.PushedFor = meta.PushedFor
	}
	defer func() {
		// send message outbox
		v.outbox(env, &vmmRes)
		// recovery unlock
		if meta.Mode != schema.ExecModeApply && meta.Nonce == meta.RecoveryMaxNonce {
			v.RecoveryUnlock(meta.Pid)
		}
	}()

	from, err := v.applyCheck(vm, env, meta)
	if err != nil {
		vmmRes.Error = err.Error()
		return err
	}

	res := vm.Apply(from, meta)
	vmmRes.Messages = res.Messages
	vmmRes.Spawns = res.Spawns
	vmmRes.Assignments = res.Assignments
	vmmRes.Output = res.Output
	vmmRes.Data = res.Data
	vmmRes.Cache = res.Cache
	if res.Error != nil {
		vmmRes.Error = res.Error.Error()
	}
	return nil
}

func (v *Vmm) applyCheck(vm schema.Vm, env *schema.Env, m schema.Meta) (from string, err error) {
	// nonce & sequence
	if m.Nonce != env.Nonce+1 {
		err = schema.ErrInvalidNonce
		log.Error("invalid nonce", "cur nonce", m.Nonce, "env nonce", env.Nonce)
		return
	}
	env.Nonce = m.Nonce

	from = m.AccId
	if m.FromProcess != "" {
		from = m.FromProcess
		// Prevent rollback or replay attacks by rejecting transactions with a lower sequence number.
		if lastSeq, ok := env.ReceivedSeq[from]; ok {
			if lastSeq >= m.Sequence {
				log.Error("sequence too low", "curSeq", m.Sequence, "lastSeq", lastSeq)
				err = schema.ErrSequenceTooLow
				return
			}
		}
		env.ReceivedSeq[from] = m.Sequence
	}

	return
}
