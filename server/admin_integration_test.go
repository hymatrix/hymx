package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hymatrix/hymx/node"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	hySchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/sdk"
	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AdminIntegrationTestSuite struct {
	suite.Suite
}

// adminIntegrationVM is a non-core VM used to exercise the real node lifecycle.
type adminIntegrationVM struct {
	value string
	fail  bool
}

func (vm *adminIntegrationVM) Apply(from string, meta vmmSchema.Meta) vmmSchema.Result {
	switch meta.Action {
	case "Set":
		data, err := goarUtils.Base64Decode(meta.Data)
		if err != nil {
			return vmmSchema.Result{Error: err}
		}
		vm.value = string(data)
	case "FailCheckpoint":
		vm.fail = true
	}
	return vmmSchema.Result{Data: vm.value}
}

func (vm *adminIntegrationVM) Checkpoint() (string, error) {
	if vm.fail {
		return "", errors.New("integration checkpoint failure")
	}
	return vm.value, nil
}

func (vm *adminIntegrationVM) Restore(data string) error { vm.value = data; return nil }
func (vm *adminIntegrationVM) Close() error              { return nil }

func (suite *AdminIntegrationTestSuite) TestStopAndCheckpointOverHTTP() {
	t := suite.T()
	redisPath, err := exec.LookPath("redis-server")
	require.NoError(t, err)
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	work := t.TempDir()
	oldWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(work))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWd)) })
	endpoint := func() string {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		address := listener.Addr().String()
		require.NoError(t, listener.Close())
		return address
	}
	redisAddr, apiAddr, adminAddr := endpoint(), endpoint(), endpoint()
	_, redisPort, err := net.SplitHostPort(redisAddr)
	require.NoError(t, err)
	redisLog, err := os.Create(filepath.Join(work, "redis.log"))
	require.NoError(t, err)
	redis := exec.Command(redisPath, "--bind", "127.0.0.1", "--port", redisPort, "--save", "", "--appendonly", "no", "--dir", work)
	redis.Stdout, redis.Stderr = redisLog, redisLog
	require.NoError(t, redis.Start())
	t.Cleanup(func() { _ = redis.Process.Kill(); _ = redis.Wait(); _ = redisLog.Close() })
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", redisAddr, time.Second)
		if err != nil {
			return false
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		_, _ = conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))
		buf := make([]byte, 64)
		n, _ := conn.Read(buf)
		return string(buf[:n]) == "+PONG\r\n"
	}, 10*time.Second, 50*time.Millisecond)

	base, admin := "http://"+apiAddr, "http://"+adminAddr
	client := sdk.New(base, filepath.Join(root, "cmd/test_keyfile.json"))
	t.Cleanup(client.Close)
	require.NoError(t, os.Mkdir("mod", 0755))
	files, err := filepath.Glob(filepath.Join(root, "cmd/mod/*.json"))
	require.NoError(t, err)
	for _, file := range files {
		data, err := os.ReadFile(file)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join("mod", filepath.Base(file)), data, 0600))
	}
	n := node.New(nil, client.Bundler, "redis://"+redisAddr+"/0", base, base,
		&nodeSchema.Info{Node: registrySchema.Node{AccId: client.GetAddress(), URL: base}}, nil)
	s := New(n, nil)
	require.NoError(t, s.Mount("integration.vm", func(env vmmSchema.Env) (vmmSchema.Vm, error) { return &adminIntegrationVM{}, nil }))
	s.Run(apiAddr, adminAddr, nodeSchema.StartModeNormal)
	t.Cleanup(s.Close)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	t.Cleanup(httpClient.CloseIdleConnections)
	require.Eventually(t, func() bool {
		res, err := httpClient.Get(admin + "/admin/vms/running")
		if err != nil {
			return false
		}
		defer res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 10*time.Second, 50*time.Millisecond)
	token, err := client.SpawnAndWait("1i03Vpe8DljkUMBEEEvR0VmbJjvgZtP_ytZdThkVSMw", client.GetAddress(), nil)
	require.NoError(t, err)
	registry, err := client.SpawnAndWait("MVTil0kn5SRiJELW7W2jLZ6cBr3QUGj1nJ67I2Wi4Ps", client.GetAddress(), []goarSchema.Tag{{Name: "Token-Pid", Value: token.Id}, {Name: "URL", Value: base}})
	require.NoError(t, err)
	mid, err := client.SaveModule(nil, hySchema.Module{Base: hySchema.DefaultBaseModule, ModuleFormat: "integration.vm"})
	require.NoError(t, err)
	require.NoError(t, os.Rename("mod-"+mid+".json", filepath.Join("mod", "mod-"+mid+".json")))
	vm, err := client.SpawnAndWait(mid, client.GetAddress(), nil)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		pids, err := n.GetProcesses(client.GetAddress())
		if err != nil {
			return false
		}
		for _, pid := range pids {
			if pid == vm.Id {
				return true
			}
		}
		return false
	}, 20*time.Second, 50*time.Millisecond)
	send := func(action, data string) {
		response, err := client.SendMessageAndWait(vm.Id, data, []goarSchema.Tag{{Name: "Action", Value: action}})
		require.NoError(t, err)
		var result vmmSchema.VmmResult
		require.NoError(t, json.Unmarshal([]byte(response.Message), &result))
		require.Empty(t, result.Error)
		require.Equal(t, "state-42", result.Data)
	}
	post := func(action, pid string, status int) string {
		body, err := json.Marshal(map[string]string{"pid": pid})
		require.NoError(t, err)
		res, err := httpClient.Post(admin+"/admin/vms/"+action, "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, status, res.StatusCode, string(data))
		t.Logf("POST %s: %d %s", action, status, data)
		return string(data)
	}
	checkpoints := func() []string { files, err := filepath.Glob("ckp/ckp-*.json"); require.NoError(t, err); return files }
	send("Set", "state-42")
	// Exercise all binary routes against persisted JSON records and the real SDK.
	spawn, err := client.Client.GetMessageBin(vm.Id)
	require.NoError(t, err)
	assert.Equal(t, vm.Id, spawn.Id)
	message, err := client.Client.GetMessageBinByNonce(vm.Id, 1)
	require.NoError(t, err)
	assignment, err := client.Client.GetAssignBinByNonce(vm.Id, 1)
	require.NoError(t, err)
	byMessage, err := client.Client.GetAssignBinByMessage(message.Id)
	require.NoError(t, err)
	assert.Equal(t, assignment.Id, byMessage.Id)
	legacy, err := client.Client.GetMessage(message.Id)
	require.NoError(t, err)
	assert.Equal(t, message.Tags, legacy.Tags)
	// A second isolated database rebuilds Registry, Token and the VM over binary HTTP.
	follower := node.New(nil, client.Bundler, "redis://"+redisAddr+"/1", base, base,
		&nodeSchema.Info{Node: registrySchema.Node{AccId: client.GetAddress()}, Registry: registry.Id, Token: token.Id}, nil)
	require.NoError(t, follower.Mount("integration.vm", func(env vmmSchema.Env) (vmmSchema.Vm, error) { return &adminIntegrationVM{}, nil }))
	follower.Run(nodeSchema.StartModeNormal)
	t.Cleanup(follower.Close)
	require.NoError(t, follower.Fork(vm.Id, "", base, vmmSchema.ExecModeDryRun))
	require.Eventually(t, func() bool {
		item, err := follower.Checkpoint(vm.Id)
		if err != nil {
			return false
		}
		data, err := goarUtils.Base64Decode(item.Data)
		if err != nil {
			return false
		}
		var snap vmmSchema.Snapshot
		return json.Unmarshal(data, &snap) == nil && snap.Env.Nonce == 1 && snap.Data == "state-42"
	}, 30*time.Second, 50*time.Millisecond)
	for _, path := range []string{"/messageBin/missing", "/messageBinByNonce/" + vm.Id + "/9999", "/assignmentBinByNonce/" + vm.Id + "/9999", "/assignmentBinByMessage/missing"} {
		res, err := httpClient.Get(base + path)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, res.StatusCode)
		_ = res.Body.Close()
	}
	post("stop", vm.Id, http.StatusOK)
	assert.NotContains(t, n.Running(), vm.Id)
	require.Empty(t, checkpoints())
	post("resume", vm.Id, http.StatusOK)
	send("Read", "")
	t.Log("default stop: no checkpoint; history recovery preserved state-42")
	post("stopWithCheckpoint", vm.Id, http.StatusOK)
	assert.NotContains(t, n.Running(), vm.Id)
	require.Len(t, checkpoints(), 1)
	post("resume", vm.Id, http.StatusOK)
	send("Read", "")
	t.Log("checkpoint stop: one checkpoint; recovery preserved state-42")
	send("FailCheckpoint", "")
	assert.Contains(t, post("stopWithCheckpoint", vm.Id, http.StatusBadRequest), "integration checkpoint failure")
	assert.Contains(t, n.Running(), vm.Id)
	send("Read", "")
	post("stop", vm.Id, http.StatusOK)
	require.Len(t, checkpoints(), 1)
	assert.Contains(t, post("stopWithCheckpoint", token.Id, http.StatusBadRequest), "err_core_process_cannot_stop")
	t.Log(fmt.Sprintf("verified non-core VM %s and core protection", vm.Id))
}

func TestAdminIntegrationTestSuite(t *testing.T) {
	if os.Getenv("HYMX_INTEGRATION") != "1" {
		t.Skip("set HYMX_INTEGRATION=1 to run with isolated local Redis")
	}
	suite.Run(t, new(AdminIntegrationTestSuite))
}
