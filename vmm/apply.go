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

	result, err := vm.Apply(from, meta)
	if err != nil {
		return err
	}

	// set result
	if result == nil {
		log.Warn("no result response from process", "meta", meta)
		result = &schema.Result{}
	}
	result.Nonce = fmt.Sprintf("%d", env.Nonce)
	result.FromProcess = meta.Pid
	result.ItemId = meta.ItemId
	result.PushedFor = meta.ItemId
	if meta.PushedFor != "" {
		result.PushedFor = meta.PushedFor
	}
	// send message outbox
	v.outbox(env, result, meta.RecoveryDryRun)
	if meta.RecoveryDryRun && meta.Nonce == meta.RecoveryMaxNonce {
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
