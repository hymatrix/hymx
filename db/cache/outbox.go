package cache

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hymatrix/hymx/db/cache/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type Outbox struct {
	targets map[string]map[string][]*goarSchema.BundleItem // sender pid -> target accid -> message queue

	rwlock sync.RWMutex
}

func NewOutbox() *Outbox {
	return &Outbox{
		targets: map[string]map[string][]*goarSchema.BundleItem{},
	}
}

func (o *Outbox) Push(pid, target string, message goarSchema.BundleItem) error {
	o.rwlock.Lock()
	defer o.rwlock.Unlock()

	if o.targets[pid] == nil {
		o.targets[pid] = make(map[string][]*goarSchema.BundleItem)
	}
	if o.targets[pid][target] == nil {
		o.targets[pid][target] = make([]*goarSchema.BundleItem, 0)
	}

	o.targets[pid][target] = append(o.targets[pid][target], &message)

	return nil
}

func (o *Outbox) Peek(pid, target string) (item *goarSchema.BundleItem, err error) {
func (o *Outbox) Peek(pid, target string) (item *goarSchema.BundleItem, err error) {
	o.rwlock.RLock()
	defer o.rwlock.RUnlock()

	pro, ok := o.targets[pid]
	if !ok {
		return nil, fmt.Errorf("no target process found for %s/%s", pid, target)
	}
	messages, ok := pro[target]
	if !ok {
		return nil, fmt.Errorf("no target sequence found for %s/%s", pid, target)
	}

	if len(messages) == 0 {
		return nil, nil
	}

	item = messages[0]
	return
}

func (o *Outbox) Commit(pid, target string, assign goarSchema.BundleItem) error {
	o.rwlock.Lock()
	defer o.rwlock.Unlock()

	pro, ok := o.targets[pid]
	if !ok {
		return fmt.Errorf("no pending process found for %s/%s", pid, target)
	}
	messages, ok := pro[target]
	if !ok || len(messages) == 0 {
		return fmt.Errorf("no pending target sequence for %s/%s", pid, target)
	}

	// Remove first message from queue - THIS FREES MEMORY
	o.targets[pid][target] = messages[1:]

	return nil
}

func (o *Outbox) Checkpoint(pid string) (string, error) {
	o.rwlock.RLock()
	defer o.rwlock.RUnlock()

	sp := schema.OutboxSnapshot{
		Id:      pid,
		Targets: o.targets[pid], // Now stores messages directly
	}

	by, err := json.Marshal(sp)
	if err != nil {
		return "", err
	}
	return string(by), nil
}

func (o *Outbox) Restore(data string) error {
	o.rwlock.Lock()
	defer o.rwlock.Unlock()

	sp := &schema.OutboxSnapshot{}
	if err := json.Unmarshal([]byte(data), sp); err != nil {
		return err
	}

	o.targets[sp.Id] = sp.Targets // Targets now contain messages directly

	return nil
}
