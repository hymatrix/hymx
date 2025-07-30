package cache

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hymatrix/hymx/db/cache/schema"
	goarSchema "github.com/permadao/goar/schema"
)

type Outbox struct {
	mailbox map[string][]*goarSchema.BundleItem // sender pid -> messages
	targets map[string]map[string][]int         // sender pid -> target accid -> sequence

	rwlock sync.RWMutex
}

func NewOutbox() *Outbox {
	return &Outbox{
		mailbox: map[string][]*goarSchema.BundleItem{},
		targets: map[string]map[string][]int{},
	}
}

func (o *Outbox) Push(pid, target string, message goarSchema.BundleItem) error {
	o.rwlock.Lock()
	defer o.rwlock.Unlock()

	if o.mailbox[pid] == nil {
		o.mailbox[pid] = make([]*goarSchema.BundleItem, 0)
	}
	o.mailbox[pid] = append(o.mailbox[pid], &message)

	if o.targets[pid] == nil {
		o.targets[pid] = make(map[string][]int)
	}
	if o.targets[pid][target] == nil {
		o.targets[pid][target] = make([]int, 0)
	}
	o.targets[pid][target] = append(o.targets[pid][target], len(o.mailbox[pid])-1)

	return nil
}

func (o *Outbox) Peek(pid, target string) (seq int, item *goarSchema.BundleItem, err error) {
	o.rwlock.RLock()
	defer o.rwlock.RUnlock()

	pro, ok := o.targets[pid]
	if !ok {
		return 0, nil, fmt.Errorf("no target process found for %s/%s", pid, target)
	}
	sequences, ok := pro[target]
	if !ok {
		return 0, nil, fmt.Errorf("no target sequence found for %s/%s", pid, target)
	}

	if len(sequences) == 0 {
		return 0, nil, nil
	}
	seq = sequences[0]
	if seq < 0 || seq >= len(o.mailbox[pid]) {
		return 0, nil, fmt.Errorf("invalid sequence index %d", seq)
	}

	item = o.mailbox[pid][seq]
	return
}

func (o *Outbox) Commit(pid, target string, assign goarSchema.BundleItem) error {
	o.rwlock.Lock()
	defer o.rwlock.Unlock()

	pro, ok := o.targets[pid]
	if !ok {
		return fmt.Errorf("no pending process found for %s/%s", pid, target)
	}
	sequences, ok := pro[target]
	if !ok || len(sequences) == 0 {
		return fmt.Errorf("no pending target sequence for %s/%s", pid, target)
	}

	o.targets[pid][target] = sequences[1:] // Remove first element

	return nil
}

func (o *Outbox) Checkpoint(pid string) (string, error) {
	o.rwlock.RLock()
	defer o.rwlock.RUnlock()

	sp := schema.OutboxSnapshot{
		Id:      pid,
		Mailbox: o.mailbox[pid],
		Targets: o.targets[pid],
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

	o.mailbox[sp.Id] = sp.Mailbox
	o.targets[sp.Id] = sp.Targets

	return nil
}
