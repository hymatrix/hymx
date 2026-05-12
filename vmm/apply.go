package vmm

import (
	"fmt"
	"strings"

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
	v.decryptParams(&meta)

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

func (v *Vmm) decryptParams(meta *schema.Meta) {
	if len(meta.Params) == 0 {
		return
	}

	decryptedParams := map[string]string{}
	for key, value := range meta.Params {
		if !strings.HasPrefix(key, schema.EncryptedTagPrefix) {
			continue
		}
		decryptedKey := strings.TrimPrefix(key, schema.EncryptedTagPrefix)
		if _, ok := meta.Params[decryptedKey]; ok {
			log.Warn("skip encrypted param because plaintext param exists", "pid", meta.Pid, "itemId", meta.ItemId, "key", key, "plaintextKey", decryptedKey)
			continue
		}
		if v.cryptor == nil {
			log.Warn("decrypt encrypted param failed", "pid", meta.Pid, "itemId", meta.ItemId, "key", key, "err", schema.ErrMissingDecryptor)
			decryptedParams[decryptedKey] = schema.ErrMissingDecryptor.Error()
			continue
		}
		decryptedValue, err := v.cryptor.Decrypt(value)
		if err != nil {
			log.Warn("decrypt encrypted param failed", "pid", meta.Pid, "itemId", meta.ItemId, "key", key, "err", err)
			decryptedParams[decryptedKey] = err.Error()
			continue
		}
		decryptedParams[decryptedKey] = decryptedValue
	}
	for key, value := range decryptedParams {
		meta.Params[key] = value
	}
}

func (v *Vmm) applyCheck(vm schema.Vm, env *schema.Env, m schema.Meta) (from string, err error) {
	// nonce & sequence
	if m.Nonce != env.Nonce+1 {
		err = schema.ErrInvalidNonce
		log.Error("invalid nonce", "cur nonce", m.Nonce, "env nonce", env.Nonce)
		return
	}
	// todo: Because the earlier if condition doesn’t update env.Nonce, the Nonce in the node database can drift out of sync with the VM’s environment Nonce, causing all subsequent transactions from that VM to fail. To resolve this, the Nonce on both the environment and the node side needs to be brought back into sync. A new slashing mechanism should be added so that if a node assigns an incorrect Nonce that doesn’t match the VM’s env Nonce, it gets penalized.
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
