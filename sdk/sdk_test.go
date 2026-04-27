package sdk

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/schema"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndSaveModule_CreatesFile(t *testing.T) {
	signer, err := goether.NewSigner("0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	s := NewFromBundler("http://127.0.0.1:8080", bundler)

	tmp := t.TempDir()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	defer os.Chdir(cwd)

	module := schema.Module{
		Base:         schema.DefaultBaseModule,
		ModuleFormat: "hymx.test.mod",
	}

	id, err := s.SaveModule([]byte("module-bytes"), module)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	filename := fmt.Sprintf("mod-%s.json", id)
	by, err := os.ReadFile(filename)
	require.NoError(t, err)

	var item goarSchema.BundleItem
	require.NoError(t, json.Unmarshal(by, &item))
	assert.Equal(t, id, item.Id)
}

func TestUploadModule_Success(t *testing.T) {
	modFile := "./mod/mod-lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY.json"
	keyFile := "./arweave-keyfile-QXZ7A1acq-E65smWygrDqibEyKOMS-73F2e7kf6PqLc.json"
	if _, err := os.Stat(modFile); os.IsNotExist(err) {
		t.Skipf("module fixture missing: %s", modFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Skipf("key fixture missing: %s", keyFile)
	}

	s := &SDK{}
	txid, err := s.UploadModuleToArweave(modFile, keyFile)
	require.NoError(t, err)
	assert.NotEmpty(t, txid)
	fmt.Println("txid:", txid)
}
