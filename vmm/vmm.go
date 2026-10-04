package vmm

import (
	"context"
	"sync"

	"github.com/hymatrix/hymx/common"
	"github.com/hymatrix/hymx/cryptor"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/vmm/core/registry"
	"github.com/hymatrix/hymx/vmm/core/token"
	"github.com/hymatrix/hymx/vmm/schema"
)

var log = common.NewLog("vmm")

// VirtualMachine Management
type Vmm struct {
	cryptor *cryptor.Cryptor

	info     *nodeSchema.Info
	registry *registry.Registry
	token    *token.Token
	coreMu   sync.RWMutex

	vmFactors map[string]schema.VmSpawnFunc // moduleFormat -> vmSpawnFunc
	vms       map[string]*schema.VmInstance // pid -> serial VM instance
	vmsLockMu sync.RWMutex
	closed    bool
	closeOnce sync.Once

	vmsRecoveryLock map[string]bool // vm lock: key pid, value true/false

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	resultChan      chan<- schema.VmmResult
	outboxChan      chan<- schema.Outbox
	registrySpawned chan struct{}
	registryReady   sync.Once
}

func New(cryptor *cryptor.Cryptor, info *nodeSchema.Info, resultChan chan<- schema.VmmResult, outboxChan chan<- schema.Outbox, registrySpawned chan struct{}) *Vmm {
	ctx, cancel := context.WithCancel(context.Background())
	return &Vmm{
		cryptor: cryptor,

		info: info,

		vmFactors: map[string]schema.VmSpawnFunc{},
		vms:       map[string]*schema.VmInstance{},

		vmsRecoveryLock: map[string]bool{},

		ctx:    ctx,
		cancel: cancel,

		resultChan:      resultChan,
		outboxChan:      outboxChan,
		registrySpawned: registrySpawned,
	}
}

func (v *Vmm) Run() {
	// mount core token & registry spawner
	v.Mount(schema.ModuleFormatToken, v.spawnToken)
	v.Mount(schema.ModuleFormatRegistry, v.spawnRegistry)
}

func (v *Vmm) Apply(m schema.Meta) {
	instance, err := v.getInstance(m.Pid)
	if err == nil {
		m = cloneMeta(m)
		err = v.submit(instance, &schema.VmTask{Run: func(instance *schema.VmInstance) error {
			return v.apply(instance.Vm, instance.Env, m)
		}})
	}
	if err != nil {
		log.Error("apply failed", "pid", m.Pid, "itemId", m.ItemId, "err", err)
	}
}

func (v *Vmm) Close() {
	v.closeOnce.Do(func() {
		log.Info("vmm is shutting down")
		v.vmsLockMu.Lock()
		v.closed = true
		instances := make([]*schema.VmInstance, 0, len(v.vms))
		for _, instance := range v.vms {
			instances = append(instances, instance)
		}
		v.vmsLockMu.Unlock()
		v.cancel()
		for _, instance := range instances {
			v.stopInstance(instance)
		}
		v.wg.Wait()
		log.Info("vmm has been shut down")
	})
}
