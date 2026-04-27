# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Project Overview

HyMatrix is a decentralized computing network that decouples computation from consensus by anchoring execution logs in immutable storage (Arweave). It enables verifiable, trustless computation anywhere.

Key components:
- **Node**: Central orchestrator managing the entire system (`/node/`)
- **Server**: HTTP API server (`/server/`)
- **VMM**: Virtual Machine Manager (`/vmm/`)
- **Chainkit**: Blockchain interactions and data persistence (`/chainkit/`)
- **SDK**: Client SDK (`/sdk/`)

Core modules:
- **Registry**: Network directory managing node identities (`/vmm/core/registry/`)
- **Token**: AX token economics (`/vmm/core/token/`)

## Build Commands

```bash
# Build the node binary
make build

# Build for specific platforms
make build-linux    # Linux x86_64
make build-darwin   # macOS x86_64
make build-arm      # macOS/Linux ARM64
make build-windows  # Windows x86_64
make build-all      # All platforms

# Clean build artifacts
make clean

# Run tests
go test ./...                    # Run all tests
go test ./db/cache/...          # Test specific package
go test -v ./sdk/...            # Verbose test output
```

## Development Setup

1. **Prerequisites**:
   - Go ≥ 1.24.0
   - Redis (latest stable)
   - Arweave keyfile or Ethereum private key

2. **Initial build**:
   ```bash
   go mod tidy
   make build
   ```

3. **Configuration**: Create `config.yaml` with node identity, Redis URL, Arweave URL, and wallet credentials

4. **Run locally** (in separate terminals):
   ```bash
   # Terminal 1: Redis
   redis-server

   # Terminal 2: Node
   ./build/hymx --config ./config.yaml
   ```

## Architecture Patterns

### Node Communication Flow

1. **Message Submission**: Clients send signed BundleItems to nodes via HTTP POST
2. **VM Execution**: VMM processes messages through registered modules
3. **Result Storage**: Results stored in Redis and optionally anchored to Arweave
4. **Outbox Pattern**: Messages are queued per (pid, target) pair and sent to target nodes/processes
   - Messages stored directly in queue structure (`targets[pid][target]`)
   - `Commit()` removes messages from memory immediately after successful send
   - Queue becomes empty when all messages are sent (Peek returns nil without error)

### Module System

Modules are registered in `cmd/main.go`:
```go
s.Mount("<moduleFormat>", spawnFunction)
```

Module format must match the Module-Format tag in the module definition. Modules need to implement the spawn interface for creating process instances.

### Wallet Identity

- Ethereum wallets (ECDSA/secp256k1) recommended for production (faster signing performance)
- Arweave wallets (RSA keypair) supported for Arweave-integrated deployments
- Node identity derived from wallet address
- Used for signing messages and network participation
- Configure via `prvKey` (Ethereum) or `keyfilePath` (Arweave) in config.yaml

### Checkpoint and Recovery

- Process state snapshots include VM state, outbox queues, and module-specific caches
- Checkpoints serialized to JSON and stored locally (and optionally to Arweave)
- Recovery restores process state from checkpoint data
- Outbox queues restored with pending messages only (not historical messages)

## Testing and Examples

The `examples/` directory contains reference implementations:

```bash
# Initialize token and registry
go run ./examples init

# Token operations
go run ./examples token_info
go run ./examples transfer
go run ./ examples stake

# Module operations
go run ./examples module_gen
go run ./examples module_load <id>
go run ./ examples module_upload <modfile>

# Network queries
go run ./examples nodes
go run ./examples node <accid>
go run ./examples nodesByProcess <pid>
go run ./examples processes <accid>

# Outbox operations
go run ./examples trysend <pid> <target>
```

## Key APIs and Patterns

### HTTP API
Base endpoints in `/server/route.go`:
- `POST /` - Submit signed bundle items
- `GET /result/:pid/:msgid` - Fetch execution results
- `GET /results/:pid` - List recent results
- Registry and token endpoints under `/nodes`, `/balanceOf`, etc.

### SDK Usage
```go
s := sdk.New("http://node:8080", "./keyfile.json")
resp, err := s.SendMessageAndWait(pid, payload, tags)
```

### Configuration
- Uses Viper for YAML config file parsing
- CLI flags via urfave/cli
- Configuration split across `cmd/cfgnode.go`, `cmd/cfgpay.go`, `cmd/cfgchainkit.go`

## Important Notes

- Set `joinNetwork: false` for local testing, `true` for network participation
- Redis required for all deployments (task queues, messaging, caching)
- Execution logs anchored to Arweave for verification
- Token operations require stake amounts defined in token schema
- Redirect handling (308) is part of the protocol for routing to correct nodes

## Cache Layer Architecture (`/db/cache/`)

The cache layer provides in-memory data structures with checkpoint/restore capabilities:

- **Outbox** (`outbox.go`): Message queue for inter-process/inter-node communication
  - Direct queue structure per (pid, target) pair
  - Thread-safe with RWMutex
  - Messages deleted from memory on Commit
- **Token** (`token.go`): AX token balances and stakes cache
- **Pay** (`pay.go`): Payment ledger and transaction tracking
- **Schema** (`schema/snap.go`): Snapshot definitions for checkpoint serialization

All cache structures implement Checkpoint/Restore for process recovery.
