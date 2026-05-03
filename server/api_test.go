package server

import (
	"testing"

	"github.com/everFinance/goether"
	hymxSchema "github.com/hymatrix/hymx/schema"
	"github.com/hymatrix/hymx/utils"
	"github.com/permadao/goar"
	"github.com/stretchr/testify/require"
)

func TestRedirectPIDForMessageUsesTargetProcess(t *testing.T) {
	signer, err := goether.NewSigner("0xdde30fa25128addf45656a39c0570fd06fce3e48056457b9f1f9fda603cc4be1")
	require.NoError(t, err)
	bundler, err := goar.NewBundler(signer)
	require.NoError(t, err)
	targetPid := "lM-6SkQOII31LeDUeNTmCoXCLBBNLllkPEDMVosFrJY"
	tags, err := utils.MessageToTags(hymxSchema.Message{Base: hymxSchema.DefaultBaseMessage})
	require.NoError(t, err)
	item, err := bundler.CreateAndSignItem([]byte("payload"), targetPid, "", tags)
	require.NoError(t, err)

	pid, err := redirectPIDForItem(item)
	require.NoError(t, err)
	require.Equal(t, targetPid, pid)
	require.NotEqual(t, item.Id, pid)
}
