package vmm

import (
	"context"
	"sync"

	"github.com/hymatrix/hymx/common"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/vmm/core/registry"
	"github.com/hymatrix/hymx/vmm/core/token"
	"github.com/hymatrix/hymx/vmm/schema"
)

var log = common.NewLog("vmm")

// VirtualMachine Management
type Vmm struct {
	info     *nodeSchema.Info
	registry *registry.Registry
	token    *token.Token

	vmFactors map[string]schema.VmSpawnFunc // moduleFormat -> vmSpawnFunc
	vms       map[string]schema.Vm          // pid -> virtual machine
	vmsEnv    map[string]*schema.Env        // pid -> vm env, eg: module, prcoess, nonce, ref
	vmsLockMu sync.RWMutex

	vmsRecoveryLock map[string]bool // vm lock: key pid, value true/false

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	resultChan chan<- schema.Result
	outboxChan chan<- schema.Outbox
	applyChan  chan schema.Meta
	ckpChan    chan schema.Checkpoint
}

func New(info *nodeSchema.Info, resultChan chan<- schema.Result, outboxChan chan<- schema.Outbox) *Vmm {
	ctx, cancel := context.WithCancel(context.Background())
	return &Vmm{
		info: info,

		vmFactors: map[string]schema.VmSpawnFunc{},
		vms:       map[string]schema.Vm{},
		vmsEnv:    map[string]*schema.Env{},

		vmsRecoveryLock: map[string]bool{},

		ctx:    ctx,
		cancel: cancel,

		resultChan: resultChan,
		outboxChan: outboxChan,
		applyChan:  make(chan schema.Meta, 1000),
		ckpChan:    make(chan schema.Checkpoint, 100),
	}
}

func (v *Vmm) Run() {
	// mount core token & registry spawner
	v.Mount(schema.ModuleFormatToken, v.spawnToken)
	v.Mount(schema.ModuleFormatRegistry, v.spawnRegistry)

	go v.runChanHandler()
}

func (v *Vmm) Apply(m schema.Meta) {
	v.applyChan <- m
}

func (v *Vmm) Close() {
	log.Info("vmm is shutting down")

	v.cancel()
	v.wg.Wait()

	v.KillAll()

	log.Info("vmm has been shut down")
}
