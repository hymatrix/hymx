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
	ErrDuplicateItem             = errors.New("err_duplicate_item")
	ErrNotFoundNodes             = errors.New("err_not_found_nodes")
	ErrNotFoundCkp               = errors.New("err_not_found_ckp")
	ErrNotFoundMod               = errors.New("err_not_found_mod")
	ErrUnrecognizedSignatureType = errors.New("err_unrecognized_signature_type")
	ErrUnauthorizedNode          = errors.New("err_unauthorized_node")
)

// RedirectError contains nodes information for 308 redirect
type RedirectError struct {
	Nodes []registrySchema.Node `json:"nodes"`
}

func (e *RedirectError) Error() string {
	return "err_redirect"
}

// NewRedirectError creates a new redirect error with nodes information
func NewRedirectError(nodes []registrySchema.Node) *RedirectError {
	return &RedirectError{Nodes: nodes}
}
