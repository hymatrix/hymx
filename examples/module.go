package main

import (
	"fmt"

	"github.com/hymatrix/hymx/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func genModule() {
	itemId, err := s.GenerateAndSaveModule([]byte{}, schema.Module{
		Base:         schema.DefaultBaseModule,
		ModuleFormat: vmmSchema.ModuleFormatRegistry,
	})
	if err != nil {
		fmt.Println("generate and save module failed, ", "err", err)
		return
	}
	fmt.Println("generate and save module success, ", "id", itemId)
}

func uploadModule(modFile, keyFile string) (txid string, err error) {
	return s.UploadModuleToArweave(modFile, keyFile)
}

func loadModule(itemId string) {
	module, err := s.Client.GetModule(itemId)
	if err != nil {
		fmt.Println("load module failed, ", "id", itemId, "err", err)
		return
	}
	fmt.Printf("load module success, id: %s, module: %+v\n", itemId, module)
}
