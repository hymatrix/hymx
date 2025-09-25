package schema

import "errors"

var (
	ErrSpawnTxNotFound       = errors.New("err_spawn_tx_not_found")
	ErrInvalidProcessType    = errors.New("err_invalid_process_type")
	ErrInvalidAssignmentType = errors.New("err_invalid_assignment_type")
	ErrInvalidDownloadItems  = errors.New("err_invalid_download_items")
)
