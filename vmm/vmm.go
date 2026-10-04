package vmm

import (
	"context"
	"maps"
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
	coreMu   sync.RWMutex
	token    *token.Token

	vmFactors map[string]schema.VmSpawnFunc // moduleFormat -> vmSpawnFunc
	vms       map[string]*schema.VmInstance // pid -> worker, VM and environment
	vmsLockMu sync.RWMutex

	vmsRecoveryLock map[string]bool // vm lock: key pid, value true/false

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	resultChan      chan<- schema.VmmResult
	outboxChan      chan<- schema.Outbox
	closing         bool
	closeOnce       sync.Once
	registrySpawned chan struct{}
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
	m.Params = maps.Clone(m.Params)
	if err := v.submit(m.Pid, false, &schema.VmTask{Run: func(instance *schema.VmInstance) error {
		return v.apply(instance, m)
	}}); err != nil {
		log.Error("apply admission failed", "pid", m.Pid, "err", err)
	}
}

func (v *Vmm) Close() {
	v.closeOnce.Do(func() {
		v.vmsLockMu.Lock()
		v.closing = true
		v.cancel()
		v.vmsLockMu.Unlock()
		v.KillAll()
		v.wg.Wait()
	})
}
