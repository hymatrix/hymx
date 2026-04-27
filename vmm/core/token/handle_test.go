package token

import (
	"math/big"
	"testing"

	"github.com/hymatrix/hymx/db/cache"
	tokenSchema "github.com/hymatrix/hymx/vmm/core/token/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
	"github.com/stretchr/testify/require"
)

func TestTransferDoesNotForwardEncryptedOriginXParams(t *testing.T) {
	from := "0x1111111111111111111111111111111111111111"
	recipient := "0x2222222222222222222222222222222222222222"
	db := cache.NewToken(tokenSchema.Info{Ticker: "TST"}, map[string]*big.Int{
		from: big.NewInt(100),
	}, map[string]*big.Int{})
	tok, err := New(db)
	require.NoError(t, err)

	res := tok.Apply(from, vmmSchema.Meta{
		Action: "Transfer",
		Params: map[string]string{
			"Recipient": recipient,
			"Quantity":  "1",
			"X-Public":  "public-value",
			"X-Secret":  "private-value",
		},
		EncryptedParams: map[string]bool{"X-Secret": true},
	})

	require.NoError(t, res.Error)
	require.Len(t, res.Messages, 2)
	for _, msg := range res.Messages {
		require.Equal(t, "public-value", tokenTagValue(msg.Tags, "X-Public"))
		require.Empty(t, tokenTagValue(msg.Tags, "X-Secret"))
		require.NotContains(t, msg.Tags, goarSchema.Tag{Name: "X-Secret", Value: "private-value"})
	}
}

func tokenTagValue(tags []goarSchema.Tag, name string) string {
	for _, tag := range tags {
		if tag.Name == name {
			return tag.Value
		}
	}
	return ""
}
