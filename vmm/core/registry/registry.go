package registry

import (
	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

var log = common.NewLog("registry")

type Registry struct {
	db schema.IDB
}

func New(db schema.IDB) (*Registry, error) {
	return &Registry{db}, nil
}

func (r *Registry) Apply(from string, meta vmmSchema.Meta) (res vmmSchema.Result) {
	var err error
	switch meta.Action {
	case "Register":
		// only accept messages from the Token process
		res, err = r.handleRegister(from, meta.Params)
	case "Unregister":
		// only accept messages from the Token process
		res, err = r.handleUnregister(from, meta.Params)
	case "RegisterProcess":
		// only accept messages from the the process itself
		res, err = r.handleRegisterProcess(from, meta.Params)
	case "UnregisterProcess":
		// only accept messages from the the process itself
		res, err = r.handleUnregisterProcess(from, meta.Params)
	default:
		err = schema.ErrInvalidAction
	}
	if err != nil {
		res.Error = err.Error()
	}

	return
}

func (r *Registry) Close() error {
	return nil
}

func (r *Registry) GetId() string {
	id, _ := r.db.GetId()
	return id
}

func (r *Registry) GetNode(accid string) (*schema.Node, error) {
	return r.db.GetNode(accid)
}

func (r *Registry) GetNodes() (map[string]schema.Node, error) {
	return r.db.GetNodes()
}

func (r *Registry) GetProcesses(accid string) ([]string, error) {
	return r.db.GetProcesses(accid)
}

func (r *Registry) GetNodesByProcess(pid string) ([]schema.Node, error) {
	return r.db.GetNodesByProcess(pid)
}

func (r *Registry) Checkpoint() (string, error) {
	return r.db.Checkpoint()
}

func (r *Registry) Restore(data string) error {
	return r.db.Restore(data)
}
