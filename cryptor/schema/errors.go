package schema

import "errors"

var (
	ErrMissingKey        = errors.New("err_missing_key")
	ErrInvalidPublicKey  = errors.New("err_invalid_public_key")
	ErrUnsupportedCipher = errors.New("err_unsupported_cipher")
)
