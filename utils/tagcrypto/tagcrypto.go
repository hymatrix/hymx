package tagcrypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/everFinance/goether"
	goar "github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
	goarUtils "github.com/permadao/goar/utils"
)

const (
	EncryptedTagPrefix    = "Encrypted-"
	CipherValuePrefix     = "hymxenc:v1"
	KeyTypeEthereumECIES  = "ethereum-ecies"
	KeyTypeArweaveRSAOAEP = "arweave-rsa-oaep-sha256"
)

var reservedTagNames = map[string]struct{}{
	"Action":          {},
	"Anchor":          {},
	"Compute-Limit":   {},
	"Data":            {},
	"Data-Protocol":   {},
	"From":            {},
	"From-Module":     {},
	"From-Process":    {},
	"Input-Encoding":  {},
	"Memory-Limit":    {},
	"Message":         {},
	"Module":          {},
	"Module-Format":   {},
	"Nonce":           {},
	"Output-Encoding": {},
	"Owner":           {},
	"Process":         {},
	"Pushed-For":      {},
	"PushedFor":       {},
	"Read-Only":       {},
	"Scheduler":       {},
	"Sequence":        {},
	"Tags":            {},
	"Target":          {},
	"Timestamp":       {},
	"Token":           {},
	"Type":            {},
	"Variant":         {},
}

func KeyTypeFromSignatureType(signType int) (string, error) {
	switch signType {
	case goarSchema.EthereumSignType:
		return KeyTypeEthereumECIES, nil
	case goarSchema.ArweaveSignType:
		return KeyTypeArweaveRSAOAEP, nil
	default:
		return "", fmt.Errorf("unsupported signature type: %d", signType)
	}
}

func HasEncryptedTags(tags []goarSchema.Tag) bool {
	for _, tag := range tags {
		if IsEncryptedTagName(tag.Name) {
			return true
		}
	}
	return false
}

func ValidateEncryptedTagNames(tags []goarSchema.Tag) error {
	for _, tag := range tags {
		if !IsEncryptedTagName(tag.Name) {
			continue
		}
		if _, err := PlainTagName(tag.Name); err != nil {
			return err
		}
	}
	return nil
}

func EncryptedPlainTagNames(tags []goarSchema.Tag) (map[string]bool, error) {
	names := map[string]bool{}
	for _, tag := range tags {
		if !IsEncryptedTagName(tag.Name) {
			continue
		}
		plainName, err := PlainTagName(tag.Name)
		if err != nil {
			return nil, err
		}
		names[plainName] = true
	}
	return names, nil
}

func IsEncryptedTagName(name string) bool {
	return strings.HasPrefix(name, EncryptedTagPrefix)
}

func PlainTagName(name string) (string, error) {
	plainName := strings.TrimPrefix(name, EncryptedTagPrefix)
	if plainName == "" || plainName == name {
		return "", errors.New("invalid encrypted tag name")
	}
	if IsReservedTagName(plainName) {
		return "", fmt.Errorf("encrypted tag uses reserved name: %s", plainName)
	}
	return plainName, nil
}

func IsReservedTagName(name string) bool {
	if _, ok := reservedTagNames[name]; ok {
		return true
	}
	return false
}

func EncryptTags(tags []goarSchema.Tag, publicKey, keyType string) ([]goarSchema.Tag, bool, error) {
	encrypted := make([]goarSchema.Tag, 0, len(tags))
	changed := false
	for _, tag := range tags {
		if !IsEncryptedTagName(tag.Name) {
			encrypted = append(encrypted, tag)
			continue
		}
		if _, err := PlainTagName(tag.Name); err != nil {
			return nil, false, err
		}
		ciphertext, err := encryptValue([]byte(tag.Value), publicKey, keyType)
		if err != nil {
			return nil, false, err
		}
		encrypted = append(encrypted, goarSchema.Tag{
			Name:  tag.Name,
			Value: formatCipherValue(keyType, ciphertext),
		})
		changed = true
	}
	return encrypted, changed, nil
}

func DecryptTags(tags []goarSchema.Tag, signer interface{}) ([]goarSchema.Tag, bool, error) {
	decrypted := make([]goarSchema.Tag, 0, len(tags))
	changed := false
	for _, tag := range tags {
		if !IsEncryptedTagName(tag.Name) {
			decrypted = append(decrypted, tag)
			continue
		}
		plainName, err := PlainTagName(tag.Name)
		if err != nil {
			return nil, false, err
		}
		keyType, ciphertext, err := parseCipherValue(tag.Value)
		if err != nil {
			return nil, false, err
		}
		plaintext, err := decryptValue(ciphertext, keyType, signer)
		if err != nil {
			return nil, false, err
		}
		decrypted = append(decrypted, goarSchema.Tag{Name: plainName, Value: string(plaintext)})
		changed = true
	}
	return decrypted, changed, nil
}

func DecryptParams(tags []goarSchema.Tag, decryptKey interface{}) (map[string]string, bool, error) {
	decrypted := map[string]string{}
	changed := false
	for _, tag := range tags {
		if !IsEncryptedTagName(tag.Name) {
			continue
		}
		if _, err := PlainTagName(tag.Name); err != nil {
			return nil, false, err
		}
		keyType, ciphertext, err := parseCipherValue(tag.Value)
		if err != nil {
			return nil, false, err
		}
		plaintext, err := decryptValue(ciphertext, keyType, decryptKey)
		if err != nil {
			return nil, false, err
		}
		decrypted[tag.Name] = string(plaintext)
		changed = true
	}
	return decrypted, changed, nil
}

func DecryptParamMap(params map[string]string, decryptKey interface{}) (map[string]string, bool, error) {
	tags := make([]goarSchema.Tag, 0, len(params))
	for key, value := range params {
		tags = append(tags, goarSchema.Tag{Name: key, Value: value})
	}
	return DecryptParams(tags, decryptKey)
}

func encryptValue(plaintext []byte, publicKey, keyType string) ([]byte, error) {
	switch keyType {
	case KeyTypeEthereumECIES:
		pubBytes, err := goarUtils.Base64Decode(publicKey)
		if err != nil {
			return nil, err
		}
		pub, err := crypto.UnmarshalPubkey(pubBytes)
		if err != nil {
			return nil, err
		}
		return ecies.Encrypt(rand.Reader, ecies.ImportECDSAPublic(pub), plaintext, nil, nil)
	case KeyTypeArweaveRSAOAEP:
		pub, err := goarUtils.OwnerToPubKey(publicKey)
		if err != nil {
			return nil, err
		}
		return rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, plaintext, nil)
	default:
		return nil, fmt.Errorf("unsupported encryption key type: %s", keyType)
	}
}

func decryptValue(ciphertext []byte, keyType string, signer interface{}) ([]byte, error) {
	switch keyType {
	case KeyTypeEthereumECIES:
		ethSigner, ok := signer.(*goether.Signer)
		if !ok {
			return nil, errors.New("ethereum encrypted tag requires ethereum signer")
		}
		return ecies.ImportECDSA(ethSigner.GetPrivateKey()).Decrypt(ciphertext, nil, nil)
	case KeyTypeArweaveRSAOAEP:
		arSigner, ok := signer.(*goar.Signer)
		if !ok {
			return nil, errors.New("arweave encrypted tag requires arweave signer")
		}
		return rsa.DecryptOAEP(sha256.New(), rand.Reader, arSigner.PrvKey, ciphertext, nil)
	default:
		return nil, fmt.Errorf("unsupported encryption key type: %s", keyType)
	}
}

func formatCipherValue(keyType string, ciphertext []byte) string {
	return CipherValuePrefix + ":" + keyType + ":" + goarUtils.Base64Encode(ciphertext)
}

func parseCipherValue(value string) (string, []byte, error) {
	parts := strings.SplitN(value, ":", 4)
	if len(parts) != 4 || parts[0]+":"+parts[1] != CipherValuePrefix {
		return "", nil, errors.New("invalid encrypted tag value")
	}
	ciphertext, err := goarUtils.Base64Decode(parts[3])
	if err != nil {
		return "", nil, err
	}
	return parts[2], ciphertext, nil
}
