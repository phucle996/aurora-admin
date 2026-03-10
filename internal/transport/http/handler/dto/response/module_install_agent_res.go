package response

type ModuleInstallAgent struct {
	AgentID           string `json:"agent_id"`
	Status            string `json:"status"`
	Hostname          string `json:"hostname"`
	AgentGRPCEndpoint string `json:"agent_grpc_endpoint"`
}
