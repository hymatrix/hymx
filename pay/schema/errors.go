package schema

import "errors"

var (
	ErrTxAlreadyExecuted = errors.New("err_transaction_already_executed")
	ErrPaymentFailed     = errors.New("err_payment_failed")
)
