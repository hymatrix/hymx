package vmm

import (
	"errors"
	"testing"
	"time"

	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/vmm/schema"
)

func TestRecoverySpawnDoesNotWaitForMissingRegistry(t *testing.T) {
	for _, mode := range []schema.ExecMode{schema.ExecModeDryRun, schema.ExecModeReplay} {
		t.Run(string(mode), func(t *testing.T) {
			v := New(nil, nil, nil, nil, make(chan struct{}))
			t.Cleanup(v.Close)
			done := make(chan error, 1)
			go func() {
				done <- v.Spawn(schema.Meta{Pid: "dependent", Mode: mode}, hymxSchema.Process{}, hymxSchema.Module{ModuleFormat: "test.runtime"})
			}()
			select {
			case err := <-done:
				if !errors.Is(err, schema.ErrRegistryNotFound) {
					t.Fatalf("expected missing registry, got %v", err)
				}
			case <-time.After(200 * time.Millisecond):
				t.Fatal("recovery worker blocked waiting for an unavailable registry")
			}
		})
	}
}
