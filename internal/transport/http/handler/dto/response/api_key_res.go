package response

import "admin/internal/service"

type APIKeyVersion struct {
	Version int64  `json:"version"`
	Key     string `json:"key"`
}

func NewAPIKeyVersion(v service.APIKeyVersion) APIKeyVersion {
	return APIKeyVersion{
		Version: v.Version,
		Key:     v.Key,
	}
}
