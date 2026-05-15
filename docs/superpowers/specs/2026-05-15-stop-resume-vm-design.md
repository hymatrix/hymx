# Stop/Resume VM Design

## Context

Hymx currently keeps live VM instances in `Vmm.vms` and `Vmm.vmsEnv`. The Registry VM records process-to-node registration, and node recovery rebuilds VMs from checkpoints plus persisted messages and assignments.

This design adds admin-only stop/resume controls for a VM process:

- `stop` checkpoints the VM, removes the live VM from `Vmm.vms`, and keeps the Registry registration intact.
- `resume` runs the normal recovery flow for that process so the VM is loaded back into `Vmm.vms`.
- VMM remains stateless with respect to stop state. A stopped VM is derived as: registered to this node in Registry, but absent from the running VMM list.
- Messages sent to a stopped VM fail immediately with a dedicated error and are not assigned or persisted.
- Stop only affects the current node process lifetime. If the Hymx node process restarts, existing recovery behavior restores all VMs.
- Core VMs cannot be stopped: token and registry are protected.

## Goals

- Provide admin HTTP APIs for stop, resume, and listing currently running VMs.
- Expose admin APIs only on a separate configured port.
- Keep normal public APIs unchanged when admin is not configured.
- Preserve user VM context: if stop fails at any point, the VM must remain or be restored to service.
- Keep Registry process records unchanged during stop.
- Reuse existing checkpoint, VMM kill, and recovery mechanisms instead of creating a separate lifecycle state machine.

## Non-Goals

- No complex authentication, tokens, sessions, or role model.
- No persistent stopped state across node restart.
- No VMM stopped map, stopped marker, or stopped state API.
- No Registry schema change for process lifecycle state.
- No direct stopped-list API. Clients can derive stopped VMs by comparing Registry process lists with the running VM list.
- No support for stopping token or registry core VMs.
- No queueing or replaying messages rejected while a VM is stopped.

## Admin API

`adminPort` is added to node config. If `adminPort` is empty or missing, no admin server is started and admin functionality is unavailable over HTTP.

When configured, the server starts a second Gin engine bound to `adminPort` with only admin routes:

```text
POST /admin/vms/stop
POST /admin/vms/resume
GET  /admin/vms/running
```

`stop` and `resume` take `pid` from the JSON request body, not the URL:

```json
{ "pid": "<pid>" }
```

`stop` and `resume` success responses use the same response envelope as `POST /` (`server/schema.Response`):

```json
{ "id": "<pid>", "message": "stopped" }
```

```json
{ "id": "<pid>", "message": "resumed" }
```

Errors use the existing error response shape:

```json
{ "error": "err_process_stopped" }
```

The running VM list response is:

```json
{ "pids": ["<pid>"] }
```

## VMM Changes

VMM must not store stopped state. No `vmsStopped`, `IsStopped`, `MarkStopped`, `UnmarkStopped`, or `GetStoppedPids` methods are added.

The VMM change is limited to exposing the currently running VM pids, which already exists as `GetVmPids()`.

For stop removal, reuse the existing `Kill(pid)` method in `vmm/manage.go` instead of creating a separate stop-state method. `Kill(pid)` already closes the VM and deletes `vms[pid]` and `vmsEnv[pid]`. If implementation needs clearer naming at the Node layer, `Node.StopVM` can call `vmm.Kill(pid)` directly.

Any improvement to `Kill` should preserve its current contract:

- return `err_process_not_found` when the VM is not running;
- close the VM before deleting it from VMM maps;
- delete both `vms` and `vmsEnv` only after close succeeds.

## Node Changes

`Node` owns lifecycle orchestration because it already has access to checkpoint persistence, nonce state, Registry lookup, and recovery.

Expected methods:

```go
func (n *Node) StopVM(pid string) error
func (n *Node) ResumeVM(pid string) error
func (n *Node) GetRunningVMs() []string
```

### Stop Flow

`StopVM(pid)` uses a conservative two-phase flow:

1. Validate `pid`.
2. Reject token and registry core VM pids with `err_core_vm_cannot_stop`.
3. Reject if the VM is not currently running in VMM.
4. Reject if the VM is recovering.
5. Create a checkpoint with the existing `Node.Checkpoint(pid)`.
6. Save the checkpoint file with existing `SaveCheckpoint`.
7. Save the checkpoint index with `db.SaveCheckpointIndex(pid, checkpointID)`.
8. Call existing `vmm.Kill(pid)` to close and remove the live VM.

If steps 1-7 fail, `StopVM` returns the error and does not modify VMM state.

If step 8 fails after any partial state change, `StopVM` must restore service before returning. The rollback path should run recovery for `pid` using the checkpoint just saved and return the original stop error after best-effort service restoration. The implementation should keep `vmm.Kill` atomic enough that partial state change is unlikely.

### Resume Flow

`ResumeVM(pid)` runs a recovery operation for a stopped process:

1. Validate `pid`.
2. Reject if the VM is already running with `err_process_already_exist`.
3. Verify Registry still has this node registered for `pid`; otherwise return `err_process_not_found`.
4. Read the current nonce with `db.GetNonce(pid)`.
5. Read the checkpoint index with `db.GetCheckpointIndex(pid)`. If unavailable, continue with an empty checkpoint id and recover from message history.
6. Call `recoveryProcess(pid, maxNonce, checkpointID, ExecModeDryRun)`.
7. If recovery succeeds, the VM is available in `Vmm.vms`.

Resume is therefore equivalent to running the existing recovery flow for one VM. There is no stopped marker to clear or restore.

### Running VM List

`GetRunningVMs()` returns `vmm.GetVmPids()`.

Clients that need stopped VMs can compare:

- Registry processes for this node from `GET /processes/:accid`;
- running VMs from `GET /admin/vms/running`.

The difference is the currently stopped set for the node process.

## Message Handling

Because VMM has no stopped state, stopped detection is derived during message handling.

When a message targets a pid that is not running locally:

1. Keep the existing redirect check first.
2. If redirect does not apply and `vmm.IsExists(pid)` is false, check Registry nodes for that process.
3. If Registry says the current node is registered for `pid`, return `err_process_stopped`.
4. Otherwise return the existing `err_process_not_found`.

This applies to normal submitted messages. A stopped VM:

- returns `err_process_stopped` from `POST /`;
- does not receive an assignment;
- does not advance nonce;
- does not persist the rejected message;
- does not produce a `VmmResult`.

Because the message is rejected before persistence, resume does not replay stop-period traffic.

## Registry Behavior

Stopping a VM does not send `UnregisterProcess` and does not mutate Registry state. Existing registry queries continue to show the process as served by the node.

This is intentional: stop is a local runtime management operation, not a protocol-level process unregistration. It also enables deriving stopped VMs by comparing Registry process registration with the running VM list.

## Errors

Add these errors where they fit existing package boundaries:

```text
err_process_stopped
err_core_vm_cannot_stop
```

`err_process_stopped` is returned by public message submission when the target VM is registered to this node but not currently running.

`err_core_vm_cannot_stop` is returned by stop for token and registry core VMs.

Resume uses existing errors where possible:

- `err_process_already_exist` when the VM is already running;
- `err_process_not_found` when the pid is neither running nor registered to this node.

No public `err_admin_api_disabled` response is required because an unset `adminPort` means the admin server does not listen.

## Configuration

`LoadNodeConfig` reads `adminPort` from config and returns it to command startup. `Server.Run` accepts the admin endpoint in addition to the public endpoint.

If `adminPort` is empty, `Server.Run` starts only the existing public API. `Server.Close` shuts down the admin server only when it was started.

Existing configs remain valid because missing `adminPort` means disabled.

## Testing

Unit and integration tests should cover:

1. `StopVM` checkpoint failure leaves VM running.
2. `StopVM` checkpoint save or checkpoint-index save failure leaves VM running.
3. Successful stop removes the VM from `Vmm.vms` by reusing `vmm.Kill` and preserves Registry process registration.
4. `POST /` to a registered-but-not-running pid returns `err_process_stopped`, does not create an assignment, and does not increment nonce.
5. `POST /` to an unknown pid still returns `err_process_not_found`.
6. Successful resume rebuilds the VM through `recoveryProcess`.
7. Resume of an already running VM returns `err_process_already_exist`.
8. token and registry stop return `err_core_vm_cannot_stop`.
9. Missing `adminPort` does not start the admin server.
10. Configured `adminPort` starts admin routes on the separate admin server without adding those routes to the public API.
11. `GET /admin/vms/running` returns currently running VM pids.

## Implementation Notes

- Do not add stopped state to VMM.
- Reuse `vmm.Kill(pid)` for stop removal if it satisfies the failure semantics.
- Keep admin HTTP handlers separate from public route registration, preferably in a new server file.
- Avoid changing Registry schema or snapshots.
- Avoid adding persistent stopped keys to Redis.
- Do not add authentication beyond the separate admin port.
