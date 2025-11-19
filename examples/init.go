package main

import (
	"fmt"

	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
	"github.com/permadao/goar/schema"
)

const (
	tokenModule    = "1i03Vpe8DljkUMBEEEvR0VmbJjvgZtP_ytZdThkVSMw"
	registryModule = "MVTil0kn5SRiJELW7W2jLZ6cBr3QUGj1nJ67I2Wi4Ps"
)

func initToken() (string, error) {
	res, err := s.SpawnAndWait(
		tokenModule,
		s.GetAddress(),
		[]schema.Tag{})
	fmt.Println(res, err)
	return res.Id, err
}

func initRegistry(tokenPid string, mainNode registrySchema.Node) {
	res, err := s.SpawnAndWait(
		registryModule,
		s.GetAddress(),
		[]schema.Tag{
			{Name: "Token-Pid", Value: tokenPid},
			{Name: "Name", Value: mainNode.Name},
			{Name: "Desc", Value: mainNode.Desc},
			{Name: "URL", Value: mainNode.URL},
		})
	fmt.Println(res, err)
}
