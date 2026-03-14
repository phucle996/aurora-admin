package response

type ModuleInstallResult struct {
	OperationID      string   `json:"operation_id,omitempty"`
	ModuleName       string   `json:"module_name"`
	AgentID          string   `json:"agent_id,omitempty"`
	Version          string   `json:"version,omitempty"`
	ArtifactChecksum string   `json:"artifact_checksum,omitempty"`
	ServiceName      string   `json:"service_name,omitempty"`
	Endpoint         string   `json:"endpoint"`
	Health           string   `json:"health,omitempty"`
	HostsUpdated     []string `json:"hosts_updated"`
	Warnings         []string `json:"warnings"`
}
