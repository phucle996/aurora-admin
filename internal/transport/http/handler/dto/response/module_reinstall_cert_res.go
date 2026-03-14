package response

type ModuleReinstallCertResult struct {
	ModuleName        string   `json:"module_name"`
	Endpoint          string   `json:"endpoint"`
	Warnings          []string `json:"warnings"`
	HealthcheckPassed bool     `json:"healthcheck_passed"`
}
