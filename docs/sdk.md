# HyMatrix Go SDK Guide

This document explains how to use the HyMatrix Go SDK to send messages, spawn processes, query results, and interact with node endpoints.

- Package: `github.com/hymatrix/hymx/sdk`
- Dependencies: `goar` bundler, HyMatrix server endpoints

## Initialization

Create an SDK bound to a node URL using an Arweave keyfile:

```go
import (
  "github.com/hymatrix/hymx/sdk"
)

s := sdk.New("http://127.0.0.1:8080", "./arweave-keyfile.json")
defer s.Close()

addr := s.GetAddress() // wallet address
```

Alternatively, initialize from an existing bundler:

```go
bundler, _ := goar.NewBundler(signer)
s := sdk.NewFromBundler("http://xxxxx", bundler)
```

### Ethereum 0x Wallet Signer
Create a signer from an Ethereum private key (hex string starting with `0x`) and use it with the bundler:

```go
import (
  "github.com/everFinance/goether"
  "github.com/permadao/goar"
  "github.com/hymatrix/hymx/sdk"
)

url := "http://xxxxx"
prvKey := "0x<your-hex-private-key>"

signer, _ := goether.NewSigner(prvKey)
bundler, _ := goar.NewBundler(signer)
s := sdk.NewFromBundler(url, bundler)

addr := s.GetAddress()
```

## Sending Data

### Send
Low-level send that signs a `BundleItem` and posts to the node.

```go
res, redirectedURL, err := s.Send("process-id", "payload", nil)
if err != nil { /* handle */ }
// redirectedURL may contain alternate node URL after 308 redirect
// res.Id is the message id
```

- Adds an `SDK-Timestamp` tag automatically.
- Returns the server `Response` `{ id, message }` and any redirect URL.

### SendMessage
High-level helper to construct a `Message` with standard tags:

```go
resp, err := s.SendMessage("target-pid", "hello", []goarSchema.Tag{
  {Name: "Action", Value: "Echo"},
})
```

### Spawn
Create a process with a module and scheduler:

```go
resp, err := s.Spawn("module-id", "scheduler-accid", []goarSchema.Tag{
  {Name: "Param", Value: "value"},
})
```

## Waiting for Results

Use async helpers to block until a `VmmResult` is available.

### SendAndWait
Signs, sends, follows redirects, then waits for the result.

```go
res, err := s.SendAndWait("process-id", "payload", nil)
// res.Message contains JSON-encoded VmmResult
```

### SendMessageAndWait
Convenient wrapper for messages:

```go
res, err := s.SendMessageAndWait("target-pid", "hello", []goarSchema.Tag{{Name: "Action", Value: "Echo"}})
```

### SpawnAndWait
Create a process and wait for its initial result:

```go
res, err := s.SpawnAndWait("module-id", "scheduler-accid", nil)
```

### ResultAndWait
Polls the node every second up to 2 minutes:

```go
result, err := s.ResultAndWait("pid", "msgid")
```

## Client API

The SDK embeds an HTTP client for direct endpoint access.

```go
c := s.Client
```

- Redirect handling: 308 responses are parsed and retried against suggested nodes.

### Node Info
```go
info, err := c.Info()
```

### Callback
```go
body, err := c.Callback("http://example.com/ping")
```

### Results
```go
r, err := c.GetResult("pid", "msgid")
list, err := c.GetResults("pid", 5)
```

### Messages & Assignments
```go
msg, err := c.GetMessage("msgid")
msgN, err := c.GetMessageByNonce("pid", 1)
assignN, err := c.GetAssignByNonce("pid", 1)
assignM, err := c.GetAssignByMessage("msgid")
```

### Registry
```go
nodes, err := c.GetNodes()
node, err := c.GetNode("accid")
byProc, err := c.GetNodesByProcess("pid")
procs, err := c.GetProcesses("accid")
```

### Token
```go
bal, err := c.BalanceOf("accid")
stk, err := c.StakeOf("accid")
```

### Cache
```go
val, err := c.GetCache("pid", "key")
```

### Outbox Trigger
```go
err := c.TrySend("pid", "target")
```

## Module Utilities

Build a signed module item for later use:

```go
item, err := s.GenerateModule([]byte("module-bytes"), schema.Module{
  Base: schema.DefaultBaseModule,
  ModuleFormat: "hymx.test.mod",
})
```

You can persist or upload these items using your own storage flow.

## Tags & Types

- Tags are `[]goarSchema.Tag` with fields `Name` and `Value`.
- Helpers: `utils.MessageToTags`, `utils.ProcessToTags`, and `utils.MergeTags`.
- Crypto tags: use `SendMessageWithEncryptedParams`, `SendMessageWithEncryptedParamsAndWait`, or `EncryptTags`; see [Crypto Tags Guide](./crypto-tags.md).
- Server response type: `serverSchema.Response { Id, Message }`.
- Execution result type: `vmmSchema.VmmResult`.

## Errors & Redirects

- Non-2xx responses return errors like `invalid server response: <code>`.
- Some methods may return `308 Permanent Redirect`; the client auto-retries to alternative nodes when possible.

## Example: Echo Roundtrip

```go
s := sdk.New("http://127.0.0.1:8080", "./arweave-keyfile.json")
defer s.Close()

res, err := s.SendMessageAndWait("echo-pid", "hello", []goarSchema.Tag{{Name: "Action", Value: "Echo"}})
if err != nil { panic(err) }
fmt.Println("result:", res.Message)
```

## Notes

- Always call `Close()` to release HTTP resources.
- Use `SendAndWait` family for simpler redirect handling and result polling.
- Balances and stakes are returned as decimal strings; convert as needed.
