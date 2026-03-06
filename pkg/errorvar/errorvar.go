package errorvar

import "errors"

var (
	// Repository layer
	ErrEndpointRepositoryNil      = errors.New("endpoint repository is nil")
	ErrEndpointNameInvalid        = errors.New("endpoint name is invalid")
	ErrCertStoreRepositoryNil     = errors.New("cert store repository is nil")
	ErrRuntimeConfigRepositoryNil = errors.New("runtime config repository is nil")
	ErrRuntimeConfigKeyInvalid    = errors.New("runtime config key is invalid")

	// Service layer - enabled modules
	ErrEnabledModuleServiceNil = errors.New("enabled module service is nil")
	ErrModuleInstallServiceNil = errors.New("module install service is nil")
	ErrModuleNameInvalid       = errors.New("module name is invalid")
	ErrModuleInstallScope      = errors.New("module install scope is invalid")
	ErrModuleInstallCommand    = errors.New("custom install command is not allowed for local install")
	ErrModuleInstallerMissing  = errors.New("module installer is not configured")
	ErrModuleEndpointNotFound  = errors.New("module endpoint not found")
	ErrModuleEndpointInvalid   = errors.New("module endpoint metadata is invalid")

	// Service layer - api key
	ErrAPIKeyServiceNil    = errors.New("api key service is nil")
	ErrAPIKeyInvalid       = errors.New("api key is invalid")
	ErrAPIKeyMismatch      = errors.New("provided api key does not match current api key")
	ErrAPIKeyConflict      = errors.New("api key rotate conflict")
	ErrAPIKeyRotateTooSoon = errors.New("api key rotate is allowed every configured interval")

	// Service layer - cert store
	ErrCertStoreServiceNil = errors.New("cert store service is nil")
	ErrCertTypeInvalid     = errors.New("cert type is invalid")
	ErrObjectIDInvalid     = errors.New("object id is invalid")

	// Service layer - token secret
	ErrTokenSecretServiceNil  = errors.New("token secret service is nil")
	ErrTokenSecretKindInvalid = errors.New("token secret kind is invalid")
	ErrTokenSecretConflict    = errors.New("token secret rotate conflict")
)
