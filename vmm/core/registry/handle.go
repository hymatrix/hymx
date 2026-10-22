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

// authSource checks the signer against the registry's own state:
//   - RegisterProcess / UnregisterProcess come from the process itself (Pid == From-Process),
//     signed by the registered node named in Acc-Id;
//   - RegisterProcess may bind a process that has no host yet, or re-register its current host;
//   - UnregisterProcess and any other message must be signed by a node that hosts From-Process.
func (r *Registry) authSource(meta vmmSchema.Meta) (err error) {
	if meta.FromProcess == "" {
		return
	}

	nodes, err := r.db.GetNodesByProcess(meta.FromProcess)
	if err != nil {
		return
	}
	hosted := false
	for _, node := range nodes {
		if node.AccId == meta.AccId {
			hosted = true
			break
		}
	}

	switch meta.Action {
	case "RegisterProcess", "UnregisterProcess":
		if meta.Params["Pid"] != meta.FromProcess || meta.Params["Acc-Id"] != meta.AccId {
			return schema.ErrUnauthorized
		}
		node, err := r.db.GetNode(meta.AccId)
		if err != nil {
			return err
		}
		if node == nil {
			return schema.ErrNodeNotFound
		}
		if hosted || (meta.Action == "RegisterProcess" && len(nodes) == 0) {
			return nil
		}
		return schema.ErrUnauthorized
	default:
		if hosted {
			return nil
		}
		return schema.ErrUnauthorized
	}
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
