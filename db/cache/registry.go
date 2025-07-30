package cache

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hymatrix/hymx/db/cache/schema"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
)

type Registry struct {
	id       string
	tokenPid string

	// index
	mainIndex string // mainNode accid
	// todo: candidateIndex map[string]struct{}
	processToNodeIndex    map[string]map[string]*registrySchema.Node // pid -> map[accid]Node
	accidToProcessesIndex map[string]map[string]string               // accid -> map[pid]pid
	registered            map[string]bool

	// all nodes
	nodes map[string]*registrySchema.Node

	rwlock sync.RWMutex
}

func NewRegistry(id, tokenPid string, mainNode registrySchema.Node) *Registry {
	return &Registry{
		id:       id,
		tokenPid: tokenPid,

		mainIndex:          mainNode.AccId,
		processToNodeIndex: map[string]map[string]*registrySchema.Node{},
		accidToProcessesIndex: map[string]map[string]string{
			mainNode.AccId: map[string]string{},
		},
		registered: map[string]bool{mainNode.AccId: true},

		nodes: map[string]*registrySchema.Node{
			mainNode.AccId: &mainNode,
		},
	}
}

func (r *Registry) GetId() (string, error) {
	r.rwlock.RLock()
	defer r.rwlock.RUnlock()

	id := r.id
	return id, nil
}

func (r *Registry) GetTokenPid() (string, error) {
	r.rwlock.RLock()
	defer r.rwlock.RUnlock()

	pid := r.tokenPid
	return pid, nil
}

func (r *Registry) Register(node registrySchema.Node) error {
	r.rwlock.Lock()
	defer r.rwlock.Unlock()

	if _, ok := r.registered[node.AccId]; ok {
		r.nodes[node.AccId] = &node
		r.registered[node.AccId] = true
		return nil
	}

	// init index
	r.accidToProcessesIndex[node.AccId] = map[string]string{}
	switch node.Role {
	case registrySchema.RoleMain:
		r.mainIndex = node.AccId
	case registrySchema.RoleCandidate:
	}

	r.nodes[node.AccId] = &node
	r.registered[node.AccId] = true
	return nil
}

func (r *Registry) Unregister(accid string) error {
	r.rwlock.Lock()
	defer r.rwlock.Unlock()

	_, ok := r.nodes[accid]
	if !ok {
		return nil
	}

	r.registered[accid] = false
	return nil
}

func (r *Registry) GetNode(accid string) (*registrySchema.Node, error) {
	r.rwlock.RLock()
	defer r.rwlock.RUnlock()

	if !r.registered[accid] {
		return nil, nil
	}

	node, ok := r.nodes[accid]
	if !ok {
		return nil, nil
	}

	n := *node
	return &n, nil
}

func (r *Registry) GetNodes() (nodes map[string]registrySchema.Node, err error) {
	r.rwlock.RLock()
	defer r.rwlock.RUnlock()

	nodes = make(map[string]registrySchema.Node)
	for k, v := range r.nodes {
		if r.registered[k] {
			nodes[k] = *v
		}
	}
	return
}

func (r *Registry) RegisterProcess(accid, pid string) error {
	r.rwlock.Lock()
	defer r.rwlock.Unlock()

	node, ok := r.nodes[accid]
	if !ok {
		return fmt.Errorf("registry db, RegisterProcess failed: not found node, accid=%v", accid)
	}

	if _, ok := r.accidToProcessesIndex[accid]; !ok {
		return fmt.Errorf("registry db, RegisterProcess failed: not found node index, accid=%v", accid)
	}
	if r.accidToProcessesIndex[accid] == nil {
		r.accidToProcessesIndex[accid] = map[string]string{}
	}
	r.accidToProcessesIndex[accid][pid] = pid

	if _, ok := r.processToNodeIndex[pid]; !ok {
		r.processToNodeIndex[pid] = map[string]*registrySchema.Node{
			node.AccId: node,
		}
		return nil
	}
	r.processToNodeIndex[pid][accid] = node

	return nil
}

func (r *Registry) UnregisterProcess(accid, pid string) error {
	r.rwlock.Lock()
	defer r.rwlock.Unlock()

	if _, ok := r.accidToProcessesIndex[accid]; !ok {
		return fmt.Errorf("registry db, UnregisterProcess failed: not found node, accid=%v", accid)
	}
	if r.accidToProcessesIndex[accid] == nil {
		return nil
	}
	delete(r.accidToProcessesIndex[accid], pid)

	if _, ok := r.processToNodeIndex[pid]; !ok {
		return nil
	}
	delete(r.processToNodeIndex[pid], accid)

	return nil
}

func (r *Registry) GetProcesses(accid string) (procs []string, err error) {
	r.rwlock.RLock()
	defer r.rwlock.RUnlock()

	processes, ok := r.accidToProcessesIndex[accid]
	if !ok || len(processes) == 0 {
		return
	}

	procs = make([]string, 0, len(processes))
	for _, pid := range processes {
		procs = append(procs, pid)
	}

	return
}

func (r *Registry) GetNodesByProcess(pid string) (nodes []registrySchema.Node, err error) {
	r.rwlock.RLock()
	defer r.rwlock.RUnlock()

	nodesMap, ok := r.processToNodeIndex[pid]
	if !ok || len(nodesMap) == 0 {
		return
	}

	nodes = []registrySchema.Node{}
	for _, n := range nodesMap {
		if r.registered[n.AccId] {
			nodes = append(nodes, *n)
		}
	}

	return
}

func (r *Registry) Checkpoint() (data string, err error) {
	r.rwlock.Lock()
	defer r.rwlock.Unlock()

	sp := schema.RegistrySnapshot{
		Id:                    r.id,
		TokenPid:              r.tokenPid,
		MainIndex:             r.mainIndex,
		ProcessToNodeIndex:    r.processToNodeIndex,
		AccidToProcessesIndex: r.accidToProcessesIndex,
		Nodes:                 r.nodes,
	}

	by, err := json.Marshal(sp)
	if err != nil {
		return
	}
	data = string(by)

	return
}

func (r *Registry) Restore(data string) error {
	r.rwlock.Lock()
	defer r.rwlock.Unlock()

	sp := &schema.RegistrySnapshot{}
	if err := json.Unmarshal([]byte(data), sp); err != nil {
		return err
	}

	r.id = sp.Id
	r.tokenPid = sp.TokenPid
	r.mainIndex = sp.MainIndex
	r.processToNodeIndex = sp.ProcessToNodeIndex
	r.accidToProcessesIndex = sp.AccidToProcessesIndex
	r.nodes = sp.Nodes

	return nil
}
