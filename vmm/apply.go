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
	from, err := v.applyCheck(vm, env, meta)
	if err != nil {
		return err
	}

	res := vm.Apply(from, meta)
	vmmRes := schema.VmmResult{
		// from meta info and vmm env
		Nonce:       fmt.Sprintf("%d", env.Nonce),
		Timestamp:   fmt.Sprintf("%d", meta.Timestamp),
		ItemId:      meta.ItemId,
		FromProcess: meta.Pid,
		// from vm result
		Messages:    res.Messages,
		Spawns:      res.Spawns,
		Assignments: res.Assignments,
		Output:      res.Output,
		Data:        res.Data,
		Cache:       res.Cache,
	}
	if res.Error != nil {
		vmmRes.Error = res.Error.Error()
	}
	if meta.PushedFor != "" {
		vmmRes.PushedFor = meta.PushedFor
	}
	vmmRes.DryRun = meta.DryRun
	// send message outbox
	v.outbox(env, &vmmRes)
	if meta.DryRun && meta.Nonce == meta.RecoveryMaxNonce {
		v.RecoveryUnlock(meta.Pid)
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
		} else {
			env.ReceivedSeq[from] = m.Sequence
		}
	}

	return
}
