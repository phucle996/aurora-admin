package request

type LoginAPIKeyRequest struct {
	APIKey string `json:"api_key"`
}

type RotateAPIKeyRequest struct {
	OldKey string `json:"old_key"`
}
