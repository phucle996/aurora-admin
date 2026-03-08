package moduleinstall

import (
	keycfg "admin/internal/key"
	"admin/internal/repository"
	"admin/pkg/errorvar"
	"context"
	"errors"
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

type endpointListSnapshot struct {
	items []repository.EndpointKV
	err   error
}

type ModuleInstallService struct {
	endpointRepo repository.EndpointRepository
	runtimeRepo  repository.RuntimeConfigRepository
	databaseURL  string

	umsInstallScriptURL      string
	platformInstallScriptURL string
	paasInstallScriptURL     string
	dbaasInstallScriptURL    string
	uiInstallScriptURL       string
}

type InstallLogFn func(stage, message string)

func NewModuleInstallService(
	endpointRepo repository.EndpointRepository,
	runtimeRepo repository.RuntimeConfigRepository,
	databaseURL string,
	umsInstallScriptURL string,
	platformInstallScriptURL string,
	paasInstallScriptURL string,
	dbaasInstallScriptURL string,
	uiInstallScriptURL string,
) *ModuleInstallService {
	return &ModuleInstallService{
		endpointRepo:             endpointRepo,
		runtimeRepo:              runtimeRepo,
		databaseURL:              strings.TrimSpace(databaseURL),
		umsInstallScriptURL:      strings.TrimSpace(umsInstallScriptURL),
		platformInstallScriptURL: strings.TrimSpace(platformInstallScriptURL),
		paasInstallScriptURL:     strings.TrimSpace(paasInstallScriptURL),
		dbaasInstallScriptURL:    strings.TrimSpace(dbaasInstallScriptURL),
		uiInstallScriptURL:       strings.TrimSpace(uiInstallScriptURL),
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
	if scope != ModuleInstallScopeRemote {
		return nil, errorvar.ErrModuleInstallScope
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
	if preflightErr := ensureTargetInstallPrivilege(ctx, target, logFn); preflightErr != nil {
		logInstall(logFn, "preflight", "[error] %v", preflightErr)
		return nil, preflightErr
	}

	result = &ModuleInstallResult{
		ModuleName: moduleName,
		Scope:      scope,
		Endpoint:   endpoint,
	}

	rollbacks := newRollbackStack(logFn)
	defer func() {
		if err == nil {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if rollbackErr := rollbacks.Run(rollbackCtx); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()

	if prepErr := s.prepareSchemaAndMigrate(ctx, moduleName, result, logFn); prepErr != nil {
		s.addSchemaRollbackStep(rollbacks, result)
		logInstall(logFn, "migration", "[error] %v", prepErr)
		return nil, prepErr
	}
	s.addSchemaRollbackStep(rollbacks, result)
	if schemaName := strings.TrimSpace(result.SchemaName); schemaName != "" {
		logInstall(logFn, "migration", "schema prepared key=%s schema=%s", result.SchemaKey, schemaName)
	}
	endpoints := s.loadEndpointListSnapshot(ctx)

	adminRPCEndpoint := ""
	if moduleRequiresAdminRPC(moduleName) {
		adminRPCEndpoint, err = s.resolveAdminBootstrapEndpoint(endpoints.items, endpoints.err)
		if err != nil {
			return nil, err
		}
	}

	command := ""
	if scope == ModuleInstallScopeRemote && strings.TrimSpace(req.InstallCommand) != "" {
		command = strings.TrimSpace(req.InstallCommand)
		logInstall(logFn, "install", "using custom install command for remote target")
	} else {
		uiEnvPath := ""
		if moduleName == "ui" {
			envRemotePath, envErr := s.generateAndPushUIEnv(ctx, target, endpoints.items, endpoints.err, logFn)
			if envErr != nil {
				return nil, envErr
			}
			uiEnvPath = envRemotePath
		}
		command = s.buildDefaultInstallCommand(moduleName, appHost, endpointPort, adminRPCEndpoint, uiEnvPath, target.Password)
		if command != "" {
			logInstall(logFn, "install", "resolved default install command for module=%s", moduleName)
		}
	}
	if command == "" {
		return nil, errorvar.ErrModuleInstallerMissing
	}

	if tlsErr := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, logFn); tlsErr != nil {
		logInstall(logFn, "tls", "[error] %v", tlsErr)
		return nil, fmt.Errorf("install tls materials failed: %w", tlsErr)
	}

	if adminHostEntry, ok := s.resolveAdminHostsEntry(endpoints.items, endpoints.err); ok {
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
	rollbacks.Add("endpoint", func(rollbackCtx context.Context) error {
		if s.endpointRepo == nil {
			return nil
		}
		cleanupCtx, cancel := context.WithTimeout(rollbackCtx, 10*time.Second)
		deleteErr := s.endpointRepo.Delete(cleanupCtx, moduleName)
		cancel()
		if deleteErr != nil {
			return fmt.Errorf("endpoint cleanup failed (%s): %w", moduleName, deleteErr)
		}
		return nil
	})
	logInstall(logFn, "endpoint", "endpoint updated")

	logInstall(logFn, "install", "running install command")
	output, exitCode, installErr := runInstallCommand(ctx, command, target, func(line string) {
		logInstall(logFn, "ssh", "%s", line)
	})
	result.InstallExecuted = true
	result.InstallOutput = strings.TrimSpace(output)
	result.InstallExitCode = exitCode
	if installErr != nil || exitCode != 0 {
		logInstall(logFn, "install", "[error] install command failed exit_code=%d", exitCode)
		if installErr != nil {
			return nil, fmt.Errorf("install command failed: %w", installErr)
		}
		return nil, fmt.Errorf("install command failed: exit_code=%d", exitCode)
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

	targets, err := s.resolveHostSyncTargets(target, endpoints.items, endpoints.err)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("cannot load existing endpoint targets: %v", err))
		targets = []moduleInstallTarget{target}
		logInstall(logFn, "hosts", "[warn] cannot load existing endpoint targets: %v", err)
	}

	hostEntries, entryErr := s.buildHostsEntries(appHost, target.Host, endpoints.items, endpoints.err)
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
	rollbacks.Clear()
	return result, nil
}

func (s *ModuleInstallService) buildDefaultInstallCommand(
	moduleName string,
	appHost string,
	appPort int32,
	adminRPCEndpoint string,
	uiEnvPath string,
	sudoPassword *string,
) string {
	scriptURL := s.installScriptURL(moduleName)
	return buildDefaultModuleInstallCommand(
		moduleName,
		scriptURL,
		appHost,
		appPort,
		adminRPCEndpoint,
		uiEnvPath,
		sudoPassword,
	)
}

func (s *ModuleInstallService) loadEndpointListSnapshot(ctx context.Context) endpointListSnapshot {
	if s == nil || s.endpointRepo == nil {
		return endpointListSnapshot{err: fmt.Errorf("module install service is nil")}
	}
	items, err := s.endpointRepo.List(ctx)
	return endpointListSnapshot{
		items: items,
		err:   err,
	}
}

func (s *ModuleInstallService) addSchemaRollbackStep(stack *rollbackStack, result *ModuleInstallResult) {
	if stack == nil || result == nil {
		return
	}
	schemaName := strings.TrimSpace(result.SchemaName)
	if schemaName == "" {
		return
	}
	schemaKey := strings.TrimSpace(result.SchemaKey)
	stack.Add("schema", func(rollbackCtx context.Context) error {
		dropCtx, cancel := context.WithTimeout(rollbackCtx, 30*time.Second)
		dropErr := dropSchemaWithSQL(dropCtx, s.databaseURL, schemaName)
		cancel()
		if dropErr != nil {
			return fmt.Errorf("rollback schema %s failed: %w", schemaName, dropErr)
		}
		if schemaKey == "" || s.runtimeRepo == nil {
			return nil
		}

		cleanupCtx, cleanupCancel := context.WithTimeout(rollbackCtx, 10*time.Second)
		deleteErr := s.runtimeRepo.Delete(cleanupCtx, schemaKey)
		cleanupCancel()
		if deleteErr != nil {
			return fmt.Errorf("schema key cleanup failed (%s): %w", schemaKey, deleteErr)
		}
		return nil
	})
}

func (s *ModuleInstallService) installScriptURL(moduleName string) string {
	switch canonicalModuleName(moduleName) {
	case "ums":
		return strings.TrimSpace(s.umsInstallScriptURL)
	case "platform":
		return strings.TrimSpace(s.platformInstallScriptURL)
	case "paas":
		return strings.TrimSpace(s.paasInstallScriptURL)
	case "dbaas":
		return strings.TrimSpace(s.dbaasInstallScriptURL)
	case "ui":
		return strings.TrimSpace(s.uiInstallScriptURL)
	default:
		return ""
	}
}

func (s *ModuleInstallService) prepareSchemaAndMigrate(
	ctx context.Context,
	moduleName string,
	result *ModuleInstallResult,
	logFn InstallLogFn,
) (err error) {
	source, ok := moduleMigrationSourceFor(moduleName)
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
