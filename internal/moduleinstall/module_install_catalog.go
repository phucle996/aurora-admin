package moduleinstall

import "strings"

type moduleMigrationSource struct {
	DownloadURLs []string
	LegacySchema string
}

type moduleInstallDefinition struct {
	Name            string
	Aliases         []string
	MigrationSource *moduleMigrationSource
	RequireAdminRPC bool
	ArtifactSource  *moduleArtifactSource
}

type moduleArtifactSource struct {
	RepoSlug        string
	BundleAssetBase string
}

var moduleInstallDefinitions = []moduleInstallDefinition{
	{
		Name: "admin",
		Aliases: []string{
			"admin", "aurora-admin", "aurora_admin",
		},
	},
	{
		Name: "vm",
		Aliases: []string{
			"vm", "vm-service", "vm_service", "kvm", "hypervisor", "libvirt", "aurora-vm",
		},
		MigrationSource: &moduleMigrationSource{
			DownloadURLs: []string{
				"https://codeload.github.com/phucle996/aurora-vm-service/zip/refs/heads/main",
			},
		},
	},
	{
		Name: "ums",
		Aliases: []string{
			"ums", "usermanagment", "user-management", "user-management-system", "user-management-service", "user", "aurora-ums",
		},
		MigrationSource: &moduleMigrationSource{
			DownloadURLs: []string{
				"https://codeload.github.com/phucle996/aurora-ums/zip/refs/heads/main",
			},
			LegacySchema: "ums",
		},
		ArtifactSource: &moduleArtifactSource{
			RepoSlug:        "phucle996/aurora-ums",
			BundleAssetBase: "aurora-ums",
		},
	},
	{
		Name: "mail",
		Aliases: []string{
			"mail", "mail-service", "mail_service", "smtp", "aurora-mail",
		},
		MigrationSource: &moduleMigrationSource{
			DownloadURLs: []string{
				"https://codeload.github.com/phucle996/aurora-mail-service/zip/refs/heads/main",
			},
		},
	},
	{
		Name: "platform",
		Aliases: []string{
			"platform", "platform-resource", "platform_resource", "plaform-resource", "plaform_resource", "aurora-platform-resource",
		},
		MigrationSource: &moduleMigrationSource{
			DownloadURLs: []string{
				"https://codeload.github.com/phucle996/aurora-platform-resource/zip/refs/heads/main",
			},
		},
		RequireAdminRPC: true,
		ArtifactSource: &moduleArtifactSource{
			RepoSlug:        "phucle996/aurora-platform-resource",
			BundleAssetBase: "aurora-platform-resource",
		},
	},
	{
		Name: "paas",
		Aliases: []string{
			"paas", "paas-service", "paas_service", "aurora-paas",
		},
		MigrationSource: &moduleMigrationSource{
			DownloadURLs: []string{
				"https://codeload.github.com/phucle996/aurora-paas-service/zip/refs/heads/main",
			},
		},
		RequireAdminRPC: true,
		ArtifactSource: &moduleArtifactSource{
			RepoSlug:        "phucle996/aurora-paas-service",
			BundleAssetBase: "aurora-paas-service",
		},
	},
	{
		Name: "dbaas",
		Aliases: []string{
			"dbaas", "dbaas-service", "dbaas_service", "dbaas-module", "dbaas_module", "aurora-dbaas",
		},
		MigrationSource: &moduleMigrationSource{
			DownloadURLs: []string{
				"https://codeload.github.com/phucle996/aurora-dbaas-module/zip/refs/heads/main",
			},
		},
		RequireAdminRPC: true,
		ArtifactSource: &moduleArtifactSource{
			RepoSlug:        "phucle996/aurora-dbaas-module",
			BundleAssetBase: "aurora-dbaas-service",
		},
	},
	{
		Name: "ui",
		Aliases: []string{
			"ui", "aurora-ui", "aurora_ui", "frontend", "web", "dashboard-ui", "dashboard_ui",
		},
	},
}

var (
	moduleCanonicalByAlias = buildModuleAliasMap(moduleInstallDefinitions)
	moduleDefinitionByName = buildModuleDefinitionMap(moduleInstallDefinitions)
)

func buildModuleAliasMap(definitions []moduleInstallDefinition) map[string]string {
	out := map[string]string{}
	for _, definition := range definitions {
		canonical := strings.TrimSpace(definition.Name)
		if canonical == "" {
			continue
		}
		out[canonical] = canonical
		for _, alias := range definition.Aliases {
			normalized := normalizeModuleName(alias)
			if normalized == "" {
				continue
			}
			out[normalized] = canonical
		}
	}
	return out
}

func buildModuleDefinitionMap(definitions []moduleInstallDefinition) map[string]moduleInstallDefinition {
	out := map[string]moduleInstallDefinition{}
	for _, definition := range definitions {
		canonical := strings.TrimSpace(definition.Name)
		if canonical == "" {
			continue
		}
		out[canonical] = definition
	}
	return out
}

func canonicalModuleName(raw string) string {
	name := normalizeModuleName(raw)
	if name == "" {
		return ""
	}
	if canonical, ok := moduleCanonicalByAlias[name]; ok {
		return canonical
	}
	return name
}

func moduleMigrationSourceFor(moduleName string) (moduleMigrationSource, bool) {
	definition, ok := moduleDefinitionByName[canonicalModuleName(moduleName)]
	if !ok || definition.MigrationSource == nil {
		return moduleMigrationSource{}, false
	}
	return *definition.MigrationSource, true
}

func moduleRequiresAdminRPC(moduleName string) bool {
	definition, ok := moduleDefinitionByName[canonicalModuleName(moduleName)]
	return ok && definition.RequireAdminRPC
}

func moduleArtifactSourceFor(moduleName string) (moduleArtifactSource, bool) {
	definition, ok := moduleDefinitionByName[canonicalModuleName(moduleName)]
	if !ok || definition.ArtifactSource == nil {
		return moduleArtifactSource{}, false
	}
	return *definition.ArtifactSource, true
}
