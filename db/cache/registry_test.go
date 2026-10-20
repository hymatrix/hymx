package cache

import (
	"testing"

	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryRestoredProcessUsesUpdatedNode(t *testing.T) {
	node := registrySchema.Node{AccId: "node", URL: "http://old"}
	db := NewRegistry("registry", "token", node)
	require.NoError(t, db.RegisterProcess(node.AccId, "process"))
	data, err := db.Checkpoint()
	require.NoError(t, err)

	restored := NewRegistry("registry", "token", node)
	require.NoError(t, restored.Restore(data))
	require.NoError(t, restored.Unregister(node.AccId))
	nodes, err := restored.GetNodesByProcess("process")
	require.NoError(t, err)
	assert.Empty(t, nodes)

	node.URL = "http://new"
	node.Name = "updated"
	require.NoError(t, restored.Register(node))
	nodes, err = restored.GetNodesByProcess("process")
	require.NoError(t, err)
	assert.Equal(t, []registrySchema.Node{node}, nodes)

	// Older snapshots can also contain stale node details in the process index.
	data, err = restored.Checkpoint()
	require.NoError(t, err)
	require.NoError(t, restored.Restore(data))
	nodes, err = restored.GetNodesByProcess("process")
	require.NoError(t, err)
	assert.Equal(t, []registrySchema.Node{node}, nodes)
}
