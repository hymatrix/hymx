package vmm

import (
	"github.com/hymatrix/hymx/utils/tagcrypto"
	"github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

func (v *Vmm) withDecryptedParamsFromTags(meta schema.Meta, tags []goarSchema.Tag) (schema.Meta, error) {
	decrypted, _, err := tagcrypto.DecryptParams(tags, v.tagDecryptKey)
	if err != nil {
		return meta, err
	}
	meta.DecryptedParams = decrypted
	return meta, nil
}

func (v *Vmm) withDecryptedParamsFromParamMap(meta schema.Meta) (schema.Meta, error) {
	decrypted, _, err := tagcrypto.DecryptParamMap(meta.Params, v.tagDecryptKey)
	if err != nil {
		return meta, err
	}
	meta.DecryptedParams = decrypted
	return meta, nil
}
