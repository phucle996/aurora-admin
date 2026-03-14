package moduleinstall

import (
	keycfg "admin/internal/key"
	"admin/internal/repository"
	runtimerepo "admin/internal/runtime/repository"
	"admin/pkg/errorvar"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	ModuleInstallScopeRemote = "remote"
	ModuleInstallRuntimeName = "linux-systemd"
	schemaDigitsCount        = 10
)

type ModuleInstallRequest struct {
	ModuleName string
	AgentID    string
	AppHost    string
}

type ModuleInstallResult struct {
	ModuleName       string
	OperationID      string
	AgentID          string
	Version          string
	ArtifactChecksum string
	ServiceName      string
	Endpoint         string
	Health           string
	HostsUpdated     []string
	Warnings         []string

	SchemaKey  string
	SchemaName string
}

type moduleInstallTarget struct {
	AgentID           string
	AgentGRPCEndpoint string
	Architecture      string
	Host              string
}

type hostsEntry struct {
	Address string
	Host    string
}

type ModuleInstallService struct {
	endpointRepo    repository.EndpointRepository
	runtimeRepo     runtimerepo.RuntimeConfigRepository
	certStoreRepo   repository.CertStoreRepository
	certStorePrefix string
	databaseURL     string

	agentRPCCAPath         string
	agentRPCClientCertPath string
	agentRPCClientKeyPath  string

	uiLegacyInstallScriptURL string
}

type InstallLogFn func(stage, message string)

func NewModuleInstallService(
	endpointRepo repository.EndpointRepository,
	runtimeRepo runtimerepo.RuntimeConfigRepository,
	certStoreRepo repository.CertStoreRepository,
	certStorePrefix string,
	databaseURL string,
	agentRPCCAPath string,
	agentRPCClientCertPath string,
	agentRPCClientKeyPath string,
	uiLegacyInstallScriptURL string,
) *ModuleInstallService {
	svc := &ModuleInstallService{
		endpointRepo:             endpointRepo,
		runtimeRepo:              runtimeRepo,
		certStoreRepo:            certStoreRepo,
		certStorePrefix:          strings.TrimSpace(certStorePrefix),
		databaseURL:              strings.TrimSpace(databaseURL),
		agentRPCCAPath:           strings.TrimSpace(agentRPCCAPath),
		agentRPCClientCertPath:   strings.TrimSpace(agentRPCClientCertPath),
		agentRPCClientKeyPath:    strings.TrimSpace(agentRPCClientKeyPath),
		uiLegacyInstallScriptURL: strings.TrimSpace(uiLegacyInstallScriptURL),
	}
	configureAgentRPCDialTLS(svc.agentRPCCAPath, svc.agentRPCClientCertPath, svc.agentRPCClientKeyPath)
	return svc
}

func (s *ModuleInstallService) InstallWithLog(ctx context.Context, req ModuleInstallRequest, logFn InstallLogFn) (result *ModuleInstallResult, err error) {
	if s == nil || s.endpointRepo == nil || s.runtimeRepo == nil || s.certStoreRepo == nil {
		return nil, errorvar.ErrModuleInstallServiceNil
	}

	moduleName := canonicalModuleName(req.ModuleName)
	if moduleName == "" || strings.Contains(moduleName, "admin") {
		return nil, errorvar.ErrModuleNameInvalid
	}

	appHost := strings.TrimSpace(req.AppHost)
	if appHost == "" {
		return nil, fmt.Errorf("app_host is required")
	}
	target, err := s.resolveInstallTargetFromAgent(ctx, req.AgentID, logFn)
	if err != nil {
		return nil, err
	}
	endpoint, endpointPort, err := resolveInstallEndpoint(appHost)
	if err != nil {
		return nil, err
	}
	operationTracker, opErr := s.beginInstallOperation(ctx, target, moduleName, appHost, endpoint)
	if opErr != nil {
		return nil, opErr
	}
	observedLogFn := wrapInstallLogFn(logFn, operationTracker)
	logInstall(observedLogFn, "install", "start module=%s runtime=%s app_host=%s app_port=%d endpoint=%s", moduleName, ModuleInstallRuntimeName, appHost, endpointPort, endpoint)
	logInstall(observedLogFn, "target", "target host=%s", target.Host)
	resolvedPort, portErr := resolveInstallPortForTarget(ctx, target, endpointPort)
	if portErr != nil {
		logInstall(observedLogFn, "install", "[error] %v", portErr)
		return nil, portErr
	}
	if resolvedPort != endpointPort {
		logInstall(observedLogFn, "install", "auto-selected available app_port=%d (previous=%d busy)", resolvedPort, endpointPort)
		endpointPort = resolvedPort
	} else {
		logInstall(observedLogFn, "install", "confirmed app_port=%d is available on target", endpointPort)
	}

	result = &ModuleInstallResult{
		ModuleName:  moduleName,
		OperationID: operationTrackerID(operationTracker),
		AgentID:     strings.TrimSpace(target.AgentID),
		Endpoint:    endpoint,
	}

	useAgentBundleInstall := shouldUseAgentBundleInstall(moduleName, target)
	var plannedBundleRelease *moduleBundleRelease
	if useAgentBundleInstall {
		plannedBundleRelease, err = s.resolveLatestModuleBundleRelease(ctx, moduleName, target.Architecture)
		if err != nil {
			return nil, err
		}
	}

	stateSource := "legacy-script"
	if useAgentBundleInstall {
		stateSource = "agent-bundle"
	}
	bundleVersion := ""
	bundleArtifactURL := ""
	bundleArtifactChecksum := ""
	if plannedBundleRelease != nil {
		bundleVersion = strings.TrimSpace(plannedBundleRelease.Version)
		bundleArtifactURL = strings.TrimSpace(plannedBundleRelease.ArtifactURL)
		bundleArtifactChecksum = strings.TrimSpace(plannedBundleRelease.ArtifactChecksum)
	}
	result.ArtifactChecksum = bundleArtifactChecksum
	if operationTracker != nil {
		opCtx, cancel := backgroundOperationWriteContext()
		operationTracker.SetInstallPlan(opCtx, target.AgentID, bundleVersion, bundleArtifactChecksum)
		cancel()
	}
	stateHandle, stateErr := s.beginInstallStateTracking(
		ctx,
		target,
		moduleName,
		ModuleInstallRuntimeName,
		appHost,
		endpointPort,
		endpoint,
		bundleVersion,
		bundleArtifactURL,
		bundleArtifactChecksum,
		stateSource,
	)
	if stateErr != nil {
		return nil, stateErr
	}

	rollbacks := newRollbackStack(observedLogFn)
	defer func() {
		if err != nil {
			s.markInstallStateFailed(context.Background(), stateHandle, err)
			if operationTracker != nil {
				opCtx, cancel := backgroundOperationWriteContext()
				operationTracker.Fail(opCtx, err)
				cancel()
			}
		}
		if err == nil && operationTracker != nil {
			opCtx, cancel := backgroundOperationWriteContext()
			operationTracker.Complete(opCtx, result)
			cancel()
		}
		if err == nil {
			return
		}
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if rollbackErr := rollbacks.Run(rollbackCtx); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()

	if prepErr := s.prepareSchemaAndMigrate(ctx, moduleName, result, observedLogFn); prepErr != nil {
		s.addSchemaRollbackStep(rollbacks, result)
		logInstall(observedLogFn, "migration", "[error] %v", prepErr)
		return nil, prepErr
	}
	s.addSchemaRollbackStep(rollbacks, result)
	if schemaName := strings.TrimSpace(result.SchemaName); schemaName != "" {
		logInstall(observedLogFn, "migration", "schema prepared key=%s schema=%s", result.SchemaKey, schemaName)
	}
	endpointItems, endpointErr := s.loadEndpointList(ctx)

	adminRPCEndpoint := ""
	if moduleName == "ums" || moduleRequiresAdminRPC(moduleName) {
		adminRPCEndpoint, err = s.resolveAdminBootstrapEndpoint(ctx, endpointItems, endpointErr)
		if err != nil {
			return nil, err
		}
	}

	command := ""
	if !useAgentBundleInstall {
		uiEnvPath := ""
		if moduleName == "ui" {
			envRemotePath, envErr := s.generateAndPushUIEnv(ctx, target, endpointItems, endpointErr, observedLogFn)
			if envErr != nil {
				return nil, envErr
			}
			uiEnvPath = envRemotePath
		}
		command = s.buildDefaultInstallCommand(moduleName, appHost, endpointPort, uiEnvPath)
		if command != "" {
			logInstall(observedLogFn, "install", "resolved default install command for module=%s", moduleName)
		}
		if command == "" {
			return nil, errorvar.ErrModuleInstallerMissing
		}
	} else {
		logInstall(observedLogFn, "install", "using typed agent bundle install for module=%s", moduleName)
	}

	if preseedErr := s.preseedInstallRouting(ctx, moduleName, target, endpoint, endpointPort, rollbacks, observedLogFn); preseedErr != nil {
		return nil, preseedErr
	}

	var tlsBundle *moduleTLSBundle
	if useAgentBundleInstall {
		logInstall(observedLogFn, "install", "running typed bundle install via agent")
		agentRes, typedTLSBundle, typedErr := s.installModuleViaAgentBundle(
			ctx,
			target,
			moduleName,
			plannedBundleRelease,
			appHost,
			endpointPort,
			endpoint,
			adminRPCEndpoint,
			observedLogFn,
		)
		if typedTLSBundle != nil {
			tlsBundle = typedTLSBundle
		}
		if agentRes != nil && agentRes.Result != nil {
			result.Version = strings.TrimSpace(agentRes.Result.Version)
			result.ServiceName = strings.TrimSpace(agentRes.Result.ServiceName)
			if strings.TrimSpace(agentRes.Result.Endpoint) != "" {
				result.Endpoint = strings.TrimSpace(agentRes.Result.Endpoint)
			}
			result.Health = strings.TrimSpace(agentRes.Result.Health)
		}
		if typedErr != nil {
			logInstall(observedLogFn, "install", "[error] typed bundle install failed")
			return nil, fmt.Errorf("install module via agent bundle failed: %w", typedErr)
		}
		logInstall(observedLogFn, "install", "typed bundle install completed")
		if syncErr := s.syncActualInstallStateFromAgentInventory(ctx, target); syncErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("sync installed module inventory failed: %v", syncErr))
			logInstall(observedLogFn, "install", "[warn] sync installed module inventory failed: %v", syncErr)
		}
	} else {
		tlsLocalBundle, tlsErr := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, observedLogFn)
		if tlsErr != nil {
			logInstall(observedLogFn, "tls", "[error] %v", tlsErr)
			return nil, fmt.Errorf("install tls materials failed: %w", tlsErr)
		}
		tlsBundle = tlsLocalBundle

		logInstall(observedLogFn, "install", "running install command")
		_, exitCode, installErr := runInstallCommand(
			ctx,
			command,
			target,
			func(line string) {
				logInstall(observedLogFn, "agent", "%s", line)
			},
			func(line string) {
				logInstall(observedLogFn, "agent", "%s", line)
			},
		)
		if installErr != nil || exitCode != 0 {
			logInstall(observedLogFn, "install", "[error] install command failed exit_code=%d", exitCode)
			if installErr != nil {
				return nil, fmt.Errorf("install command failed: %w", installErr)
			}
			return nil, fmt.Errorf("install command failed: exit_code=%d", exitCode)
		}
		logInstall(observedLogFn, "install", "install command completed exit_code=%d", exitCode)
	}

	if !useAgentBundleInstall {
		tlsPresent, tlsCheckOutput, tlsCheckErr := moduleTLSExistsOnTarget(ctx, target, moduleName)
		if tlsCheckErr != nil {
			logInstall(observedLogFn, "tls", "[warn] cannot verify tls materials on target: %v", tlsCheckErr)
		} else if !tlsPresent {
			logInstall(observedLogFn, "tls", "[warn] tls materials missing after install, reinstalling: %s", strings.TrimSpace(tlsCheckOutput))
			repairedBundle, repairErr := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, observedLogFn)
			if repairErr != nil {
				logInstall(observedLogFn, "tls", "[error] tls self-heal failed: %v", repairErr)
				return nil, fmt.Errorf("tls materials missing after install: %w", repairErr)
			}
			tlsBundle = repairedBundle
			logInstall(observedLogFn, "tls", "tls self-heal completed")
		}
	}

	targetAddr := normalizeAddress(target.Host)
	if targetAddr == "" {
		targetAddr = "127.0.0.1"
	}
	hostEntryHost := strings.TrimSpace(appHost)
	if hostEntryHost == "" {
		return nil, fmt.Errorf("app_host is required")
	}
	hostEntries := []hostsEntry{
		{
			Address: targetAddr,
			Host:    hostEntryHost,
		},
	}
	logInstall(observedLogFn, "hosts", "sync app host /etc/hosts on target (required) host=%s address=%s target=%s", hostEntryHost, targetAddr, target.Host)

	hostsUpdated, hostWarnings := syncHostsForTargets(ctx, hostEntries, []moduleInstallTarget{target})
	result.HostsUpdated = hostsUpdated
	result.Warnings = append(result.Warnings, hostWarnings...)
	if len(hostsUpdated) == 0 {
		for _, warning := range hostWarnings {
			logInstall(observedLogFn, "hosts", "[error] %s", warning)
		}
		if len(hostWarnings) == 0 {
			logInstall(observedLogFn, "hosts", "[error] no host entries were updated on target")
		}
		return nil, fmt.Errorf("sync app host to /etc/hosts failed")
	}
	logInstall(observedLogFn, "hosts", "app host synced targets=%s", strings.Join(hostsUpdated, ","))
	for _, warning := range hostWarnings {
		logInstall(observedLogFn, "hosts", "[warn] %s", warning)
	}

	if err := s.seedHostRoutingEntry(ctx, hostEntryHost, targetAddr); err != nil {
		logInstall(observedLogFn, "hosts", "[error] %v", err)
		return nil, fmt.Errorf("seed host routing failed: %w", err)
	}
	hostRoutingKey := keycfg.RuntimeHostEntryKey(hostEntryHost)
	logInstall(observedLogFn, "hosts", "seeded host routing key=%s", hostRoutingKey)
	rollbacks.Add("hosts", func(rollbackCtx context.Context) error {
		deleteScript := buildHostsDeleteCommand(hostEntryHost)
		runCtx, cancel := context.WithTimeout(rollbackCtx, 20*time.Second)
		_, _, runErr := runCommandOnTarget(runCtx, target, deleteScript, 20*time.Second, nil, nil)
		cancel()
		if runErr != nil {
			return fmt.Errorf("rollback hosts file cleanup failed: %w", runErr)
		}

		if s.runtimeRepo == nil {
			return nil
		}
		deleteCtx, deleteCancel := context.WithTimeout(rollbackCtx, 10*time.Second)
		deleteErr := s.runtimeRepo.Delete(deleteCtx, hostRoutingKey)
		deleteCancel()
		if deleteErr != nil {
			return fmt.Errorf("rollback runtime host key cleanup failed: %w", deleteErr)
		}
		return nil
	})

	agentBroadcastUpdated, agentBroadcastWarnings := s.broadcastHostsToConnectedAgents(ctx, hostEntries, target.AgentID)
	for _, item := range agentBroadcastUpdated {
		result.HostsUpdated = append(result.HostsUpdated, "agent:"+item)
	}
	for _, warning := range agentBroadcastWarnings {
		result.Warnings = append(result.Warnings, warning)
		logInstall(observedLogFn, "hosts", "[warn] %s", warning)
	}

	if useAgentBundleInstall {
		logInstall(observedLogFn, "nginx", "nginx proxy managed by agent bundle installer")
	} else {
		if nginxErr := ensureModuleNginxProxyOnTarget(ctx, target, moduleName, appHost, endpointPort, observedLogFn); nginxErr != nil {
			logInstall(observedLogFn, "nginx", "[error] %v", nginxErr)
			return nil, fmt.Errorf("configure nginx proxy failed: %w", nginxErr)
		}
	}

	if seedErr := s.seedModuleTLSBundle(ctx, moduleName, tlsBundle); seedErr != nil {
		logInstall(observedLogFn, "tls", "[error] %v", seedErr)
		return nil, fmt.Errorf("seed module tls bundle failed: %w", seedErr)
	}
	s.addModuleTLSRollbackStep(rollbacks, moduleName)
	logInstall(observedLogFn, "tls", "seeded module tls bundle into cert store")

	logInstall(observedLogFn, "install", "[done] module install completed module=%s", moduleName)
	s.markInstallStateInstalled(ctx, stateHandle, result.Version, resolvedInstalledServiceName(result, moduleName), result.Endpoint, result.Health)
	rollbacks.Clear()
	return result, nil
}

func (s *ModuleInstallService) buildDefaultInstallCommand(
	moduleName string,
	appHost string,
	appPort int32,
	uiEnvPath string,
) string {
	if s == nil || canonicalModuleName(moduleName) != "ui" {
		return ""
	}
	return buildDefaultUIInstallCommand(
		s.uiLegacyInstallScriptURL,
		appHost,
		appPort,
		uiEnvPath,
	)
}

func (s *ModuleInstallService) loadEndpointList(ctx context.Context) ([]repository.EndpointKV, error) {
	if s == nil || s.endpointRepo == nil {
		return nil, fmt.Errorf("module install service is nil")
	}
	return s.endpointRepo.List(ctx)
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

	logInstall(logFn, "migration", "migrations applied source=%s count=%d", migrationSource, len(migrationFiles))
	return nil
}
