package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

const testModuleTags = `[{"name":"Data-Protocol","value":"hymx"},{"name":"Variant","value":"v0.1.0"},{"name":"Type","value":"Module"},{"name":"Module-Format","value":"test.runtime"},{"name":"Image-ID","value":"sha256:test"}]`

func writeTestModule(t *testing.T, id string, payloadFirst bool, payloadMiB int) {
	t.Helper()
	if err := os.MkdirAll("mod", 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join("mod", "mod-"+id+".json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	prefix := `{"tags":` + testModuleTags + `,"data":"`
	if payloadFirst {
		prefix = `{"data":"`
	}
	if _, err := f.WriteString(prefix); err != nil {
		t.Fatal(err)
	}
	chunk := strings.Repeat("a", 1<<20)
	for i := 0; i < payloadMiB; i++ {
		if _, err := f.WriteString(chunk); err != nil {
			t.Fatal(err)
		}
	}
	suffix := `"}`
	if payloadFirst {
		suffix = `","tags":` + testModuleTags + `}`
	}
	if _, err := f.WriteString(suffix); err != nil {
		t.Fatal(err)
	}
}

func TestLoadModuleMetadataBoundedMemory(t *testing.T) {
	for _, payloadFirst := range []bool{false, true} {
		name := "tags-first"
		if payloadFirst {
			name = "payload-first"
		}
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeTestModule(t, "large", payloadFirst, 32)
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			module, err := (&Node{}).loadModuleByLocal("large")
			runtime.ReadMemStats(&after)
			if err != nil {
				t.Fatal(err)
			}
			if module.ModuleFormat != "test.runtime" {
				t.Fatalf("unexpected module: %+v", module)
			}
			allocated := after.TotalAlloc - before.TotalAlloc
			t.Logf("32 MiB module payload: allocated %.2f MiB", float64(allocated)/(1<<20))
			if allocated > 8<<20 {
				t.Fatalf("metadata load allocated %d bytes for an unused payload", allocated)
			}
		})
	}
}

func TestLoadModuleCachesIndependentMetadata(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestModule(t, "cached", false, 0)
	n := &Node{}
	module, err := n.LoadModule("cached")
	if err != nil {
		t.Fatal(err)
	}
	module.Tags[0].Value = "changed-by-caller"
	if err := os.Remove(filepath.Join("mod", "mod-cached.json")); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := n.LoadModule("cached")
			if err != nil {
				t.Error(err)
				return
			}
			encoded, _ := json.Marshal(got.Tags)
			if strings.Contains(string(encoded), "changed-by-caller") {
				t.Error("caller mutated shared cached metadata")
			}
			got.Tags[0].Value = "another-caller"
		}()
	}
	wg.Wait()
}
