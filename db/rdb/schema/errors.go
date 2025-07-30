package schema

import "errors"

var (
	ErrInvalidProcessID = errors.New("err_invalid_process_id")
	ErrDBLockedTimeout  = errors.New("err_db_lock_timeout")
	ErrInvalidMsgID     = errors.New("err_invalid_msg_id")
)
