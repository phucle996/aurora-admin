package moduleinstall

import (
	keycfg "admin/internal/key"
	"admin/internal/repository"
	"admin/pkg/errorvar"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	ModuleInstallScopeLocal  = "local"
	ModuleInstallScopeRemote = "remote"

	schemaDigitsCount = 10
)

type ModuleInstallRequest struct {
	ModuleName     string
	Scope          string
	AppHost        string
	AppPort        int32
	Endpoint       string
	InstallCommand string

	SSHHost               string
	SSHPort               int32
	SSHUsername           string
	SSHPassword           *string
	SSHPrivateKey         *string
	SSHHostKeyFingerprint *string
}

type ModuleInstallResult struct {
	ModuleName      string
	Scope           string
	Endpoint        string
	EndpointValue   string
	InstallExecuted bool
	InstallOutput   string
	InstallExitCode int
	HostsUpdated    []string
	Warnings        []string

	SchemaKey       string
	SchemaName      string
	MigrationFiles  []string
	MigrationSource string

	HealthcheckPassed bool
	HealthcheckOutput string
}

type moduleInstallTarget struct {
	Scope              string
	Username           string
	Host               string
	Port               int32
	Password           *string
	PrivateKey         *string
	HostKeyFingerprint *string
}

type hostsEntry struct {
	Address string
	Host    string
}

type moduleMigrationSource struct {
	DownloadURLs []string
	LegacySchema string
}

var moduleMigrationSources = map[string]moduleMigrationSource{
	"vm": {
		DownloadURLs: []string{
			"https://codeload.github.com/phucle996/aurora-vm-service/zip/refs/heads/main",
		},
	},
	"ums": {
		DownloadURLs: []string{
			"https://codeload.github.com/phucle996/aurora-ums/zip/refs/heads/main",
		},
		LegacySchema: "ums",
	},
	"mail": {
		DownloadURLs: []string{
			"https://codeload.github.com/phucle996/aurora-mail-service/zip/refs/heads/main",
		},
	},
	"platform": {
		DownloadURLs: []string{
			"https://codeload.github.com/phucle996/aurora-platform-resource/zip/refs/heads/main",
		},
	},
}

type ModuleInstallService struct {
	endpointRepo repository.EndpointRepository
	runtimeRepo  repository.RuntimeConfigRepository
	databaseURL  string

	umsInstallScriptURL      string
	platformInstallScriptURL string
}

type InstallLogFn func(stage, message string)

func NewModuleInstallService(
	endpointRepo repository.EndpointRepository,
	runtimeRepo repository.RuntimeConfigRepository,
	databaseURL string,
	umsInstallScriptURL string,
	platformInstallScriptURL string,
) *ModuleInstallService {
	return &ModuleInstallService{
		endpointRepo:             endpointRepo,
		runtimeRepo:              runtimeRepo,
		databaseURL:              strings.TrimSpace(databaseURL),
		umsInstallScriptURL:      strings.TrimSpace(umsInstallScriptURL),
		platformInstallScriptURL: strings.TrimSpace(platformInstallScriptURL),
	}
}

func (s *ModuleInstallService) InstallWithLog(ctx context.Context, req ModuleInstallRequest, logFn InstallLogFn) (result *ModuleInstallResult, err error) {
	if s == nil || s.endpointRepo == nil || s.runtimeRepo == nil {
		return nil, errorvar.ErrModuleInstallServiceNil
	}

	moduleName := canonicalModuleName(req.ModuleName)
	if moduleName == "" || strings.Contains(moduleName, "admin") {
		return nil, errorvar.ErrModuleNameInvalid
	}

	scope := normalizeScope(req.Scope)
	if scope != ModuleInstallScopeLocal && scope != ModuleInstallScopeRemote {
		return nil, errorvar.ErrModuleInstallScope
	}
	if scope == ModuleInstallScopeLocal && strings.TrimSpace(req.InstallCommand) != "" {
		return nil, errorvar.ErrModuleInstallCommand
	}

	appHost := strings.TrimSpace(req.AppHost)
	if appHost == "" {
		return nil, fmt.Errorf("app_host is required")
	}
	endpoint, endpointPort, err := resolveInstallEndpoint(scope, appHost, req.AppPort, req.Endpoint)
	if err != nil {
		return nil, err
	}

	target, err := buildInstallTarget(scope, req)
	if err != nil {
		return nil, err
	}
	logInstall(logFn, "install", "start module=%s scope=%s app_host=%s app_port=%d endpoint=%s", moduleName, scope, appHost, endpointPort, endpoint)
	logInstall(logFn, "target", "target host=%s port=%d user=%s", target.Host, target.Port, target.Username)

	result = &ModuleInstallResult{
		ModuleName: moduleName,
		Scope:      scope,
		Endpoint:   endpoint,
	}

	rollbackSchemaName := ""
	endpointUpserted := false
	defer func() {
		if err == nil {
			return
		}
		if strings.TrimSpace(rollbackSchemaName) != "" {
			logInstall(logFn, "rollback", "rollback schema start schema=%s", rollbackSchemaName)
			rollbackCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			dropErr := dropSchemaWithSQL(rollbackCtx, s.databaseURL, rollbackSchemaName)
			cancel()
			if dropErr != nil {
				logInstall(logFn, "rollback", "[error] rollback schema failed schema=%s", rollbackSchemaName)
				err = fmt.Errorf("%w; rollback schema %s failed: %v", err, rollbackSchemaName, dropErr)
			} else {
				logInstall(logFn, "rollback", "rollback schema completed schema=%s", rollbackSchemaName)
			}

			schemaKey := strings.TrimSpace(result.SchemaKey)
			if schemaKey != "" && s.runtimeRepo != nil {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				deleteErr := s.runtimeRepo.Delete(cleanupCtx, schemaKey)
				cancel()
				if deleteErr != nil {
					logInstall(logFn, "rollback", "[warn] rollback schema key cleanup failed key=%s err=%v", schemaKey, deleteErr)
					err = fmt.Errorf("%w; schema key cleanup failed (%s): %v", err, schemaKey, deleteErr)
					return
				}
				logInstall(logFn, "rollback", "schema key cleanup completed key=%s", schemaKey)
			}
		}
		if endpointUpserted && s.endpointRepo != nil {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			deleteErr := s.endpointRepo.Delete(cleanupCtx, moduleName)
			cancel()
			if deleteErr != nil {
				logInstall(logFn, "rollback", "[warn] endpoint cleanup failed key=%s err=%v", keycfg.EndpointKey(moduleName), deleteErr)
				err = fmt.Errorf("%w; endpoint cleanup failed (%s): %v", err, moduleName, deleteErr)
				return
			}
			logInstall(logFn, "rollback", "endpoint cleanup completed key=%s", keycfg.EndpointKey(moduleName))
		}
	}()

	if err := s.prepareSchemaAndMigrate(ctx, moduleName, result, logFn); err != nil {
		rollbackSchemaName = strings.TrimSpace(result.SchemaName)
		logInstall(logFn, "migration", "[error] %v", err)
		return nil, err
	}
	rollbackSchemaName = strings.TrimSpace(result.SchemaName)
	if rollbackSchemaName != "" {
		logInstall(logFn, "migration", "schema prepared key=%s schema=%s", result.SchemaKey, rollbackSchemaName)
	}

	adminRPCEndpoint := ""
	if moduleName == "platform" {
		adminRPCEndpoint, err = s.resolveAdminBootstrapEndpoint(ctx)
		if err != nil {
			return nil, err
		}
	}

	command := buildDefaultModuleInstallCommand(
		moduleName,
		result.SchemaName,
		appHost,
		endpoint,
		s.databaseURL,
		adminRPCEndpoint,
		s.umsInstallScriptURL,
		s.platformInstallScriptURL,
	)
	if scope == ModuleInstallScopeRemote && strings.TrimSpace(req.InstallCommand) != "" {
		command = strings.TrimSpace(req.InstallCommand)
		logInstall(logFn, "install", "using custom install command for remote target")
	} else if command != "" {
		logInstall(logFn, "install", "resolved default install command for module=%s", moduleName)
	}
	if command == "" {
		return nil, errorvar.ErrModuleInstallerMissing
	}

	if tlsErr := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, logFn); tlsErr != nil {
		logInstall(logFn, "tls", "[error] %v", tlsErr)
		return nil, fmt.Errorf("install tls materials failed: %w", tlsErr)
	}

	if adminHostEntry, ok := s.resolveAdminHostsEntry(ctx); ok {
		logInstall(logFn, "hosts", "pre-sync admin host %s -> %s", adminHostEntry.Host, adminHostEntry.Address)
		_, warnings := syncHostsForTargets(ctx, []hostsEntry{adminHostEntry}, []moduleInstallTarget{target})
		for _, warning := range warnings {
			logInstall(logFn, "hosts", "[warn] %s", warning)
		}
	}

	endpointValue := encodeEndpointValue(target, endpoint)
	result.EndpointValue = endpointValue
	logInstall(logFn, "endpoint", "upsert endpoint key=%s", keycfg.EndpointKey(moduleName))
	if err := s.endpointRepo.Upsert(ctx, moduleName, endpointValue); err != nil {
		logInstall(logFn, "endpoint", "[error] %v", err)
		return nil, err
	}
	endpointUpserted = true
	logInstall(logFn, "endpoint", "endpoint updated")

	logInstall(logFn, "install", "running install command")
	output, exitCode, installErr := runInstallCommand(ctx, command, target, func(line string) {
		logInstall(logFn, "ssh", "%s", line)
	})
	result.InstallExecuted = true
	result.InstallOutput = strings.TrimSpace(output)
	result.InstallExitCode = exitCode
	if installErr != nil {
		logInstall(logFn, "install", "[error] install command failed exit_code=%d", exitCode)
		return nil, fmt.Errorf("install command failed: %w", installErr)
	}
	logInstall(logFn, "install", "install command completed exit_code=%d", exitCode)

	logInstall(logFn, "healthcheck", "checking endpoint health")
	for _, candidate := range buildHealthcheckCandidates(endpoint) {
		logInstall(logFn, "healthcheck", "candidate=%s", candidate)
	}
	healthcheckOutput, healthErr := ensureCurlAndCheckEndpoint(ctx, target, moduleName, endpoint, func(line string) {
		logInstall(logFn, "healthcheck", "%s", line)
	})
	result.HealthcheckOutput = strings.TrimSpace(healthcheckOutput)
	if healthErr != nil {
		logInstall(logFn, "healthcheck", "[error] %v", healthErr)
		return nil, fmt.Errorf("service healthcheck failed: %w", healthErr)
	}
	result.HealthcheckPassed = true
	logInstall(logFn, "healthcheck", "healthcheck passed")

	targets, err := s.resolveHostSyncTargets(ctx, target)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("cannot load existing endpoint targets: %v", err))
		targets = []moduleInstallTarget{target}
		logInstall(logFn, "hosts", "[warn] cannot load existing endpoint targets: %v", err)
	}

	hostEntries, entryErr := s.buildHostsEntries(ctx, appHost, target.Host)
	if entryErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("cannot build hosts entries: %v", entryErr))
		hostEntries = []hostsEntry{{Address: normalizeAddress(target.Host), Host: strings.TrimSpace(appHost)}}
		logInstall(logFn, "hosts", "[warn] cannot build hosts entries: %v", entryErr)
	}
	logInstall(logFn, "hosts", "sync /etc/hosts entries=%d targets=%d", len(hostEntries), len(targets))

	hostsUpdated, warnings := syncHostsForTargets(ctx, hostEntries, targets)
	result.HostsUpdated = hostsUpdated
	result.Warnings = append(result.Warnings, warnings...)
	if len(hostsUpdated) > 0 {
		logInstall(logFn, "hosts", "hosts sync updated=%s", strings.Join(hostsUpdated, ","))
	}
	for _, warning := range warnings {
		logInstall(logFn, "hosts", "[warn] %s", warning)
	}

	logInstall(logFn, "install", "[done] module install completed module=%s", moduleName)
	rollbackSchemaName = ""
	return result, nil
}

func (s *ModuleInstallService) prepareSchemaAndMigrate(
	ctx context.Context,
	moduleName string,
	result *ModuleInstallResult,
	logFn InstallLogFn,
) (err error) {
	source, ok := moduleMigrationSources[moduleName]
	if !ok {
		logInstall(logFn, "migration", "skip migrations for module=%s (no source)", moduleName)
		return nil
	}
	if strings.TrimSpace(s.databaseURL) == "" {
		return fmt.Errorf("database_url is empty for module %s", moduleName)
	}

	migrationsDir, migrationSource, cleanup, err := materializeMigrations(ctx, moduleName, source)
	if err != nil {
		return fmt.Errorf("prepare migrations for %s failed: %w", moduleName, err)
	}
	defer cleanup()
	logInstall(logFn, "migration", "migration source=%s", migrationSource)

	schemaName, err := generateSchemaName(moduleName)
	if err != nil {
		return fmt.Errorf("generate schema for %s failed: %w", moduleName, err)
	}

	if err := createSchemaWithSQL(ctx, s.databaseURL, schemaName); err != nil {
		return fmt.Errorf("create schema failed: %w", err)
	}
	logInstall(logFn, "migration", "created schema=%s", schemaName)

	schemaKey := keycfg.RuntimeSchemaKey(moduleName)
	result.SchemaKey = schemaKey
	result.SchemaName = schemaName

	if err := s.runtimeRepo.Upsert(ctx, schemaKey, schemaName); err != nil {
		return fmt.Errorf("seed schema key %s failed: %w", schemaKey, err)
	}
	logInstall(logFn, "migration", "seeded runtime config key=%s", schemaKey)

	migrationFiles, err := listMigrationUpFiles(migrationsDir)
	if err != nil {
		return err
	}

	rewrittenFiles, rewriteCleanup, rewritten, err := rewriteMigrationFilesForSchema(migrationFiles, source.LegacySchema, schemaName)
	if err != nil {
		return err
	}
	defer rewriteCleanup()
	if rewritten {
		logInstall(logFn, "migration", "rewritten migration schema legacy=%s target=%s", source.LegacySchema, schemaName)
		migrationFiles = rewrittenFiles
	}

	logInstall(logFn, "migration", "apply migrations count=%d", len(migrationFiles))
	if err := runMigrationFilesWithSQL(ctx, s.databaseURL, schemaName, migrationFiles, logFn); err != nil {
		return err
	}

	result.MigrationFiles = make([]string, 0, len(migrationFiles))
	for _, file := range migrationFiles {
		result.MigrationFiles = append(result.MigrationFiles, filepath.Base(file))
	}
	result.MigrationSource = migrationSource
	logInstall(logFn, "migration", "migrations applied")
	return nil
}
