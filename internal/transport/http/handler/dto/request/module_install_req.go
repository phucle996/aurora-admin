package request

type ModuleInstallRequest struct {
	ModuleName            string  `json:"module_name"`
	Scope                 string  `json:"scope"`
	AppHost               string  `json:"app_host"`
	AppPort               int32   `json:"app_port"`
	Endpoint              string  `json:"endpoint"`
	InstallCommand        string  `json:"install_command"`
	SSHHost               string  `json:"ssh_host"`
	SSHPort               int32   `json:"ssh_port"`
	SSHUsername           string  `json:"ssh_username"`
	SSHPassword           *string `json:"ssh_password"`
	SSHPrivateKey         *string `json:"ssh_private_key"`
	SSHHostKeyFingerprint *string `json:"ssh_host_key_fingerprint"`
}
