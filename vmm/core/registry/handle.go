package registry

import (
	"fmt"

	"github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/hymatrix/hymx/vmm/core/registry/utils"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func (r *Registry) handleRegister(from string, params map[string]string) (res vmmSchema.Result, err error) {
	if err = r.authToken(from); err != nil {
		return
	}

	node, err := utils.Decode(params)
	if err != nil {
		return
	}

	if err = r.db.Register(node); err != nil {
		return
	}

	res.Data = fmt.Sprintf("Node registered successfully, Acc-Id:%v", node.AccId)
	return
}

func (r *Registry) handleUnregister(from string, params map[string]string) (res vmmSchema.Result, err error) {
	if err = r.authToken(from); err != nil {
		return
	}

	accid, ok := params["Acc-Id"]
	if !ok {
		err = schema.ErrMissingParam
		return
	}

	if err = r.db.Unregister(accid); err != nil {
		return
	}

	res.Data = fmt.Sprintf("Node unregistered successfully, Acc-Id:%v", accid)
	return
}

func (r *Registry) handleRegisterProcess(from string, params map[string]string) (res vmmSchema.Result, err error) {
	pid, accid, err := r.authProc(from, params)
	if err != nil {
		return
	}

	if err = r.db.RegisterProcess(accid, pid); err != nil {
		return
	}

	res.Data = fmt.Sprintf("Process registered successfully, Acc-Id:%v, Pid:%v", accid, pid)
	return
}

func (r *Registry) handleUnregisterProcess(from string, params map[string]string) (res vmmSchema.Result, err error) {
	pid, accid, err := r.authProc(from, params)
	if err != nil {
		return
	}

	if err = r.db.UnregisterProcess(accid, pid); err != nil {
		return
	}

	res.Data = fmt.Sprintf("Process unregistered successfully, Acc-Id:%v, Pid:%v", accid, pid)
	return
}

// Only receive messages from core Token.
func (r *Registry) authToken(from string) (err error) {
	tokenPid, err := r.db.GetTokenPid()
	if err != nil {
		return
	}
	if from != tokenPid {
		err = schema.ErrUnauthorized
	}
	return
}

// Only receive messages from the process itself.
func (r *Registry) authProc(from string, params map[string]string) (pid, accid string, err error) {
	ok := false
	pid, ok = params["Pid"]
	if !ok {
		err = schema.ErrMissingParam
		return
	}
	if pid != from {
		err = schema.ErrUnauthorized
		return
	}

	accid, ok = params["Acc-Id"]
	if !ok {
		err = schema.ErrMissingParam
		return
	}
	node, err := r.db.GetNode(accid)
	if err != nil {
		return
	}
	if node == nil {
		err = schema.ErrNodeNotFound
		log.Error("node not found", "accid", accid)
	}

	return
}
