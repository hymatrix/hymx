package sdk

import (
	"strings"

	vmmSchema "github.com/hymatrix/hymx/vmm/schema"
	goarSchema "github.com/permadao/goar/schema"
)

// EncryptTags encrypts tag values and prefixes tag names for VMM decryption.
func (s *SDK) EncryptTags(tags []goarSchema.Tag) (encryptedTags []goarSchema.Tag, err error) {
	if len(tags) == 0 {
		return nil, nil
	}
	encryptor, err := s.Client.GetCryptor()
	if err != nil {
		return nil, err
	}
	encryptedTags = make([]goarSchema.Tag, 0, len(tags))
	for _, tag := range tags {
		encryptedValue, err := encryptor.Encrypt(tag.Value)
		if err != nil {
			return nil, err
		}
		encryptedTags = append(encryptedTags, goarSchema.Tag{
			Name:  encryptedTagName(tag.Name),
			Value: encryptedValue,
		})
	}
	return encryptedTags, nil
}

func encryptedTagName(name string) string {
	if strings.HasPrefix(name, vmmSchema.EncryptedTagPrefix) {
		return name
	}
	return vmmSchema.EncryptedTagPrefix + name
}
