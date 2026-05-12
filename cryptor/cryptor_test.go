package cryptor

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hymatrix/hymx/cryptor/schema"
)

func TestRSAKeyfileEncryptDecrypt(t *testing.T) {
	c, err := NewRSAFromKeyfile("../cmd/test_keyfile.json")
	if err != nil {
		t.Fatalf("NewRSAFromKeyfile() error = %v", err)
	}

	plaintext := "hymx rsa payload"
	ciphertext, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	decrypted, err := c.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestRSAEncryptWithPublicKeyDecryptWithPrivateKey(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	privateCryptor := NewRSAFromPrivateKey(privateKey)
	publicKey, algorithm, err := privateCryptor.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey() error = %v", err)
	}
	publicCryptor, err := NewFromPublicKey(algorithm, publicKey)
	if err != nil {
		t.Fatalf("NewFromPublicKey() error = %v", err)
	}

	plaintext := "public rsa payload"
	ciphertext, err := publicCryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	decrypted, err := privateCryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestECCEncryptDecrypt(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	privateCryptor, err := NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	if err != nil {
		t.Fatalf("NewECCFromPrivateKey() error = %v", err)
	}
	publicKey, algorithm, err := privateCryptor.PublicKey()
	if err != nil {
		t.Fatalf("PublicKey() error = %v", err)
	}
	publicCryptor, err := NewFromPublicKey(algorithm, publicKey)
	if err != nil {
		t.Fatalf("NewFromPublicKey() error = %v", err)
	}

	plaintext := "hymx ecc payload"
	ciphertext, err := publicCryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	decrypted, err := privateCryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestECCEncryptWithHexPublicKey(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	privateCryptor, err := NewECCFromPrivateKey(hexutil.Encode(crypto.FromECDSA(privateKey)))
	if err != nil {
		t.Fatalf("NewECCFromPrivateKey() error = %v", err)
	}
	publicCryptor, err := NewFromPublicKey(schema.AlgorithmECC, hexutil.Encode(crypto.FromECDSAPub(&privateKey.PublicKey)))
	if err != nil {
		t.Fatalf("NewFromPublicKey() error = %v", err)
	}

	plaintext := "hex public key payload"
	ciphertext, err := publicCryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	decrypted, err := privateCryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestRSAKeyfileAlgorithm(t *testing.T) {
	rsaCryptor, err := NewRSAFromKeyfile("../cmd/test_keyfile.json")
	if err != nil {
		t.Fatalf("NewRSAFromKeyfile() error = %v", err)
	}
	if rsaCryptor.Algorithm != schema.AlgorithmRSA {
		t.Fatalf("NewRSAFromKeyfile() algorithm = %s, want %s", rsaCryptor.Algorithm, schema.AlgorithmRSA)
	}
}
