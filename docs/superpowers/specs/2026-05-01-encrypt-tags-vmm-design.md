# Encrypt Tags VMM Refactor Design

## Context

Encrypted tags currently decrypt in the node layer before execution reaches the VMM. That gives the node two incompatible responsibilities: it handles routing, persistence, and assignment, but also mutates the execution view by replacing encrypted tag values and stripping the encrypted prefix. This makes checkpoint sanitization complicated because the VMM environment can contain decrypted values that must not be serialized.

This refactor moves encrypted tag decryption to the VMM boundary. The node passes raw bundle item data through unchanged. The VMM derives an in-memory decrypted view for VM execution, while checkpoint data remains a raw replayable environment plus VM state and outbox.

## Goals

- The node must not decrypt encrypted tags.
- The node must not rename tag keys or strip encrypted prefixes.
- The encrypted tag prefix changes to `Encrypted-`.
- Raw tags retain their original keys and ciphertext values through node, VMM env, checkpoint, and outbox persistence.
- VM execution receives both the raw parameter view and a separate decrypted parameter view.
- Checkpoints serialize only raw env, VM state, and outbox.
- Restore re-derives decrypted parameters inside VMM from the raw env.

## Non-Goals

- This change does not introduce a new key management system.
- This change does not support both old `Hymx-Encrypted-` and new `Encrypted-` prefixes.
- This change does not make encrypted reserved protocol tags valid.
- This change does not change SDK redirect behavior except for the prefix update.

## Architecture

The VMM owns encrypted tag decryption at execution boundaries. It should hold a dependency named around decryption capability, not node identity, for example `tagDecryptKey` or `decryptKey`. The current implementation can pass the existing node signer into that field, because `tagcrypto` already knows how to decrypt with Ethereum and Arweave signer types. The VMM API should avoid naming this dependency `signer`, because future deployments may use a separate decryption key or a KMS-backed decryptor.

The node keeps raw bundle item handling responsibilities:

- verify bundle signatures;
- decode item type, signer, process id, and routing fields from raw tags;
- route or assign process/message items;
- persist raw items and signed checkpoints;
- pass raw process/message data into VMM.

The VMM derives execution data:

- `Spawn` derives decrypted params from raw process tags before spawning the VM.
- `Apply` derives decrypted params from raw message meta before calling `vm.Apply`.
- `Restore` derives decrypted params from raw checkpoint env before spawning/restoring the VM.

## Data Model

`vmm/schema.Meta` should use two parameter maps:

- `Params map[string]string`: raw parameter view parsed from raw tags.
- `DecryptedParams map[string]string`: plaintext view derived only from encrypted tags.

`Params` contains both normal tags and encrypted tags. Encrypted entries keep the original key and ciphertext value:

```text
Params["Action"] = "Transfer"
Params["Encrypted-Secret"] = "hymxenc:v1:ethereum-ecies:<ciphertext>"
```

`DecryptedParams` contains only encrypted tag entries. The key keeps the original encrypted tag name:

```text
DecryptedParams["Encrypted-Secret"] = "private-value"
```

`DecryptedParams` must be excluded from checkpoint JSON with `json:"-"`.

## Tag Crypto Behavior

`tagcrypto.EncryptedTagPrefix` changes from `Hymx-Encrypted-` to `Encrypted-`.

Encryption keeps the key unchanged and replaces only the value with ciphertext. Decryption for execution should derive a parameter map instead of producing a renamed tag list. A helper such as `DecryptParams(tags, decryptKey)` should return:

```text
map[string]string{
  "Encrypted-Secret": "private-value",
}
```

Existing validation still applies. `Encrypted-Type`, `Encrypted-Module`, and other encrypted reserved protocol tags remain invalid because protocol routing and validation fields must stay public and stable.

## Node Flow

`Handle` and `HandleMode` should decode raw items directly. The existing node-side `decryptInternalItem`, `decodeInternalItem`, `sanitizeCheckpointSnapshot`, and `decryptSnapshotEnv` responsibilities should be removed or retired.

`applyProcess` should:

- remember the raw spawn item if that cache remains useful;
- build `Meta.Params` from raw `proc.Tags`;
- leave `Meta.DecryptedParams` unset;
- pass raw `proc` and raw `meta` to `vmm.Spawn`.

`applyMessage` should:

- build `Meta.Params` from raw `msg.Tags`;
- leave `Meta.DecryptedParams` unset;
- pass raw `meta` to `vmm.Apply`.

The node should receive decryption errors only as VMM spawn/apply failures, not by pre-decrypting input.

## VMM Flow

Before VM execution, the VMM should create an execution meta from the raw meta:

1. Validate encrypted tag names.
2. Decrypt encrypted tag values using the configured decrypt key.
3. Populate `Meta.DecryptedParams`.
4. Preserve `Meta.Params` unchanged.

`Spawn` should derive from `process.Tags`. `Apply` should derive from the raw meta params or the original raw message tags represented in those params. `Restore` should derive from `snap.Env.Process.Tags` before spawning and restoring the VM.

Logs must redact both `Params` and `DecryptedParams`.

## VM and Core Module Behavior

VMs and core modules should use `Meta.DecryptedParams` when they intentionally need plaintext encrypted tag values. They should use `Meta.Params` when they need the raw public chain view.

Forwarding logic that currently prevents decrypted `X-` params from leaking should be updated to check `DecryptedParams` by encrypted key. Since encrypted keys keep the `Encrypted-` prefix, encrypted values should not match existing `X-` forwarding rules unless a module explicitly opts into reading and forwarding decrypted values. The default behavior is no decrypted value forwarding.

## Checkpoint and Restore

Checkpoint data must serialize only:

- raw `Env`;
- VM state data;
- outbox snapshot.

Raw `Env` means:

- `Env.Meta.Params` contains raw tag keys and values;
- `Env.Meta.DecryptedParams` is not serialized;
- `Env.Process.Tags` contains raw tag keys and values.

The node should sign and store this snapshot without sanitizing encrypted tag state.

On restore, the node loads and unmarshals the snapshot, then passes it directly to `vmm.Restore`. The VMM re-derives `Env.Meta.DecryptedParams` from `Env.Process.Tags`, spawns the VM if needed, restores VM state, and records the restored raw env.

## Error Handling

Decryption errors are VMM boundary errors:

- invalid encrypted tag name;
- encrypted reserved protocol tag;
- malformed ciphertext;
- unsupported encryption key type;
- decrypt key type mismatch;
- decryption failure.

Spawn failures return from `vmm.Spawn` to node. Apply failures should populate the VMM result error path consistently with existing apply errors.

## Testing

Update tests around these behaviors:

- `tagcrypto` uses `Encrypted-` and decrypts to a params map keyed by encrypted tag names.
- node no longer decrypts or mutates encrypted tags before VMM.
- VMM spawn passes raw `Params` and derived `DecryptedParams` to the VM.
- VMM apply passes raw `Params` and derived `DecryptedParams` to the VM.
- checkpoint JSON does not contain plaintext decrypted values.
- restore re-derives `DecryptedParams` from raw env.
- encrypted reserved tags fail at the VMM boundary.
- logs redact both raw and decrypted params.

## Compatibility

Plaintext tags continue to work unchanged. Existing users of encrypted tags must migrate from the old `Hymx-Encrypted-` prefix to `Encrypted-`. Checkpoints created after this refactor are raw and do not need node-side encrypted tag sanitization.
