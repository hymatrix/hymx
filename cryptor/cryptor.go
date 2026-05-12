package cryptor

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/cryptor/schema"
	"github.com/permadao/goar"
)

// Cryptor encrypts and decrypts strings with either RSA or ECC keys.
type Cryptor struct {
	Algorithm string
	Address   string

	rsaPrivate *rsa.PrivateKey
	rsaPublic  *rsa.PublicKey
	eccPrivate *ecdsa.PrivateKey
	eccPublic  *ecdsa.PublicKey
}

// NewRSAFromKeyfile creates an RSA cryptor from an Arweave keyfile path.
func NewRSAFromKeyfile(path string) (*Cryptor, error) {
	signer, err := goar.NewSignerFromPath(path)
	if err != nil {
		return nil, err
	}
	return NewRSAFromPrivateKey(signer.PrvKey), nil
}

// NewRSAFromKeyfileBytes creates an RSA cryptor from Arweave keyfile bytes.
func NewRSAFromKeyfileBytes(keyfile []byte) (*Cryptor, error) {
	signer, err := goar.NewSigner(keyfile)
	if err != nil {
		return nil, err
	}
	return NewRSAFromPrivateKey(signer.PrvKey), nil
}

// NewRSAFromPrivateKey creates an RSA cryptor from a private key.
func NewRSAFromPrivateKey(privateKey *rsa.PrivateKey) *Cryptor {
	signer := goar.NewSignerByPrivateKey(privateKey)
	return &Cryptor{
		Algorithm:  schema.AlgorithmRSA,
		Address:    signer.Address,
		rsaPrivate: privateKey,
		rsaPublic:  &privateKey.PublicKey,
	}
}

// NewECCFromPrivateKey creates an ECC cryptor from an Ethereum private key.
func NewECCFromPrivateKey(prvKey string) (*Cryptor, error) {
	signer, err := goether.NewSigner(prvKey)
	if err != nil {
		return nil, err
	}
	privateKey := signer.GetPrivateKey()
	return &Cryptor{
		Algorithm:  schema.AlgorithmECC,
		Address:    signer.Address.String(),
		eccPrivate: privateKey,
		eccPublic:  &privateKey.PublicKey,
	}, nil
}

// NewFromPublicKey creates an encryption-only cryptor from a public key string.
func NewFromPublicKey(algorithm string, publicKey string) (*Cryptor, error) {
	switch algorithm {
	case schema.AlgorithmRSA:
		pub, err := parseRSAPublicKey(publicKey)
		if err != nil {
			return nil, err
		}
		return &Cryptor{
			Algorithm: schema.AlgorithmRSA,
			rsaPublic: pub,
		}, nil
	case schema.AlgorithmECC:
		pub, err := parseECCPublicKey(publicKey)
		if err != nil {
			return nil, err
		}
		return &Cryptor{
			Algorithm: schema.AlgorithmECC,
			eccPublic: pub,
		}, nil
	default:
		return nil, schema.ErrUnsupportedCipher
	}
}

// Encrypt encrypts plaintext and returns a base64 ciphertext string.
func (c *Cryptor) Encrypt(plaintext string) (string, error) {
	var ciphertext []byte
	var err error
	switch c.Algorithm {
	case schema.AlgorithmRSA:
		if c.rsaPublic == nil {
			return "", schema.ErrMissingKey
		}
		ciphertext, err = rsa.EncryptOAEP(sha256.New(), rand.Reader, c.rsaPublic, []byte(plaintext), nil)
	case schema.AlgorithmECC:
		if c.eccPublic == nil {
			return "", schema.ErrMissingKey
		}
		ciphertext, err = ecies.Encrypt(rand.Reader, ecies.ImportECDSAPublic(c.eccPublic), []byte(plaintext), nil, nil)
	default:
		return "", schema.ErrUnsupportedCipher
	}
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

	var plaintext []byte
	switch c.Algorithm {
	case schema.AlgorithmRSA:
		if c.rsaPrivate == nil {
			return "", schema.ErrMissingKey
		}
		plaintext, err = rsa.DecryptOAEP(sha256.New(), rand.Reader, c.rsaPrivate, ciphertextBytes, nil)
	case schema.AlgorithmECC:
		if c.eccPrivate == nil {
			return "", schema.ErrMissingKey
		}
		plaintext, err = ecies.ImportECDSA(c.eccPrivate).Decrypt(ciphertextBytes, nil, nil)
	default:
		return "", schema.ErrUnsupportedCipher
	}
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// PublicKey returns the public key string and its algorithm.
func (c *Cryptor) PublicKey() (string, string, error) {
	switch c.Algorithm {
	case schema.AlgorithmRSA:
		if c.rsaPublic == nil {
			return "", c.Algorithm, schema.ErrMissingKey
		}
		publicKey, err := marshalPublicKey(c.rsaPublic)
		return publicKey, c.Algorithm, err
	case schema.AlgorithmECC:
		if c.eccPublic == nil {
			return "", c.Algorithm, schema.ErrMissingKey
		}
		return hexutil.Encode(crypto.FromECDSAPub(c.eccPublic)), c.Algorithm, nil
	default:
		return "", c.Algorithm, schema.ErrUnsupportedCipher
	}
}

func marshalPublicKey(publicKey any) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})), nil
}

func parseRSAPublicKey(publicKey string) (*rsa.PublicKey, error) {
	pub, err := parsePKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, schema.ErrInvalidPublicKey
	}
	return rsaPub, nil
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

	pub, err := parsePKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	eccPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, schema.ErrInvalidPublicKey
	}
	return eccPub, nil
}

func parsePKIXPublicKey(publicKey string) (any, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(publicKey)))
	if block == nil {
		return nil, schema.ErrInvalidPublicKey
	}
	return x509.ParsePKIXPublicKey(block.Bytes)
}
