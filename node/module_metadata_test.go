package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testModuleTags = `[{"name":"Data-Protocol","value":"hymx"},{"name":"Variant","value":"v0.1.0"},{"name":"Type","value":"Module"},{"name":"Module-Format","value":"test.runtime"}]`

func TestReadModuleTags(t *testing.T) {
	var expected []goarSchema.Tag
	require.NoError(t, json.Unmarshal([]byte(testModuleTags), &expected))
	for _, prefix := range []string{
		`{`,
		`{"signatureType":3,"owner":"test",`,
		`{"data":"escaped\\\"value\u0041",`,
		`{"extra":{"tags":[{"name":"wrong"}],"values":[true,false,null,-1.25e2,{}]},`,
	} {
		t.Run(prefix, func(t *testing.T) {
			tags, err := readModuleTags(strings.NewReader(prefix + `"t\u0061gs":` + testModuleTags + `,"data":"unused"}`))
			require.NoError(t, err)
			assert.Equal(t, expected, tags)
		})
	}
	// Metadata extraction stops after tags; it does not validate the payload.
	tags, err := readModuleTags(strings.NewReader(`{"TAGS":` + testModuleTags + `,"data":"`))
	require.NoError(t, err)
	assert.Equal(t, expected, tags)
}

func TestReadModuleTagsRejectsInvalidHeaders(t *testing.T) {
	for _, input := range []string{
		`[]`, `{}`, `{"data":"unterminated`, `{"data":"bad\q","tags":[]}`,
		`{"data":"bad\u0X00","tags":[]}`, `{"data":wat,"tags":[]}`,
		`{"extra":[1,],"tags":[]}`, `{"extra":{"a" 1},"tags":[]}`,
		`{"tags":"wrong-type"}`, `{"extra":` + strings.Repeat("[", 130) + `0`,
		`{"tags":[{"name":"` + strings.Repeat("x", 1<<20) + `"}]}`,
		`{"` + strings.Repeat("x", 65<<10) + `":0,"tags":[]}`,
	} {
		t.Run(input[:min(len(input), 60)], func(t *testing.T) {
			_, err := readModuleTags(strings.NewReader(input))
			assert.Error(t, err)
		})
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
			require.NoError(t, os.Mkdir("mod", 0755))
			file, err := os.Create(filepath.Join("mod", "mod-large.json"))
			require.NoError(t, err)
			t.Cleanup(func() { file.Close() })
			prefix := `{"tags":` + testModuleTags + `,"data":"`
			suffix := `"}`
			if payloadFirst {
				prefix = `{"data":"`
				suffix = `","tags":` + testModuleTags + `}`
			}
			_, err = file.WriteString(prefix)
			require.NoError(t, err)
			chunk := strings.Repeat("a", 1<<20)
			for i := 0; i < 32; i++ {
				_, err = file.WriteString(chunk)
				require.NoError(t, err)
			}
			_, err = file.WriteString(suffix)
			require.NoError(t, err)
			require.NoError(t, file.Close())

			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			module, err := (&Node{}).loadModuleByLocal("large")
			runtime.ReadMemStats(&after)
			require.NoError(t, err)
			assert.Equal(t, "test.runtime", module.ModuleFormat)
			allocated := after.TotalAlloc - before.TotalAlloc
			t.Logf("32 MiB payload: allocated %.2f MiB", float64(allocated)/(1<<20))
			assert.Less(t, allocated, uint64(8<<20), "unused payload must not be materialized")
		})
	}
}
