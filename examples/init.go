package main

import (
	"fmt"

	"github.com/permadao/goar/schema"
)

const (
	tokenModule    = "1i03Vpe8DljkUMBEEEvR0VmbJjvgZtP_ytZdThkVSMw"
	registryModule = "MVTil0kn5SRiJELW7W2jLZ6cBr3QUGj1nJ67I2Wi4Ps"
)

func initToken() string {
	res, err := s.SpawnAndWait(
		tokenModule,
		s.GetAddress(),
		[]schema.Tag{})
	fmt.Println(res, err)
	return res.Id
}

func initRegistry(tokenPid string) {
	res, err := s.SpawnAndWait(
		registryModule,
		s.GetAddress(),
		[]schema.Tag{
			{Name: "Token-Pid", Value: tokenPid},
		})
	fmt.Println(res, err)
}
