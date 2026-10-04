package schema

import "errors"

// VmPendingMessageLimit is an advisory threshold, not a reserved admission quota.
const VmPendingMessageLimit = 48

var ErrProcessBusy = errors.New("err_process_busy")
