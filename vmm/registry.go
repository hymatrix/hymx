package vmm

import (
	"github.com/hymatrix/hymx/db/cache"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/vmm/core/registry"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/hymatrix/hymx/vmm/schema"
)

func (v *Vmm) RegistryId() string {
	if v.registry == nil {
		return ""
	}
	return v.registry.GetId()
}

func (v *Vmm) GetNode(accid string) (*registrySchema.Node, error) {
	if v.registry == nil {
		return nil, nil
	}
	return v.registry.GetNode(accid)
}

func (v *Vmm) GetNodes() (map[string]registrySchema.Node, error) {
	if v.registry == nil {
		return nil, nil
	}
	return v.registry.GetNodes()
}

func (v *Vmm) GetProcesses(accid string) ([]string, error) {
	if v.registry == nil {
		return nil, nil
	}
	return v.registry.GetProcesses(accid)
}

func (v *Vmm) GetNodesByProcess(pid string) ([]registrySchema.Node, error) {
	if v.registry == nil {
		return nil, nil
	}
	return v.registry.GetNodesByProcess(pid)
}

func (v *Vmm) spawnRegistry(env schema.Env) (vm schema.Vm, err error) {
	if v.registry != nil {
		return nil, schema.ErrRegistryAlreadyCreated
	}
	tokenPid, ok := env.Meta.Params["Token-Pid"]
	if !ok {
		return nil, schema.ErrMissingParam
	}

	if !env.Meta.DryRun {
		v.info.Node.Role = registrySchema.RoleMain
		v.info.JoinNetwork = true
	}

	db := cache.NewRegistry(env.Meta.ItemId, tokenPid, nodeSchema.GenesisNode)
	regVm, err := registry.New(db)
	if err != nil {
		return
	}

	v.registry = regVm
	regVm.Apply(tokenPid, schema.Meta{
		Action: "RegisterProcess",
		Params: map[string]string{
			"Pid":    tokenPid,
			"Acc-Id": env.Meta.AccId,
		}})
	regVm.Apply(env.Meta.ItemId, schema.Meta{
		Action: "RegisterProcess",
		Params: map[string]string{
			"Pid":    env.Meta.ItemId,
			"Acc-Id": env.Meta.AccId,
		}})

	return regVm, nil
}
