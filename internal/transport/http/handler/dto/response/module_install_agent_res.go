package response

type ModuleInstallAgent struct {
	AgentID           string `json:"agent_id"`
	Status            string `json:"status"`
	Hostname          string `json:"hostname"`
	IPAddress         string `json:"ip_address"`
	AgentGRPCEndpoint string `json:"agent_grpc_endpoint"`
	LastSeenAt        string `json:"last_seen_at"`
	Host              string `json:"host"`
	Port              int32  `json:"port"`
	Username          string `json:"username"`
}
