package node

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	goarSchema "github.com/permadao/goar/schema"
)

func TestCacheModuleItemWritesBundleItemJSON(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	item := &goarSchema.BundleItem{
		Id:   "module-1",
		Data: "payload",
		Tags: []goarSchema.Tag{{Name: "Module-Format", Value: "hymx.vmdocker.v0.0.1"}},
	}
	if err := cacheModuleItem("module-1", item); err != nil {
		t.Fatalf("cacheModuleItem failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join("mod", "mod-module-1.json"))
	if err != nil {
		t.Fatalf("read cached module failed: %v", err)
	}

	var cached goarSchema.BundleItem
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("unmarshal cached module failed: %v", err)
	}
	if cached.Id != item.Id {
		t.Fatalf("unexpected cached item id: got %q want %q", cached.Id, item.Id)
	}
	if cached.Data != item.Data {
		t.Fatalf("unexpected cached item data: got %q want %q", cached.Data, item.Data)
	}
}

func TestResolveModuleFilePathFallsBackToLegacyPath(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	legacyPath := legacyModuleFilePath("module-legacy")
	if err := os.WriteFile(legacyPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write legacy module file failed: %v", err)
	}

	resolved, err := resolveModuleFilePath("module-legacy")
	if err != nil {
		t.Fatalf("resolveModuleFilePath failed: %v", err)
	}
	if resolved != legacyPath {
		t.Fatalf("unexpected resolved path: got %q want %q", resolved, legacyPath)
	}
}
