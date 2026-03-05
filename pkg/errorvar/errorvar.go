package errorvar

import "errors"

var (
	// Repository layer
	ErrEndpointRepositoryNil  = errors.New("endpoint repository is nil")
	ErrCertStoreRepositoryNil = errors.New("cert store repository is nil")

	// Service layer - enabled modules
	ErrEnabledModuleServiceNil = errors.New("enabled module service is nil")

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
