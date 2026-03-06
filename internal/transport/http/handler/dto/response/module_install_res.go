package response

type ModuleInstallResult struct {
	ModuleName      string   `json:"module_name"`
	Scope           string   `json:"scope"`
	Endpoint        string   `json:"endpoint"`
	EndpointValue   string   `json:"endpoint_value"`
	InstallExecuted bool     `json:"install_executed"`
	InstallOutput   string   `json:"install_output"`
	InstallExitCode int      `json:"install_exit_code"`
	HostsUpdated    []string `json:"hosts_updated"`
	Warnings        []string `json:"warnings"`

	SchemaKey       string   `json:"schema_key"`
	SchemaName      string   `json:"schema_name"`
	MigrationFiles  []string `json:"migration_files"`
	MigrationSource string   `json:"migration_source"`

	HealthcheckPassed bool   `json:"healthcheck_passed"`
	HealthcheckOutput string `json:"healthcheck_output"`
}
