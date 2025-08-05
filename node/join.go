package node

import (
	"net/url"
	"path"

	tokenSchema "github.com/hymatrix/hymx/vmm/core/token/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (n *Node) runJoin() {
	if !n.info.JoinNetwork {
		return
	}

	u, err := url.Parse(n.info.Node.URL)
	if err != nil {
		panic(err)
	}
	u.Path = path.Join(u.Path, "info")
	if _, err := n.sdk.Client.Callback(u.String()); err != nil {
		log.Error("failed to join the network, current URL cannot respond to internet requests", "url", u, "err", err)
		panic(err)
	}

	info, err := n.sdk.Client.Info()
	if err != nil {
		log.Error("failed to join the network", "err", err)
		panic(err)
	}
	n.info.Token = info.Token
	n.info.Registry = info.Registry

	sAmt, err := n.sdk.Client.StakeOf(n.sdk.GetAddress())
	if err != nil {
		log.Error("failed to join the network", "err", err)
		panic(err)
	}
	if sAmt.Cmp(tokenSchema.StakeMinAmount) >= 0 {
		log.Info("already joined the network!")
		return
	}

	amt, err := n.sdk.Client.BalanceOf(n.sdk.GetAddress())
	if err != nil {
		log.Error("failed to join the network", "err", err)
		panic(err)
	}
	if amt.Cmp(tokenSchema.StakeMinAmount) < 0 {
		log.Error("join network failed, insufficient balance", "balance", amt, "min amount", tokenSchema.StakeMinAmount)
		panic("failed to join the network, insufficient balance")
	}

	// todo: handle error
	_, err = n.sdk.SendMessage(n.info.Token, "",
		[]goarSchema.Tag{
			{Name: "Action", Value: "Stake"},
			{Name: "Quantity", Value: tokenSchema.StakeMinAmount.String()},
			{Name: "Registry", Value: n.info.Registry},
			{Name: "Acc-Id", Value: n.sdk.GetAddress()},
			{Name: "Name", Value: n.info.Node.Name},
			{Name: "Desc", Value: n.info.Node.Desc},
			{Name: "URL", Value: n.info.Node.URL},
		},
	)
	if err != nil {
		log.Error("failed to join the network", "err", err)
		panic(err)
	}

	log.Info("join the network successfully!")
}

func (n *Node) leave() {
	if !n.info.JoinNetwork {
		return
	}

	// todo: handle error
	n.sdk.SendMessage(n.info.Token, "",
		[]goarSchema.Tag{
			{Name: "Action", Value: "Unstake"},
			{Name: "Quantity", Value: tokenSchema.StakeMinAmount.String()},
			{Name: "Registry", Value: n.info.Registry},
			{Name: "Acc-Id", Value: n.sdk.GetAddress()},
			{Name: "Name", Value: n.info.Node.Name},
			{Name: "Desc", Value: n.info.Node.Desc},
			{Name: "URL", Value: n.info.Node.URL},
		},
	)

	log.Info("left the network successfully!")
}
