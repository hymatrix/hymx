package schema

import "errors"

var (
	ErrMissingRecipient      = errors.New("err_missing_recipient")
	ErrMissingQuantity       = errors.New("err_missing_quantity")
	ErrInvalidQuantityFormat = errors.New("err_invalid_quantity_format")
	ErrInsufficientBalance   = errors.New("err_insufficient_balance")
	ErrInsufficientStake     = errors.New("err_insufficient_stake")
	ErrUnauthorized          = errors.New("err_unauthorized")
)
