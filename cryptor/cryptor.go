package cryptor

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/cryptor/schema"
)

// Cryptor encrypts and decrypts strings with ECC keys.
type Cryptor struct {
	Address string

	eccPrivate *ecdsa.PrivateKey
	eccPublic  *ecdsa.PublicKey
}

// NewECCFromPrivateKey creates an ECC cryptor from an Ethereum private key.
func NewECCFromPrivateKey(prvKey string) (*Cryptor, error) {
	signer, err := goether.NewSigner(prvKey)
	if err != nil {
		return nil, err
	}
	privateKey := signer.GetPrivateKey()
	return &Cryptor{
		Address:    signer.Address.String(),
		eccPrivate: privateKey,
		eccPublic:  &privateKey.PublicKey,
	}, nil
}

// NewFromPublicKey creates an encryption-only cryptor from a public key string.
func NewFromPublicKey(publicKey string) (*Cryptor, error) {
	pub, err := parseECCPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	return &Cryptor{
		eccPublic: pub,
	}, nil
}

// Encrypt encrypts plaintext and returns a base64 ciphertext string.
func (c *Cryptor) Encrypt(plaintext string) (string, error) {
	if c.eccPublic == nil {
		return "", schema.ErrMissingKey
	}
	ciphertext, err := ecies.Encrypt(rand.Reader, ecies.ImportECDSAPublic(c.eccPublic), []byte(plaintext), nil, nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decodes a base64 ciphertext string and decrypts it.
func (c *Cryptor) Decrypt(ciphertext string) (string, error) {
	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	if c.eccPrivate == nil {
		return "", schema.ErrMissingKey
	}
	plaintext, err := ecies.ImportECDSA(c.eccPrivate).Decrypt(ciphertextBytes, nil, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// PublicKey returns the public key string.
func (c *Cryptor) PublicKey() (publicKey string, err error) {
	if c.eccPublic == nil {
		return "", schema.ErrMissingKey
	}
	return hexutil.Encode(crypto.FromECDSAPub(c.eccPublic)), nil
}

func parseECCPublicKey(publicKey string) (*ecdsa.PublicKey, error) {
	publicKey = strings.TrimSpace(publicKey)
	if strings.HasPrefix(publicKey, "0x") {
		publicKeyBytes, err := hexutil.Decode(publicKey)
		if err != nil {
			return nil, err
		}
		pub, err := crypto.UnmarshalPubkey(publicKeyBytes)
		if err != nil {
			return nil, err
		}
		return pub, nil
	}

	return nil, schema.ErrInvalidPublicKey
}
