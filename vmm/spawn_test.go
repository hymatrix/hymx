package vmm

import (
	"testing"

	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestGenSpawnResultForwardsOnlyRawXParams(t *testing.T) {
	v := &Vmm{}
	env := &schema.Env{
		Meta: schema.Meta{
			ItemId:      "item-id",
			Pid:         "process-id",
			FromProcess: "parent-process",
			Params: map[string]string{
				"X-Public":           "public-value",
				"Encrypted-X-Secret": "ciphertext",
			},
			DecryptedParams: map[string]string{
				"Encrypted-X-Secret": "private-value",
			},
		},
		Process: hymxSchema.Process{
			Tags: []goarSchema.Tag{{Name: "Reference", Value: "7"}},
		},
	}

	result := v.genSpawnResult(env)
	require.Len(t, result.Messages, 1)
	tags := result.Messages[0].Tags
	require.Equal(t, "public-value", tagValue(tags, "X-Public"))
	require.Empty(t, tagValue(tags, "X-Secret"))
	require.Empty(t, tagValue(tags, "Encrypted-X-Secret"))
	require.False(t, hasTagValue(tags, "private-value"))
}

func TestGenSpawnResultUsesRawReferenceWhenEncryptedReferenceIsPresent(t *testing.T) {
	v := &Vmm{}
	env := &schema.Env{
		Meta: schema.Meta{
			ItemId:      "item-id",
			Pid:         "process-id",
			FromProcess: "parent-process",
			Params:      map[string]string{"Encrypted-Reference": "ciphertext"},
			DecryptedParams: map[string]string{
				"Encrypted-Reference": "private-reference",
			},
		},
		Process: hymxSchema.Process{
			Tags: []goarSchema.Tag{
				{Name: "Reference", Value: "7"},
				{Name: "Encrypted-Reference", Value: "ciphertext"},
			},
		},
	}

	result := v.genSpawnResult(env)
	require.Len(t, result.Messages, 1)
	tags := result.Messages[0].Tags
	require.Equal(t, "7", tagValue(tags, "Reference"))
	require.NotContains(t, tags, goarSchema.Tag{Name: "Reference", Value: "private-reference"})
}

func TestGenSpawnResultUsesDefaultReferenceWhenOnlyEncryptedReferenceIsPresent(t *testing.T) {
	v := &Vmm{}
	env := &schema.Env{
		Meta: schema.Meta{
			ItemId:      "item-id",
			Pid:         "process-id",
			FromProcess: "parent-process",
			Params:      map[string]string{"Encrypted-Reference": "ciphertext"},
			DecryptedParams: map[string]string{
				"Encrypted-Reference": "private-reference",
			},
		},
		Process: hymxSchema.Process{
			Tags: []goarSchema.Tag{{Name: "Encrypted-Reference", Value: "ciphertext"}},
		},
	}

	result := v.genSpawnResult(env)
	require.Len(t, result.Messages, 1)
	tags := result.Messages[0].Tags
	require.Equal(t, "0", tagValue(tags, "Reference"))
	require.False(t, hasTagValue(tags, "private-reference"))
}

func TestSpawnChecksExistingProcessBeforeDecryptingTags(t *testing.T) {
	v := New(nil, make(chan schema.VmmResult, 1), make(chan schema.Outbox, 1), make(chan struct{}), nil)
	v.addVm(testVm{}, &schema.Env{Meta: schema.Meta{Pid: "process-id"}})

	err := v.Spawn(
		schema.Meta{Pid: "process-id"},
		hymxSchema.Process{
			Tags: []goarSchema.Tag{{Name: "Encrypted-Secret", Value: "not-a-cipher"}},
		},
		hymxSchema.Module{},
	)

	require.ErrorIs(t, err, schema.ErrProcessAlreadyExists)
}

func TestSpawnDecryptFailureUnlocksRecovery(t *testing.T) {
	v := New(nil, make(chan schema.VmmResult, 1), make(chan schema.Outbox, 1), make(chan struct{}), nil)
	v.RecoveryLock("process-id")

	err := v.Spawn(
		schema.Meta{
			Pid:              "process-id",
			Mode:             schema.ExecModeReplay,
			Nonce:            0,
			RecoveryMaxNonce: 0,
		},
		hymxSchema.Process{
			Tags: []goarSchema.Tag{{Name: "Encrypted-Secret", Value: "not-a-cipher"}},
		},
		hymxSchema.Module{},
	)

	require.Error(t, err)
	require.False(t, v.IsRecovering("process-id"))
}

func tagValue(tags []goarSchema.Tag, name string) string {
	for _, tag := range tags {
		if tag.Name == name {
			return tag.Value
		}
	}
	return ""
}

func hasTagValue(tags []goarSchema.Tag, value string) bool {
	for _, tag := range tags {
		if tag.Value == value {
			return true
		}
	}
	return false
}
