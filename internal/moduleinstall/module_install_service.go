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
	ModuleInstallScopeRemote  = "remote"
	ModuleInstallRuntimeLinux = "linux"
	ModuleInstallRuntimeK8s   = "k8s"

	schemaDigitsCount = 10
)

type ModuleInstallRequest struct {
	ModuleName        string
	Scope             string
	InstallRuntime    string
	AgentID           string
	AgentGRPCEndpoint string
	AppHost           string
	AppPort           int32
	Endpoint          string
	InstallCommand    string
	Kubeconfig        string
	KubeconfigPath    string
	TargetHost        string
	SudoPassword      *string
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
}

type moduleInstallTarget struct {
	Scope             string
	InstallRuntime    string
	AgentID           string
	AgentGRPCEndpoint string
	Kubeconfig        string
	KubeconfigPath    string
	Host              string
	SudoPassword      *string
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
	endpointRepo    repository.EndpointRepository
	runtimeRepo     repository.RuntimeConfigRepository
	certStoreRepo   repository.CertStoreRepository
	certStorePrefix string
	databaseURL     string

	agentRPCCAPath         string
	agentRPCClientCertPath string
	agentRPCClientKeyPath  string

	installScriptURLByModule map[string]string
}

type InstallLogFn func(stage, message string)

func NewModuleInstallService(
	endpointRepo repository.EndpointRepository,
	runtimeRepo repository.RuntimeConfigRepository,
	certStoreRepo repository.CertStoreRepository,
	certStorePrefix string,
	databaseURL string,
	agentRPCCAPath string,
	agentRPCClientCertPath string,
	agentRPCClientKeyPath string,
	installScriptURLByModule map[string]string,
) *ModuleInstallService {
	normalizedScriptURLs := make(map[string]string, len(installScriptURLByModule))
	for moduleName, scriptURL := range installScriptURLByModule {
		canonicalName := canonicalModuleName(moduleName)
		if canonicalName == "" {
			continue
		}
		normalizedScriptURLs[canonicalName] = strings.TrimSpace(scriptURL)
	}

	svc := &ModuleInstallService{
		endpointRepo:             endpointRepo,
		runtimeRepo:              runtimeRepo,
		certStoreRepo:            certStoreRepo,
		certStorePrefix:          strings.TrimSpace(certStorePrefix),
		databaseURL:              strings.TrimSpace(databaseURL),
		agentRPCCAPath:           strings.TrimSpace(agentRPCCAPath),
		agentRPCClientCertPath:   strings.TrimSpace(agentRPCClientCertPath),
		agentRPCClientKeyPath:    strings.TrimSpace(agentRPCClientKeyPath),
		installScriptURLByModule: normalizedScriptURLs,
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

	scope := normalizeScope(req.Scope)
	if scope != ModuleInstallScopeRemote {
		return nil, errorvar.ErrModuleInstallScope
	}
	installRuntime := normalizeInstallRuntime(req.InstallRuntime)
	isK8sRuntime := installRuntime == ModuleInstallRuntimeK8s

	appHost := strings.TrimSpace(req.AppHost)
	if appHost == "" {
		return nil, fmt.Errorf("app_host is required")
	}
	if hydrateErr := s.hydrateInstallTargetFromAgent(ctx, &req, logFn); hydrateErr != nil {
		return nil, hydrateErr
	}
	endpoint, endpointPort, err := resolveInstallEndpoint(scope, appHost, req.AppPort, req.Endpoint)
	if err != nil {
		return nil, err
	}

	target, err := buildInstallTarget(scope, req)
	if err != nil {
		return nil, err
	}
	logInstall(logFn, "install", "start module=%s scope=%s runtime=%s app_host=%s app_port=%d endpoint=%s", moduleName, scope, installRuntime, appHost, endpointPort, endpoint)
	logInstall(logFn, "target", "target host=%s", target.Host)
	if !isK8sRuntime {
		resolvedPort, portErr := resolveInstallPortForTarget(ctx, target, endpointPort, req.AppPort > 0)
		if portErr != nil {
			logInstall(logFn, "install", "[error] %v", portErr)
			return nil, portErr
		}
		if resolvedPort != endpointPort {
			logInstall(logFn, "install", "auto-selected available app_port=%d (previous=%d busy)", resolvedPort, endpointPort)
			endpointPort = resolvedPort
		} else {
			logInstall(logFn, "install", "confirmed app_port=%d is available on target", endpointPort)
		}
	} else {
		logInstall(logFn, "install", "k8s runtime detected: skip host port preflight")
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
	if moduleName == "ums" || moduleRequiresAdminRPC(moduleName) {
		adminRPCEndpoint, err = s.resolveAdminBootstrapEndpoint(ctx, endpoints.items, endpoints.err)
		if err != nil {
			return nil, err
		}
	}

	command := ""
	if scope == ModuleInstallScopeRemote && strings.TrimSpace(req.InstallCommand) != "" {
		command = strings.TrimSpace(req.InstallCommand)
		logInstall(logFn, "install", "using custom install command for remote target")
	} else {
		if isK8sRuntime {
			return nil, fmt.Errorf("install_command is required for k8s runtime")
		}
		uiEnvPath := ""
		if moduleName == "ui" {
			envRemotePath, envErr := s.generateAndPushUIEnv(ctx, target, endpoints.items, endpoints.err, logFn)
			if envErr != nil {
				return nil, envErr
			}
			uiEnvPath = envRemotePath
		}
		command = s.buildDefaultInstallCommand(moduleName, appHost, endpointPort, adminRPCEndpoint, uiEnvPath, target.SudoPassword)
		if command != "" {
			logInstall(logFn, "install", "resolved default install command for module=%s", moduleName)
		}
	}
	if command == "" {
		return nil, errorvar.ErrModuleInstallerMissing
	}

	if preseedErr := s.preseedInstallRouting(ctx, moduleName, target, endpoint, endpointPort, result, rollbacks, logFn); preseedErr != nil {
		return nil, preseedErr
	}

	var tlsBundle *moduleTLSBundle
	if !isK8sRuntime {
		tlsLocalBundle, tlsErr := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, logFn)
		if tlsErr != nil {
			logInstall(logFn, "tls", "[error] %v", tlsErr)
			return nil, fmt.Errorf("install tls materials failed: %w", tlsErr)
		}
		tlsBundle = tlsLocalBundle
	} else {
		logInstall(logFn, "tls", "k8s runtime detected: skip target tls material install")
	}

	logInstall(logFn, "install", "running install command")
	output, exitCode, installErr := runInstallCommand(
		ctx,
		command,
		target,
		func(line string) {
			logInstall(logFn, "agent", "%s", line)
		},
		func(line string) {
			logInstall(logFn, "agent", "%s", line)
		},
	)
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

	if !isK8sRuntime {
		tlsPresent, tlsCheckOutput, tlsCheckErr := moduleTLSExistsOnTarget(ctx, target, moduleName)
		if tlsCheckErr != nil {
			logInstall(logFn, "tls", "[warn] cannot verify tls materials on target: %v", tlsCheckErr)
		} else if !tlsPresent {
			logInstall(logFn, "tls", "[warn] tls materials missing after install, reinstalling: %s", strings.TrimSpace(tlsCheckOutput))
			repairedBundle, repairErr := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, logFn)
			if repairErr != nil {
				logInstall(logFn, "tls", "[error] tls self-heal failed: %v", repairErr)
				return nil, fmt.Errorf("tls materials missing after install: %w", repairErr)
			}
			tlsBundle = repairedBundle
			logInstall(logFn, "tls", "tls self-heal completed")
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
		logInstall(logFn, "hosts", "sync app host /etc/hosts on target (required) host=%s address=%s target=%s", hostEntryHost, targetAddr, target.Host)

		hostsUpdated, hostWarnings := syncHostsForTargets(ctx, hostEntries, []moduleInstallTarget{target})
		result.HostsUpdated = hostsUpdated
		result.Warnings = append(result.Warnings, hostWarnings...)
		if len(hostsUpdated) == 0 {
			for _, warning := range hostWarnings {
				logInstall(logFn, "hosts", "[error] %s", warning)
			}
			if len(hostWarnings) == 0 {
				logInstall(logFn, "hosts", "[error] no host entries were updated on target")
			}
			return nil, fmt.Errorf("sync app host to /etc/hosts failed")
		}
		logInstall(logFn, "hosts", "app host synced targets=%s", strings.Join(hostsUpdated, ","))
		for _, warning := range hostWarnings {
			logInstall(logFn, "hosts", "[warn] %s", warning)
		}

		if err := s.seedHostRoutingEntry(ctx, hostEntryHost, targetAddr); err != nil {
			logInstall(logFn, "hosts", "[error] %v", err)
			return nil, fmt.Errorf("seed host routing failed: %w", err)
		}
		hostRoutingKey := keycfg.RuntimeHostEntryKey(hostEntryHost)
		logInstall(logFn, "hosts", "seeded host routing key=%s", hostRoutingKey)
		rollbacks.Add("hosts", func(rollbackCtx context.Context) error {
			deleteScript := buildHostsDeleteCommand(hostEntryHost, target.SudoPassword)
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
			logInstall(logFn, "hosts", "[warn] %s", warning)
		}

		if nginxErr := ensureModuleNginxProxyOnTarget(ctx, target, moduleName, appHost, endpointPort, logFn); nginxErr != nil {
			logInstall(logFn, "nginx", "[error] %v", nginxErr)
			return nil, fmt.Errorf("configure nginx proxy failed: %w", nginxErr)
		}

		if seedErr := s.seedModuleTLSBundle(ctx, moduleName, tlsBundle); seedErr != nil {
			logInstall(logFn, "tls", "[error] %v", seedErr)
			return nil, fmt.Errorf("seed module tls bundle failed: %w", seedErr)
		}
		s.addModuleTLSRollbackStep(rollbacks, moduleName)
		logInstall(logFn, "tls", "seeded module tls bundle into cert store")
	} else {
		result.Warnings = append(result.Warnings, "k8s runtime mode: skipped target tls/hosts/nginx/cert-store steps")
		logInstall(logFn, "install", "k8s runtime mode: skipped target tls/hosts/nginx/cert-store steps")
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
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.installScriptURLByModule[canonicalModuleName(moduleName)])
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
