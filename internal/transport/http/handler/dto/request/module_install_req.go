package request

type ModuleInstallRequest struct {
	ModuleName     string  `json:"module_name"`
	Scope          string  `json:"scope"`
	InstallRuntime string  `json:"install_runtime"`
	AgentID        string  `json:"agent_id"`
	AppHost        string  `json:"app_host"`
	AppPort        int32   `json:"app_port"`
	Endpoint       string  `json:"endpoint"`
	InstallCommand string  `json:"install_command"`
	Kubeconfig     string  `json:"kubeconfig"`
	KubeconfigPath string  `json:"kubeconfig_path"`
	SudoPassword   *string `json:"sudo_password"`
}
