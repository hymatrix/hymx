package schema

import "errors"

var (
	ErrRegistryAlreadyCreated = errors.New("err_registry_already_created")
	ErrTokenAlreadyCreated    = errors.New("err_token_already_created")
	ErrInvalidModuleFormat    = errors.New("err_invalid_module_format")
	ErrInvalidNonce           = errors.New("err_invalid_nonce")
	ErrSequenceTooLow         = errors.New("err_sequence_too_low")
	ErrSpawnProcessFailed     = errors.New("err_spawn_process_failed")
	ErrProcessAlreadyExists   = errors.New("err_process_already_exist")
	ErrProcessNotFound        = errors.New("err_process_not_found")
	ErrProcessEnvNotFound     = errors.New("err_process_env_not_found")
	ErrRegistryNotFound       = errors.New("err_registry_not_found")
	ErrMissingParam           = errors.New("err_missing_param")
	ErrInvalidAccid           = errors.New("err_invalid_accid")
	ErrFactoryAlreadyMounted  = errors.New("err_factory_already_mounted")
	ErrMissingDecryptor       = errors.New("err_missing_decryptor")
	ErrDecryptParamFailed     = errors.New("err_decrypt_param_failed")
)
