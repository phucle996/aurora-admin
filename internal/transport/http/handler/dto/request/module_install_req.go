package request

type ModuleInstallRequest struct {
	ModuleName     string  `json:"module_name"`
	Scope          string  `json:"scope"`
	AgentID        string  `json:"agent_id"`
	AppHost        string  `json:"app_host"`
	AppPort        int32   `json:"app_port"`
	Endpoint       string  `json:"endpoint"`
	InstallCommand string  `json:"install_command"`
	SudoPassword   *string `json:"sudo_password"`
}
