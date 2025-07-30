package schema

import "errors"

var (
	ErrUnauthorized  = errors.New("err_unauthorized")
	ErrMissingParam  = errors.New("err_missing_param")
	ErrNodeNotFound  = errors.New("err_node_not_found")
	ErrInvalidAction = errors.New("err_invalid_action")
)
