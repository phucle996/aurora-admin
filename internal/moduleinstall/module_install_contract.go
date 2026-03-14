package moduleinstall

import "strings"

const (
	installerRPCVersionV1 = "aurora.installer.rpc.v1"
	bundleSchemaVersionV1 = "aurora.installer.bundle.v1"
)

type agentArtifactCapabilities struct {
	Install           bool `json:"install"`
	Restart           bool `json:"restart"`
	Uninstall         bool `json:"uninstall"`
	Migration         bool `json:"migration"`
	AdminRPCBootstrap bool `json:"admin_rpc_bootstrap"`
	NginxIntegration  bool `json:"nginx_integration"`
}

func defaultAgentArtifactCapabilities() agentArtifactCapabilities {
	return agentArtifactCapabilities{
		Install:   true,
		Restart:   true,
		Uninstall: true,
	}
}

func normalizeAgentInstallerAPIVersion(raw string) string {
	switch strings.TrimSpace(raw) {
	case "", installerRPCVersionV1:
		return installerRPCVersionV1
	default:
		return ""
	}
}

func normalizeAgentBundleSchemaVersion(raw string) string {
	switch strings.TrimSpace(raw) {
	case "", bundleSchemaVersionV1, "v1":
		return bundleSchemaVersionV1
	default:
		return ""
	}
}

func normalizeAgentArtifactCapabilities(caps agentArtifactCapabilities) agentArtifactCapabilities {
	if !caps.Install && !caps.Restart && !caps.Uninstall && !caps.Migration && !caps.AdminRPCBootstrap && !caps.NginxIntegration {
		return defaultAgentArtifactCapabilities()
	}
	return caps
}
