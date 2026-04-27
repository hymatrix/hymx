package vmm

import (
	"testing"

	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestGenSpawnResultDoesNotForwardEncryptedOriginXParams(t *testing.T) {
	v := &Vmm{}
	env := &schema.Env{
		Meta: schema.Meta{
			ItemId:      "item-id",
			Pid:         "process-id",
			FromProcess: "parent-process",
			Params: map[string]string{
				"X-Public": "public-value",
				"X-Secret": "private-value",
			},
			EncryptedParams: map[string]bool{"X-Secret": true},
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
	require.NotContains(t, tags, goarSchema.Tag{Name: "X-Secret", Value: "private-value"})
}

func TestGenSpawnResultDoesNotForwardEncryptedOriginReference(t *testing.T) {
	v := &Vmm{}
	env := &schema.Env{
		Meta: schema.Meta{
			ItemId:          "item-id",
			Pid:             "process-id",
			FromProcess:     "parent-process",
			Params:          map[string]string{"Reference": "private-reference"},
			EncryptedParams: map[string]bool{"Reference": true},
		},
		Process: hymxSchema.Process{
			Tags: []goarSchema.Tag{{Name: "Reference", Value: "private-reference"}},
		},
	}

	result := v.genSpawnResult(env)
	require.Len(t, result.Messages, 1)
	tags := result.Messages[0].Tags
	require.Equal(t, "0", tagValue(tags, "Reference"))
	require.NotContains(t, tags, goarSchema.Tag{Name: "Reference", Value: "private-reference"})
}

func tagValue(tags []goarSchema.Tag, name string) string {
	for _, tag := range tags {
		if tag.Name == name {
			return tag.Value
		}
	}
	return ""
}
