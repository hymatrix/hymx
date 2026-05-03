# Encrypted Custom Tags

**Summary**
Add SDK/node support for encrypted custom tags. Publicly stored `BundleItem` data keeps encrypted tag names/values; node execution receives a derived internal view where encrypted custom tags are decrypted and renamed back to their original names. Decrypted values must never be committed to Redis, Arweave, chainkit queues, checkpoints, or returned message APIs.

**Key Changes**
- Add encryption metadata to `/info` via `node/schema.Info`:
  - `Encryption-Public-Key`: current node public key, using existing bundler owner format.
  - `Encryption-Key-Type`: `ethereum-ecies` or `arweave-rsa-oaep-sha256`.
- Add a small tag encryption package/helper:
  - Prefix: `Hymx-Encrypted-`.
  - Public stored tag example: `Hymx-Encrypted-Secret`, value `hymxenc:v1:<key-type>:<base64-ciphertext>`.
  - Internal decrypted tag becomes `Secret`.
  - Reject encrypted reserved/protocol tags; only custom/extension tags are allowed.
- In SDK:
  - `Send`, `SendMessage`, `Spawn`, and async variants automatically detect prefixed tags.
  - If encrypted tags exist, SDK fetches and caches `/info`, encrypts values before signing, then sends.
  - For `308` redirect, SDK must re-fetch redirected node `/info`, re-encrypt original plaintext tags for that node, re-sign a new item, and resend.
- In node:
  - Keep the original verified `BundleItem` unchanged for assignment, Redis commit, chainkit upload, and public retrieval.
  - Build a decrypted internal copy only after raw signature verification and protocol routing checks.
  - Pass decrypted `schema.Message` / `schema.Process` into `applyMessage` / `applyProcess`; pass original raw item into assignment and persistence.
  - If decryption fails on the responsible node, reject the submission.

**Security Boundaries**
- Reserved tags remain plaintext and cannot use the encrypted prefix: `Data-Protocol`, `Variant`, `Type`, `Module`, `Scheduler`, `From-Process`, `Pushed-For`, `Sequence`, `Owner`, `Target`, `Anchor`, `Data`, `Tags`, plus assignment/checkpoint protocol fields.
- No logs should print decrypted tag values.
- `db.Commit`, `chainkit.getBundleItems`, `GetMessage`, and Arweave uploads must only see the original encrypted item.
- VMM `Meta.Params` may contain decrypted values because that is internal execution state; result/cache persistence remains module-controlled and must not automatically include decrypted params.

**Test Plan**
- Unit test ETH ECIES encrypt/decrypt round trip from `/info` public key and node private key.
- Unit test Arweave RSA-OAEP encrypt/decrypt round trip from `/info` public key and node private key.
- SDK test: prefixed custom tag is encrypted before signing; raw item contains prefix+ciphertext and not plaintext.
- SDK redirect test: encrypted send re-encrypts and re-signs for redirected node instead of replaying the first ciphertext.
- Node test: raw item committed to DB keeps ciphertext; internal VMM params receive decrypted custom tag without prefix.
- Node test: encrypted reserved tag is rejected.
- Run `go test ./sdk/... ./node/... ./utils/...` and then `go test ./...`.

**Assumptions**
- “自定义 tag” means only extension/business tags, not protocol routing, auth, assignment, checkpoint, or module scheduling tags.
- Decrypted values are available only for current in-memory node execution.
- Public API compatibility is preserved: existing plaintext tags continue to work unchanged.
