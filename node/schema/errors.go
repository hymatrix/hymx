package schema

import (
	"errors"

	registrySchema "github.com/hymatrix/hymx/vmm/core/registry/schema"
)

var (
	ErrInvalidType               = errors.New("err_invalid_type")
	ErrProcessIsRecovering       = errors.New("err_process_is_recovering")
	ErrProcessAlreadyExists      = errors.New("err_process_already_exist")
	ErrProcessNotFound           = errors.New("err_process_not_found")
	ErrRedirect                  = errors.New("err_redirect")
	ErrSchedulerWrong            = errors.New("err_scheduler_wrong")
	ErrDuplicateItem             = errors.New("err_duplicate_item")
	ErrNotFoundNodes             = errors.New("err_not_found_nodes")
	ErrNotFoundCkp               = errors.New("err_not_found_ckp")
	ErrNotFoundMod               = errors.New("err_not_found_mod")
	ErrUnrecognizedSignatureType = errors.New("err_unrecognized_signature_type")
	ErrUnauthorizedNode          = errors.New("err_unauthorized_node")
)

// RedirectError contains nodes information for 308 redirect
type RedirectError struct {
	Pid   string                `json:"pid"`
	Nodes []registrySchema.Node `json:"nodes"`
}

func (e *RedirectError) Error() string {
	return "err_redirect"
}

// NewRedirectError creates a new redirect error with nodes information
func NewRedirectError(pid string, nodes []registrySchema.Node) *RedirectError {
	return &RedirectError{Pid: pid, Nodes: nodes}
}

// ProcessNotFoundError carries pid for better context and unwraps to ErrProcessNotFound
type ProcessNotFoundError struct {
	Pid string `json:"pid"`
}

func (e *ProcessNotFoundError) Error() string {
	return ErrProcessNotFound.Error()
}

func NewProcessNotFoundError(pid string) *ProcessNotFoundError {
	return &ProcessNotFoundError{Pid: pid}
}

type SchedulerWrongError struct {
	Scheduler string `json:"scheduler"`
}

func (e *SchedulerWrongError) Error() string {
	return ErrSchedulerWrong.Error()
}

func NewSchedulerWrongError(scheduler string) *SchedulerWrongError {
	return &SchedulerWrongError{Scheduler: scheduler}
}
