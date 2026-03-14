package request

type ModuleInstallRequest struct {
	ModuleName string `json:"module_name"`
	AgentID    string `json:"agent_id"`
	AppHost    string `json:"app_host"`
}
