package vmm

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/hymatrix/hymx/cryptor"
	"github.com/hymatrix/hymx/vmm/schema"
)

func TestDecryptParams(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	c := cryptor.NewRSAFromPrivateKey(privateKey)
	encryptedValue, err := c.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	v := &Vmm{cryptor: c}
	meta := schema.Meta{
		Params: map[string]string{
			"Foo":           "plain",
			"Encrypted-Bar": encryptedValue,
		},
	}

	v.decryptParams(&meta)
	if meta.Params["Foo"] != "plain" {
		t.Fatalf("Foo = %q, want plain", meta.Params["Foo"])
	}
	if meta.Params["Bar"] != "secret" {
		t.Fatalf("Bar = %q, want secret", meta.Params["Bar"])
	}
	if meta.Params["Encrypted-Bar"] != encryptedValue {
		t.Fatalf("Encrypted-Bar = %q, want %q", meta.Params["Encrypted-Bar"], encryptedValue)
	}
}

func TestDecryptParamsSkipsExistingPlaintextParam(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	c := cryptor.NewRSAFromPrivateKey(privateKey)
	encryptedValue, err := c.Encrypt("secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	v := &Vmm{cryptor: c}
	meta := schema.Meta{
		Params: map[string]string{
			"Bar":           "plain-secret",
			"Encrypted-Bar": encryptedValue,
		},
	}

	v.decryptParams(&meta)
	if meta.Params["Bar"] != "plain-secret" {
		t.Fatalf("Bar = %q, want plain-secret", meta.Params["Bar"])
	}
	if meta.Params["Encrypted-Bar"] != encryptedValue {
		t.Fatalf("Encrypted-Bar = %q, want %q", meta.Params["Encrypted-Bar"], encryptedValue)
	}
}

func TestDecryptParamsMissingDecryptor(t *testing.T) {
	v := &Vmm{}
	meta := schema.Meta{
		Params: map[string]string{
			"Encrypted-Bar": "secret",
		},
	}

	v.decryptParams(&meta)
	if meta.Params["Bar"] != schema.ErrMissingDecryptor.Error() {
		t.Fatalf("Bar = %q, want %q", meta.Params["Bar"], schema.ErrMissingDecryptor.Error())
	}
}
