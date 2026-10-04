# HyMatrix

**HyMatrix** is an infinitely scalable decentralized computing network that decouples computation from consensus by anchoring execution logs in immutable storage (Arweave), enabling verifiable, trustless computation anywhere.

- ✅ **Trustless Verification** - Trust via execution logs, not global consensus
- ⚙️ **Multiple Runtimes** - Docker and WASM support (EVM coming soon)
- 📦 **Open Marketplace** - Join by staking AX tokens
- 💡 **Enterprise-Ready** - Built for AI, DeFi, and large-scale applications

> **Compute anywhere. Verify everything.**

## Table of Contents

- [Overview](#overview)
- [Version Information](#version-information)
  - [Node Version](#node-version)
  - [Protocol Version](#protocol-version)
  - [Version Compatibility](#version-compatibility)
- [Software Dependencies](#software-dependencies)
- [Getting Started](#getting-started)
  - [Wallet Setup](#wallet-setup)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [Running](#running)
- [Network Participation](#network-participation)
- [Developer Resources](#developer-resources)
  - [Examples](#examples)
  - [Documentation](#documentation)

## Overview

HyMatrix is a decentralized computing network where participants stake **AX** tokens (or **tAX** on testnet) to operate nodes and process computation tasks. The network architecture consists of:

**Registry Module** - The network's global directory that:
* Manages node identity (wallet address)
* Stores node metadata (name, description, URL)
* Tracks supported runtime environments
* Facilitates inter-VM communication

This architecture enables seamless service discovery and cross-VM operations while maintaining a trustless verification model through immutable execution logs.

## Version Information

HyMatrix uses two key version identifiers to ensure network compatibility and proper protocol handling:
- Node Version: Core node version, used to identify node functionality and performance
- Protocol Version: Protocol version, used to identify communication protocols between nodes
Specific information can be viewed in the /info endpoint
```
{
  "Protocol": "hymx",
  "Variant": "v0.1.0", // Protocol Version
  "NodeVersion": "v0.1.3",
  ......
}
```


## Software Dependencies

| Component        | Version        | Required       | Description                                     |
| ---------------- | -------------- | -------------- | ----------------------------------------------- |
| **Redis**        | Latest stable  | ✅ Required     | Internal task queue, messaging, and caching     |
| **Go**           | ≥ 1.24.0       | Optional (only for source builds) | Go language toolchain for building nodes         |
| **Git**          | Latest stable  | Optional (source builds only)     | Source control for cloning HyMatrix repo         |

## Getting Started

### Wallet Setup

Each HyMatrix node requires a unique wallet address that serves as its network identity and economic account for staking, rewards, and other operations.

| Wallet Type       | Technology | Recommendation                                  |
| ----------------- | ---------- | ----------------------------------------------- |
| **Ethereum** | ECDSA (secp256k1) | ✅ **Recommended** for faster signing performance |
| **Arweave**  | RSA keypair | For Arweave-integrated deployments |

> 💡 **Tip**: For production nodes, use Ethereum wallets for optimal performance.

### Installation

Build the HyMatrix node from source:

```bash
git clone https://github.com/hymatrix/hymx
cd hymx
go mod tidy
make build
```

### Configuration

Create a `config.yaml` file with the following settings:

```yaml
# Node Service
port: :8080
ginMode: release  # Options: "debug", "release"

# Redis Configuration
redisURL: redis://@localhost:6379/0

# Storage & Network
arweaveURL: https://arweave.net
hymxURL: http://127.0.0.1:8080

# Node Identity (Wallet)
prvKey: 0x64dd2342616f385f3e8157cf7246cf394217e13e8f91b7d208e9f8b60e25ed1b
keyfilePath:

# Node Info
nodeName: test1
nodeDesc: first test node
nodeURL: http://127.0.0.1:8080

# Registration & Network Join
joinNetwork: false
```

**Key Configuration Fields:**

| Field | Description |
|-------|-------------|
| `hymxURL` | Local node address for SDK-based calls |
| `prvKey` | Ethereum private key for signing operations |
| `nodeName`/`nodeURL` | Node metadata for network identification |
| `joinNetwork` | Set `false` for local testing, `true` to join network |


### Running

1. **Start Redis** - Ensure Redis is running and `redisURL` in your config is correct

2. **Launch the Node** - Run the binary with your configuration:

   ```bash
   ./hymx --config ./config.yaml
   ```

   Successful startup will show:
   ```
   INFO[07-25|00:00:01] server is running   module=node-v0.0.1 wallet=0x... port=:8080
   ```

### Stopping

For a daemon started with `hymx start`, run these commands from the same working directory:

```bash
hymx stop                # graceful shutdown without VM checkpoints
hymx stop --checkpoint   # save all running VM checkpoints, then shut down
```

Ctrl+C and SIGTERM also shut down without VM checkpoints. Existing checkpoints are retained;
startup uses the existing checkpoint/history recovery flow, so skipping a new checkpoint can
increase recovery time and requires the database history to remain available.

The stop command confirms signal delivery, not shutdown or checkpoint completion. The daemon
removes its PID file after shutdown; checkpoint failures are logged and shutdown continues.
`--checkpoint` uses SIGUSR1 and requires a daemon running this version; do not use it with an older daemon.

For embedded use, `Node.Close()` and `Server.Close()` skip VM checkpoints;
use `CloseWithCheckpoint()` to save them. Single-VM `Node.Stop(pid)` and `POST /admin/vms/stop`
also skip checkpoints; use `Node.StopWithCheckpoint(pid)` or `POST /admin/vms/stopWithCheckpoint`
to save before stopping. Payment state persistence is unchanged.

Outbox messages are stored in Redis lists independently of VM checkpoints. VM checkpoint and
restore no longer save or restore outbox queues; legacy checkpoint outbox fields are ignored.
Restarting hymx retains messages already written to Redis, but does not automatically resend them.
Call SDK `TrySend(pid, target)` (or `POST /trysend`) to resume a queue, or let a new message to the
same queue trigger sending. Redis durability depends on its persistence and eviction configuration;
outputs not yet written to Redis and failed writes are not protected by this change.
Before upgrading, drain pending messages from the old version: neither its in-memory outbox nor
outbox data in old VM checkpoints is automatically imported into Redis.

Local verification: `python3 scripts/checkpoint-local-network.py` starts isolated Redis and node
processes, checks Token/Registry operations and both shutdown/recovery paths, then cleans up its processes.

For the single-VM admin APIs, run `HYMX_INTEGRATION=1 go test ./server -run '^TestAdminIntegrationTestSuite$' -count=1 -timeout=90s -v`.
This starts a real local node and isolated Redis with a test-only non-core VM, verifying stop,
stopWithCheckpoint, resume and checkpoint failure behavior. It is skipped in ordinary test runs.

## Join the Network

To join the HyMatrix network as a node operator:

1. Configure your node with `joinNetwork: true`
2. Stake the required AX tokens
3. Complete the registration process

Participating nodes earn rewards for computation, log submission, and network services.

For detailed instructions, see [Join the Network](https://docs.hymatrix.com/docs/category/join-the-network)

## Developer Resources

### Examples

Reference implementations are available in the `examples` directory.

### Documentation

For comprehensive guides, API references, and advanced topics, refer to the [official documentation](https://docs.hymatrix.com/).

- API Reference: [HyMatrix HTTP API](docs/api.md)
- SDK Guide: [HyMatrix Go SDK Guide](docs/sdk.md)
