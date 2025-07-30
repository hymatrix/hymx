package utils

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	nodeSchema "github.com/hymatrix/hymx/node/schema"
	"github.com/hymatrix/hymx/schema"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

func Decode(item goarSchema.BundleItem) (pid, accid, fromProcess string, instance interface{}, err error) {
	switch item.SignatureType {
	case goarSchema.ArweaveSignType:
		accid, err = goarUtils.OwnerToAddress(item.Owner)
		if err != nil {
			return
		}
	case goarSchema.EthereumSignType:
		var pubBytes []byte
		pubBytes, err = goarUtils.Base64Decode(item.Owner)
		if err != nil {
			return
		}
		accid = common.BytesToAddress(crypto.Keccak256(pubBytes[1:])[12:]).String()
	default:
		err = nodeSchema.ErrUnrecognizedSignatureType
		return
	}

	switch GetType(item) {
	case schema.TypeProcess:
		pid = item.Id
		var proc schema.Process
		proc, err = DecodeItemToProcess(item)
		if err != nil {
			return
		}

		fromProcess = proc.FromProcess
		instance = proc
	case schema.TypeMessage:
		pid = item.Target
		var msg schema.Message
		msg, err = DecodeItemToMessage(item)
		if err != nil {
			return
		}

		fromProcess = msg.FromProcess
		instance = msg
	case schema.TypeAssignment:
		var assign schema.Assignment
		assign, err = DecodeItemToAssignment(item)
		if err != nil {
			return
		}

		pid = assign.Process
	default:
		err = nodeSchema.ErrInvalidType
	}

	return
}

func DecodeItemToProcess(item goarSchema.BundleItem) (proc schema.Process, err error) {
	proc, err = TagsToProcess(item.Tags)
	return
}

func DecodeItemToMessage(item goarSchema.BundleItem) (msg schema.Message, err error) {
	msg, err = TagsToMessage(item.Tags)
	if err != nil {
		return
	}

	// decode data
	data, err := goarUtils.Base64Decode(item.Data)
	if err != nil {
		return
	}
	msg.Tags = append(msg.Tags, goarSchema.Tag{Name: "Data", Value: string(data)})

	return
}

func DecodeItemToAssignment(item goarSchema.BundleItem) (assign schema.Assignment, err error) {
	assign, err = TagsToAssignment(item.Tags)
	return
}

func GetType(item goarSchema.BundleItem) string {
	return GetTagsValue("Type", item.Tags)
}
