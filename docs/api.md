# HyMatrix HTTP API

This document describes the public HTTP API exposed by the HyMatrix node. All endpoints are served over HTTP and return JSON.

- Base URL: `http://<host>:<port>` (configured by the server)
- Content type: requests and responses use `application/json` unless stated otherwise
- Authentication: none (local/test network use). Production deployments should front the node with proper access controls.

## Error Model

- Client errors: `400` with body `{ "error": "<message>" }`
- Payment required: `402` with body `X402Response` (see Payment section)
- Redirects: `308 Permanent Redirect` with `Location` header and JSON body describing alternative nodes

Example client error response:
```json
{ "error": "err_invalid_params" }
```

### Redirect Handling (308)
Some endpoints may respond with `308` and a list of alternative nodes. Clients should retry the same request against one of the provided URLs.

Response body (array of nodes):
```json
[
  {
    "Acc-Id": "acc1",
    "Name": "node-1",
    "Role": "main",
    "Desc": "primary",
    "URL": "http://127.0.0.1:8080"
  }
]
```

## Endpoints

### Core Messaging

- `POST /`
  - Description: Submit a signed bundle item. The body must be a serialized goar bundle item.
  - Request: raw JSON of `BundleItem`
  - Success: `200` with `{ "id": "<item-id>" }`
  - Errors:
    - `400` with `{ "error": "..." }` on decode or handle failures
    - `400` with `{ "error": "err_process_stopped" }` when the target process is registered to this node but not currently running. The message is not assigned or persisted.
    - `402` with `X402Response` when payment is required
    - `308` with node redirect for spawn/message redirection
  - Example:
    ```bash
    curl -X POST \
      -H 'Content-Type: application/json' \
      --data '<bundle-item-json>' \
      http://localhost:8080/
    ```

- `POST /trysend`
  - Description: Trigger outbox sending for a given process and target (best-effort).
  - Request: `{ "pid": "<process-id>", "target": "<accid-or-url>" }`
  - Success: `200` (empty body)
  - Errors: `400` when params are missing or invalid

### Results

- `GET /result/:pid/:msgid`
  - Description: Fetch the execution result for a specific process `pid` and message id `msgid`.
  - Success:
    - `200` with `VmmResult` JSON when available
    - `200` with `null` when not found and no redirect
    - `308` with `Location` header and node list for redirect

- `GET /results/:pid?limit=<n>`
  - Description: List recent results for a process.
  - Query:
    - `limit` optional, default `5`
  - Success: `200` with `{ "edges": [ { "cursor": "<base64>", "node": <VmmResult> } ] }`

### Messages & Assignments

- `GET /message/:msgid`
  - Description: Fetch a `BundleItem` by message id.
  - Success: `200` with `BundleItem` JSON

- `GET /messageByNonce/:pid/:nonce`
  - Description: Fetch a `BundleItem` by nonce.
  - Success:
    - `200` with `BundleItem`
    - `404` with `null` when not found

- `GET /assignmentByNonce/:pid/:nonce`
  - Description: Fetch assignment by nonce.
  - Success:
    - `200` with `BundleItem`
    - `404` with `null` when not found

- `GET /assignmentByMessage/:msgid`
  - Description: Fetch assignment by message id.
  - Success: `200` with `BundleItem`

### Registry

- `GET /nodes`
  - Description: Get registry map of nodes `{ accid: Node }`.
  - Success: `200` with object map

- `GET /node/:accid`
  - Description: Get node detail by account id.
  - Success: `200` with `Node` or `200` with `null` when not found

- `GET /nodesByProcess/:pid`
  - Description: List nodes serving a process.
  - Success: `200` with `Node[]`

- `GET /processes/:accid`
  - Description: List processes assigned to a node account.
  - Success: `200` with `string[]`

### Token

- `GET /balanceOf/:accid` and `GET /balanceof/:accid`
  - Description: Get token balance for account id.
  - Success: `200` with numeric string (e.g. `"1000000000"`)

- `GET /stakeOf/:accid` and `GET /stakeof/:accid`
  - Description: Get stake amount for account id.
  - Success: `200` with numeric string

### Cache

- `GET /cache/:pid/:key`
  - Description: Get cached value for a process key.
  - Success: `200` with JSON string value (can be empty string)

### Modules

- `GET /modules`
  - Description: Get all supported module names.
  - Success: `200` with `string[]`

- `GET /module/:mid`
  - Description: Load module definition by module id.
  - Success: `200` with `Module` JSON
  - Errors: `400` with `{ "error": "err_not_found_mod" }` when missing

### Utilities

- `GET /info`
  - Description: Node information and versioning.
  - Success: `200` with JSON including protocol variant and node version

- `GET /callback?url=<http-url>`
  - Description: Server-side callback request to URL.
  - Success: `200` with `{ "message": "ok" }`
  - Errors:
    - `400` with `{ "error": "err_invalid_params" }` when `url` missing
    - `400` with `{ "error": "err_callback_failed" }` on fetch failure

### Admin

Admin endpoints are served only on the configured `adminPort`. If `adminPort` is empty or missing, the admin server is not started and these endpoints are unavailable.

- `POST /admin/vms/stop`
  - Description: Checkpoint and stop a live VM process on this node. The process remains registered in Registry.
  - Request: `{ "pid": "<process-id>" }`
  - Success: `200` with `{ "id": "<pid>", "message": "stopped" }`
  - Errors:
    - `400` with `{ "error": "err_invalid_params" }` when `pid` is missing
    - `400` with `{ "error": "err_core_process_cannot_stop" }` for token or registry
    - `400` with `{ "error": "err_process_not_found" }` when the VM is not live

- `POST /admin/vms/resume`
  - Description: Resume a registered but non-running VM by running recovery for that process.
  - Request: `{ "pid": "<process-id>" }`
  - Success: `200` with `{ "id": "<pid>", "message": "resumed" }`
  - Errors:
    - `400` with `{ "error": "err_invalid_params" }` when `pid` is missing
    - `400` with `{ "error": "err_process_already_exist" }` when the VM is already running
    - `400` with `{ "error": "err_process_not_found" }` when the process is not registered to this node

- `GET /admin/vms/running`
  - Description: List VM pids currently running in this node process.
  - Success: `200` with `{ "pids": ["<pid>"] }`

Clients can derive stopped VMs by comparing `GET /processes/:accid` with `GET /admin/vms/running`.

## Data Structures

### Node
```json
{
  "Acc-Id": "accX",
  "Name": "node-1",
  "Role": "main",
  "Desc": "desc",
  "URL": "http://127.0.0.1:8080"
}
```

### VmmResult (partial)
```json
{
  "Nonce": "123",
  "Timestamp": "1733385600",
  "Item-Id": "<item-id>",
  "From-Process": "<pid>",
  "Pushed-For": "",
  "Messages": [],
  "Spawns": [],
  "Assignments": [],
  "Output": {},
  "Data": "",
  "Cache": {},
  "Error": ""
}
```

### Error
```json
{ "error": "err_invalid_params" }
```

### X402Response
```json
{
  "x402Version": "1.0",
  "error": "",
  "accepts": [
    {
      "scheme": "ax",
      "network": "arweave",
      "resource": "hymx",
      "payTo": "<address>",
      "asset": "AX",
      "amount": "1000"
    }
  ]
}
```

## Notes
- Redirects are part of the protocol to route requests to the correct node for a process.
- Numeric values like balances are returned as strings.
- Some endpoints may return `null` with `200` when a resource is not found (by design).
