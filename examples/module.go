package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hymatrix/hymx/schema"
	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
)

func module() {
	// item, _ := s.GenerateModule([]byte{}, schema.Module{
	// 	Base:         schema.DefaultBaseModule,
	// 	ModuleFormat: "org.type.1.0.0",
	// })
	item, _ := s.GenerateModule([]byte{}, schema.Module{
		Base:         schema.DefaultBaseModule,
		ModuleFormat: vmmSchema.ModuleFormatRegistry,
	})
	// item, _ := s.GenerateModule([]byte{}, schema.Module{
	// 	Base:         schema.DefaultBaseModule,
	// 	ModuleFormat: vmmSchema.ModuleFormatToken,
	// })
	bin, _ := json.Marshal(item)

	filename := fmt.Sprintf("mod-%s.json", item.Id)
	os.WriteFile(filename, bin, 0644)
}
