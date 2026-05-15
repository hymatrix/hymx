# Stop/Resume VM Design

## Context

Hymx currently keeps live VM instances in `Vmm.vms` and `Vmm.vmsEnv`. The Registry VM records process-to-node registration, and node recovery rebuilds VMs from checkpoints plus persisted messages and assignments.

This design adds admin-only stop/resume controls for a VM process:

- `stop <pid>` checkpoints the VM, removes the live VM from `Vmm.vms`, and keeps the Registry registration intact.
- `resume <pid>` runs the normal recovery flow for that process so the VM is loaded back into `Vmm.vms`.
- Messages sent to a stopped VM fail immediately with a dedicated error and are not assigned or persisted.
- Stop state is in-memory only. If the Hymx node process restarts, existing recovery behavior restores all VMs.
- Core VMs cannot be stopped: token and registry are protected.

## Goals

- Provide admin HTTP APIs for stop, resume, and listing stopped VMs.
- Expose admin APIs only on a separate configured port.
- Keep normal public APIs unchanged when admin is not configured.
- Preserve user VM context: if stop fails at any point, the VM must remain or be restored to service.
- Keep Registry process records unchanged during stop.
- Reuse existing checkpoint and recovery mechanisms instead of creating a separate restore path.

## Non-Goals

- No complex authentication, tokens, sessions, or role model.
- No persistent stopped state across node restart.
- No Registry schema change for process lifecycle state.
- No support for stopping token or registry core VMs.
- No queueing or replaying messages rejected while a VM is stopped.

## Admin API

`adminPort` is added to node config. If `adminPort` is empty or missing, no admin server is started and admin functionality is unavailable over HTTP.

When configured, the server starts a second Gin engine bound to `adminPort` with only admin routes:

```text
POST /admin/vms/:pid/stop
POST /admin/vms/:pid/resume
GET  /admin/vms/stopped
```

Responses:

```json
{ "pid": "<pid>", "message": "stopped" }
```

```json
{ "pid": "<pid>", "message": "resumed" }
```

```json
{ "pids": ["<pid>"] }
```

Errors use the existing error response shape:

```json
{ "error": "err_process_stopped" }
```

## VMM Changes

New VM lifecycle methods are added in `vmm/manage.go`.

Expected methods:

```go
func (v *Vmm) Stop(pid string) error
func (v *Vmm) MarkStopped(pid string)
func (v *Vmm) UnmarkStopped(pid string)
func (v *Vmm) IsStopped(pid string) bool
func (v *Vmm) GetStoppedPids() []string
```

`Stop` removes the VM from `vms` and `vmsEnv` and marks the pid stopped. It must hold the VMM lock while mutating these maps so the transition is atomic from callers' perspective.

`MarkStopped` and `UnmarkStopped` support rollback and resume orchestration from `Node`.

The stopped set is memory-only and lives on `Vmm`, for example:

```go
vmsStopped map[string]bool
```

## Node Changes

`Node` owns lifecycle orchestration because it already has access to checkpoint persistence, nonce state, and recovery.

Expected methods:

```go
func (n *Node) StopVM(pid string) error
func (n *Node) ResumeVM(pid string) error
func (n *Node) ListStoppedVMs() []string
```

### Stop Flow

`StopVM(pid)` uses a conservative two-phase flow:

1. Validate `pid`.
2. Reject token and registry core VM pids with `err_core_vm_cannot_stop`.
3. Reject if the VM does not exist, is already stopped, or is recovering.
4. Create a checkpoint with the existing `Node.Checkpoint(pid)`.
5. Save the checkpoint file with existing `SaveCheckpoint`.
6. Save the checkpoint index with `db.SaveCheckpointIndex(pid, checkpointID)`.
7. Call `vmm.Stop(pid)` to close and remove the live VM and mark it stopped.

If steps 1-6 fail, `StopVM` returns the error and does not modify VMM state.

If step 7 fails after any partial state change, `StopVM` must restore service before returning. The rollback path should:

1. Clear the stopped marker for `pid`.
2. Restore from the checkpoint just saved, or run recovery if restore is not sufficient.
3. Return the original stop error after the VM is serviceable again.

The implementation should make `vmm.Stop` atomic enough that this rollback path is rarely needed, but the Node-level contract is still that failed stop does not leave user VM context broken.

### Resume Flow

`ResumeVM(pid)` runs a recovery operation for the stopped process:

1. Validate that `pid` is currently stopped.
2. Read the current nonce with `db.GetNonce(pid)`.
3. Read the checkpoint index with `db.GetCheckpointIndex(pid)`.
4. Clear the stopped marker so recovery can apply messages normally.
5. Call `recoveryProcess(pid, maxNonce, checkpointID, ExecModeDryRun)`.
6. If recovery fails, mark the pid stopped again and return the error.
7. If recovery succeeds, leave the pid unmarked and available in `Vmm.vms`.

Resume is therefore equivalent to running the existing recovery flow for one VM.

## Message Handling

After `Node.Handle` decodes the target pid and before assignment, it checks:

```go
if n.vmm.IsStopped(pid) {
    return schema.ErrProcessStopped
}
```

This applies to normal submitted messages. A stopped VM:

- returns `err_process_stopped` from `POST /`;
- does not receive an assignment;
- does not advance nonce;
- does not persist the rejected message;
- does not produce a `VmmResult`.

Because the message is rejected before persistence, resume does not replay stop-period traffic.

## Registry Behavior

Stopping a VM does not send `UnregisterProcess` and does not mutate Registry state. Existing registry queries continue to show the process as served by the node.

This is intentional: stop is a local runtime management operation, not a protocol-level process unregistration.

## Errors

Add these errors where they fit existing package boundaries:

```text
err_process_stopped
err_process_not_stopped
err_core_vm_cannot_stop
```

`err_process_stopped` is returned by public message submission when the target VM is stopped.

`err_process_not_stopped` is returned by resume when the pid is not in the stopped set.

`err_core_vm_cannot_stop` is returned by stop for token and registry core VMs.

No public `err_admin_api_disabled` response is required because an unset `adminPort` means the admin server does not listen.

## Configuration

`LoadNodeConfig` reads `adminPort` from config and returns it to command startup. `Server.Run` accepts the admin endpoint in addition to the public endpoint.

If `adminPort` is empty, `Server.Run` starts only the existing public API. `Server.Close` shuts down the admin server only when it was started.

Existing configs remain valid because missing `adminPort` means disabled.

## Testing

Unit and integration tests should cover:

1. `StopVM` checkpoint failure leaves VM running and unstopped.
2. `StopVM` checkpoint save or checkpoint-index save failure leaves VM running and unstopped.
3. Successful stop removes the VM from `Vmm.vms`, marks it stopped, and preserves Registry process registration.
4. `POST /` to a stopped pid returns `err_process_stopped`, does not create an assignment, and does not increment nonce.
5. Successful resume clears stopped state and rebuilds the VM through `recoveryProcess`.
6. Failed resume re-marks the pid stopped.
7. token and registry stop return `err_core_vm_cannot_stop`.
8. Missing `adminPort` does not start the admin server.
9. Configured `adminPort` starts admin routes on the separate admin server without adding those routes to the public API.

## Implementation Notes

- Keep lifecycle methods in `vmm/manage.go` as requested.
- Keep admin HTTP handlers separate from public route registration, preferably in a new server file.
- Avoid changing Registry schema or snapshots.
- Avoid adding persistent stopped keys to Redis.
- Do not add authentication beyond the separate admin port.
