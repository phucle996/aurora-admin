package response

type ModuleReinstallCertResult struct {
	ModuleName string   `json:"module_name"`
	Scope      string   `json:"scope"`
	Endpoint   string   `json:"endpoint"`
	TargetHost string   `json:"target_host"`
	CertPath   string   `json:"cert_path"`
	KeyPath    string   `json:"key_path"`
	CAPath     string   `json:"ca_path"`
	Warnings   []string `json:"warnings"`

	HealthcheckPassed bool   `json:"healthcheck_passed"`
	HealthcheckOutput string `json:"healthcheck_output"`
}
