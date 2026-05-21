# Crypto Tags Guide

Crypto tags are HyMatrix message or process tags whose values are encrypted before they are sent to a node. They are useful for passing sensitive parameters to a VM without exposing the plaintext in the signed bundle item tags.

In the current implementation, a crypto tag is represented as a normal `goar` tag with:

- Name prefixed by `Encrypted-`
- Value encrypted with the target node's public encryption key

For example, a plaintext parameter:

```go
goarSchema.Tag{Name: "Secret", Value: "top-secret"}
```

is sent as:

```go
goarSchema.Tag{Name: "Encrypted-Secret", Value: "<base64-ciphertext>"}
```

Before VM execution, the VMM decrypts `Encrypted-Secret` and adds `Secret` to `Meta.Params`.

## When to Use Crypto Tags

Use crypto tags for parameters that should not be publicly readable from bundle item tags, such as:

- API keys or short-lived credentials
- private routing parameters
- user-specific secrets
- sensitive business parameters consumed by the VM

Do not use crypto tags for required protocol tags such as `Data-Protocol`, `Variant`, `Type`, `Module`, or `Scheduler`. Those tags are needed by the node before VM execution and must remain plaintext.

## Requirements

Crypto tags require both sides to support encryption:

- The node must be started with an EVM private key in `prvKey`.
- The node publishes the matching public key through `/info` as `Encryption-Public-Key`.
- The SDK encrypts values with that public key.
- The VMM decrypts values with the node's private key before calling `Vm.Apply`.

If the node is started only from an Arweave keyfile, `Encryption-Public-Key` is empty and encrypted parameter helpers cannot initialize the SDK cryptor from `/info`.

Example node config:

```yaml
prvKey: 0x<evm-private-key>
keyfilePath: ./test_keyfile.json
```

When `prvKey` is set, the node can sign with the EVM signer and decrypt crypto tags.

## SDK Usage

### Send Plain and Encrypted Params

Use `SendMessageWithEncryptedParams` when a message has a mix of public and private parameters:

```go
import (
  "github.com/hymatrix/hymx/sdk"
  goarSchema "github.com/permadao/goar/schema"
)

s := sdk.New("http://127.0.0.1:8080", "./arweave-keyfile.json")
defer s.Close()

resp, err := s.SendMessageWithEncryptedParams(
  "target-process-id",
  "payload",
  []goarSchema.Tag{
    {Name: "Action", Value: "Submit"},
    {Name: "PublicRef", Value: "order-123"},
  },
  []goarSchema.Tag{
    {Name: "Secret", Value: "top-secret"},
    {Name: "Token", Value: "session-token"},
  },
)
if err != nil {
  panic(err)
}

_ = resp.Id
```

The SDK sends the public params unchanged and converts the encrypted params to `Encrypted-Secret` and `Encrypted-Token`.

### Send and Wait for Result

Use `SendMessageWithEncryptedParamsAndWait` when the client should block until the VMM result is available:

```go
res, err := s.SendMessageWithEncryptedParamsAndWait(
  "target-process-id",
  "payload",
  []goarSchema.Tag{{Name: "Action", Value: "Submit"}},
  []goarSchema.Tag{{Name: "Secret", Value: "top-secret"}},
)
if err != nil {
  panic(err)
}

fmt.Println(res.Message)
```

### Encrypt Tags Manually

Use `EncryptTags` directly if you need to build the final tag list yourself:

```go
encryptedTags, err := s.EncryptTags([]goarSchema.Tag{
  {Name: "Secret", Value: "top-secret"},
})
if err != nil {
  panic(err)
}

resp, err := s.SendMessage(
  "target-process-id",
  "payload",
  append(
    []goarSchema.Tag{{Name: "Action", Value: "Submit"}},
    encryptedTags...,
  ),
)
```

`EncryptTags` loads the node public key lazily from `Client.Info()` unless a cryptor was already injected through `Client.SetCryptor`.

## VM Runtime Behavior

During message or process execution, tags are converted into `Meta.Params`. Before `Vm.Apply` is called, the VMM scans the params for names with the `Encrypted-` prefix.

For each encrypted param:

1. The prefix is removed to calculate the plaintext key.
2. The value is decrypted with the node decryptor.
3. The decrypted value is added to `Meta.Params` under the plaintext key.
4. The original encrypted param remains present.

Example:

```go
// Tags sent by the client:
Encrypted-Secret = "<ciphertext>"
PublicRef = "order-123"

// Params visible to the VM after decryption:
meta.Params["Encrypted-Secret"] = "<ciphertext>"
meta.Params["Secret"] = "top-secret"
meta.Params["PublicRef"] = "order-123"
```

Inside a VM:

```go
func (vm *MyVm) Apply(from string, meta schema.Meta) schema.Result {
  secret := meta.Params["Secret"]
  publicRef := meta.Params["PublicRef"]

  _ = secret
  _ = publicRef

  return schema.Result{Output: map[string]string{"status": "ok"}}
}
```

## Conflict and Error Rules

### Plaintext Wins on Key Conflict

If both `Secret` and `Encrypted-Secret` are present, the VMM keeps the plaintext `Secret` value and skips decrypting the encrypted value into `Secret`.

```go
meta.Params["Secret"] = "plain-secret"
meta.Params["Encrypted-Secret"] = "<ciphertext>"
```

The VM sees `Secret = "plain-secret"`.

### Decryption Failure

If decryption fails, the VMM sets the plaintext key to the error string:

```go
meta.Params["Secret"] = "err_decrypt_param_failed"
```

The original `Encrypted-Secret` value remains in `Meta.Params`.

### Node Without Decryptor

If the node does not support decryption, encrypted params are left unchanged and the plaintext key is not added:

```go
meta.Params["Encrypted-Secret"] = "<ciphertext>"
// meta.Params["Secret"] is not set
```

VM code should treat missing decrypted params as an error when the secret is required.

### Duplicate Tags

HyMatrix rejects duplicate tag names when converting tags into messages, processes, modules, assignments, or params. Duplicate checks are exact and case-sensitive.

```go
[]goarSchema.Tag{
  {Name: "Secret", Value: "a"},
  {Name: "Secret", Value: "b"}, // duplicate
}
```

When the SDK merges generated protocol tags with custom params, earlier tags take precedence. For example, `SendMessageWithEncryptedParams` preserves plaintext params over encrypted params if both slices contain the same tag name.

## Reserved Tags

Keep protocol and routing tags in plaintext. These names are used by the node and should not be hidden behind `Encrypted-`:

| Tag | Purpose |
| --- | --- |
| `Data-Protocol` | HyMatrix protocol marker |
| `Variant` | protocol version |
| `Type` | item type, such as `Message` or `Process` |
| `Module` | process module id |
| `Scheduler` | process scheduler account id |
| `From-Process` | source process |
| `Pushed-For` | forwarded item id |
| `Action` | message action |
| `Sequence` | cross-process message sequence |
| `Process` | assignment/checkpoint process id |
| `Message` | assignment message id |
| `Nonce` | assignment/checkpoint nonce |
| `Timestamp` | assignment timestamp |

Custom business tags are good candidates for encryption.

## Best Practices

- Encrypt only the tags that need confidentiality; keep routing and protocol tags plaintext.
- Use short string values because tag values are still carried in signed item metadata.
- Prefer `SendMessageWithEncryptedParams` over manual encryption for normal SDK usage.
- Validate required decrypted params in VM code instead of assuming they exist.
- Do not send both plaintext and encrypted versions of the same logical key unless the plaintext fallback is intentional.
- Start decrypting nodes with `prvKey`; otherwise clients cannot discover a usable `Encryption-Public-Key` from `/info`.
