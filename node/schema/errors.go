package schema

import (
	"errors"
)

var (
	ErrInvalidType               = errors.New("err_invalid_type")
	ErrProcessIsRecovering       = errors.New("err_process_is_recovering")
	ErrProcessAlreadyExists      = errors.New("err_process_already_exist")
	ErrProcessNotFound           = errors.New("err_process_not_found")
	ErrSpawnRedirect             = errors.New("err_spwan_redirect")
	ErrMessageRedirect           = errors.New("err_message_redirect")
	ErrDuplicateItem             = errors.New("err_duplicate_item")
	ErrNotFoundNodes             = errors.New("err_not_found_nodes")
	ErrNotFoundCkp               = errors.New("err_not_found_ckp")
	ErrNotFoundMod               = errors.New("err_not_found_mod")
	ErrUnrecognizedSignatureType = errors.New("err_unrecognized_signature_type")
	ErrUnauthorizedNode          = errors.New("err_unauthorized_node")
)
