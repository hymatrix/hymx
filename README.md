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
  - [Redis Security](#redis-security)
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
| **Redis**        | Pinned, patched release (see [Redis Security](#redis-security)) | ✅ Required     | Internal task queue, messaging, and caching     |
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

# Redis Configuration (dedicated ACL user, see "Redis Security")
redisURL: redis://hymx:<url-encoded-password>@127.0.0.1:6379/0

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
| `redisURL` | Redis connection URL. Use a dedicated ACL user with a password; use `rediss://` when Redis is reached over an untrusted network |
| `hymxURL` | Local node address for SDK-based calls |
| `prvKey` | Ethereum private key for signing operations |
| `nodeName`/`nodeURL` | Node metadata for network identification |
| `joinNetwork` | Set `false` for local testing, `true` to join network |

> 🔒 `config.yaml` contains wallet material and Redis credentials. Make it owned by the service account that runs hymx and readable only by it: `chmod 600 config.yaml`.

When `enableChainkit: true`, `chainkit.redisURL` is a second Redis connection; apply the same credentials and network rules to it.

### Redis Security

HyMatrix stores its message queues, results, checkpoint indexes, outbox queues and caches in Redis. Anyone who can reach that Redis instance can read, modify or delete this data. Redis must only be reachable by hymx — never expose it to the Internet (see the [Redis security guide](https://redis.io/docs/latest/operate/oss_and_stack/management/security/)).

The `redis://@localhost:6379/0` style URL (no credentials) is only acceptable for throwaway local development.

#### 1. Network exposure

**Redis installed directly on the host** — bind to loopback only in `redis.conf`:

```conf
bind 127.0.0.1 -::1
protected-mode yes
```

**hymx on the host, Redis in Docker** — publish the port on loopback only:

```yaml
services:
  redis:
    ports:
      - "127.0.0.1:6379:6379"   # loopback only
```

> ⚠️ Never use a mapping without a host address, such as `"6379:6379"` or `-p 6379:6379`. Docker then publishes the port on **all** host interfaces (`0.0.0.0` and `[::]`) and its iptables rules can bypass host firewalls such as ufw. See [Docker port publishing](https://docs.docker.com/engine/network/port-publishing/).

**hymx and Redis both in Docker** — do not publish Redis at all; put it on an internal network shared only with hymx:

```yaml
services:
  hymx:
    image: <your-hymx-image>
    networks: [frontend, redis-internal]
    # redisURL: redis://hymx:<url-encoded-password>@redis:6379/0

  redis:
    image: redis:<patched-version>@sha256:<digest>
    expose: ["6379"]           # reachable from redis-internal only, no "ports:"
    networks: [redis-internal]

networks:
  frontend:
  redis-internal:
    internal: true
```

Inside the container Redis may bind to all container interfaces; exposure is controlled by Docker networking.

#### 2. Authentication and ACLs

Disable the `default` user and give hymx a dedicated ACL user that can only run the commands it needs on its own key prefixes. Example `users.acl`:

```conf
user default off
user hymx on #<sha256-hex-of-password> resetkeys ~p:* ~m:* ~ckp:* ~cache:* ~chainkit:* ~outbox:* resetchannels -@all +auth +hello +ping +select +client|setinfo +get +set +exists +del +scan +hget +hset +hdel +rpush +lpop +llen +lrange +lindex +sadd +sismember +smembers +scard
```

- Generate the password hash with `printf '%s' "$REDIS_PASSWORD" | shasum -a 256`, so the plaintext password is not stored in the ACL file.
- Load the file with `aclfile /usr/local/etc/redis/users.acl` in `redis.conf`.
- Keep `protected-mode yes` as defense in depth.
- Administrative (`CONFIG`, `ACL`, `SHUTDOWN`, `DEBUG`, `MODULE`), destructive (`FLUSHALL`, `FLUSHDB`, `KEYS`) and scripting (`EVAL`, `FUNCTION`) commands are denied; hymx does not need them. Use a separate admin user for maintenance and never put it in `redisURL`.
- The command list matches what the current hymx code uses. If a future version needs another command, hymx logs a `NOPERM` error; grant only that command.
- URL-encode special characters in the password inside `redisURL` (for example `@` → `%40`, `/` → `%2F`).
- If Redis must be reached over a network you do not fully trust, enable TLS on Redis and use `rediss://` in `redisURL`.

#### 3. Version pinning

- Do not use mutable tags such as `redis:latest` or `redis:7`. Pin a supported, patched release and its image digest:

  ```bash
  docker pull redis:<patched-version>
  docker inspect --format '{{index .RepoDigests 0}}' redis:<patched-version>
  ```

- Minimum versions for [CVE-2025-49844](https://redis.io/blog/security-advisory-cve-2025-49844/) (critical Lua use-after-free): **7.2.11**, **7.4.6**, **8.0.4**, **8.2.2** or later on the same branch. Follow Redis security advisories and move to new patch releases promptly; a dependency bot (e.g. Dependabot or Renovate) can keep the pinned digest up to date.

#### 4. Container hardening

Recommended settings for the Redis container:

```yaml
services:
  redis:
    image: redis:<patched-version>@sha256:<digest>
    command: ["redis-server", "/usr/local/etc/redis/redis.conf"]
    user: "999:999"            # redis user in the official image
    cap_drop: [ALL]
    security_opt: ["no-new-privileges:true"]
    read_only: true
    tmpfs: ["/tmp"]
    volumes:
      - redis-data:/data
      - ./redis/redis.conf:/usr/local/etc/redis/redis.conf:ro
      - ./redis/users.acl:/usr/local/etc/redis/users.acl:ro
    pids_limit: 256
    mem_limit: 2g              # size to your workload; also set maxmemory in redis.conf
    restart: unless-stopped

volumes:
  redis-data:
```

Do not mount the Docker socket or unrelated host paths into the Redis container. Files bind-mounted into the container (`redis.conf`, `users.acl`, existing data) must be readable by UID 999.

#### 5. Migrating an existing installation

1. Stop hymx (`hymx stop --checkpoint` if you want fresh checkpoints) and back up Redis data: run `SAVE` (or `BGSAVE`) and copy `dump.rdb` and the `appendonlydir`/AOF files from the data directory.
2. Recreate Redis from a pinned, patched image with the network layout above (no public port, or loopback-only when hymx runs on the host), reusing the backed-up data directory.
3. Add the ACL file, update `redisURL` (and `chainkit.redisURL` if used) with the new user and password, and rotate any credentials used previously.
4. Start hymx and confirm it runs normally.

#### 6. Verifying the deployment

```bash
# From another machine: the connection must fail
redis-cli -h <server-public-ip> -p 6379 ping

# On the host: Redis must only listen on loopback (no 0.0.0.0 or [::])
ss -ltnp | grep 6379

# Unauthenticated access must be rejected with NOAUTH
redis-cli -h 127.0.0.1 -p 6379 ping

# The hymx user works, but administrative/scripting commands are rejected with NOPERM
redis-cli -h 127.0.0.1 -p 6379 --user hymx --askpass ping
redis-cli -h 127.0.0.1 -p 6379 --user hymx --askpass config get maxmemory
redis-cli -h 127.0.0.1 -p 6379 --user hymx --askpass eval "return 1" 0
```

For the internal-network Compose layout, run the `redis-cli` checks with `docker compose exec redis redis-cli ...` and confirm `docker compose port redis 6379` prints nothing.

### Running

1. **Start Redis** - Ensure Redis is running with the settings in [Redis Security](#redis-security) and `redisURL` in your config is correct

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
