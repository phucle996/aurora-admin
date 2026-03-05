package service

import (
	"admin/internal/repository"
	"admin/pkg/errorvar"
	sshpkg "admin/pkg/ssh"
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ModuleInstallScopeLocal  = "local"
	ModuleInstallScopeRemote = "remote"

	schemaDigitsCount = 10
	schemaKeyPrefix   = "postgresql/schema"
)

type ModuleInstallRequest struct {
	ModuleName     string
	Scope          string
	AppHost        string
	Endpoint       string
	InstallCommand string

	SSHHost       string
	SSHPort       int32
	SSHUsername   string
	SSHPassword   *string
	SSHPrivateKey *string
}

type ModuleInstallResult struct {
	ModuleName      string   `json:"module_name"`
	Scope           string   `json:"scope"`
	Endpoint        string   `json:"endpoint"`
	EndpointValue   string   `json:"endpoint_value"`
	InstallExecuted bool     `json:"install_executed"`
	InstallOutput   string   `json:"install_output"`
	InstallExitCode int      `json:"install_exit_code"`
	HostsUpdated    []string `json:"hosts_updated"`
	Warnings        []string `json:"warnings"`

	SchemaKey       string   `json:"schema_key"`
	SchemaName      string   `json:"schema_name"`
	MigrationFiles  []string `json:"migration_files"`
	MigrationSource string   `json:"migration_source"`

	HealthcheckPassed bool   `json:"healthcheck_passed"`
	HealthcheckOutput string `json:"healthcheck_output"`
}

type moduleInstallTarget struct {
	Scope      string
	Username   string
	Host       string
	Port       int32
	Password   *string
	PrivateKey *string
}

type hostsEntry struct {
	Address string
	Host    string
}

type moduleMigrationSource struct {
	DownloadURLs []string
	LocalDirs    []string
}

var moduleMigrationSources = map[string]moduleMigrationSource{
	"vm": {
		DownloadURLs: []string{
			"https://codeload.github.com/phucle996/aurora-vm-service/zip/refs/heads/main",
		},
		LocalDirs: []string{
			"../vm-service/migrations",
			"vm-service/migrations",
		},
	},
	"ums": {
		DownloadURLs: []string{
			"https://codeload.github.com/phucle996/aurora-user-management-system/zip/refs/heads/main",
			"https://codeload.github.com/phucle996/UserManagmentSystem/zip/refs/heads/main",
		},
		LocalDirs: []string{
			"../UserManagmentSystem/migrations",
			"UserManagmentSystem/migrations",
		},
	},
	"mail": {
		DownloadURLs: []string{
			"https://codeload.github.com/phucle996/aurora-mail-service/zip/refs/heads/main",
		},
		LocalDirs: []string{
			"../mail-service/migrations",
			"mail-service/migrations",
		},
	},
}

var moduleAliasToCanonical = map[string]string{
	"vm":              "vm",
	"vm-service":      "vm",
	"kvm":             "vm",
	"hypervisor":      "vm",
	"libvirt":         "vm",
	"ums":             "ums",
	"user":            "ums",
	"user-management": "ums",
	"usermanagment":   "ums",
	"mail":            "mail",
	"mail-service":    "mail",
}

type ModuleInstallService struct {
	endpointRepo repository.EndpointRepository
	runtimeRepo  repository.RuntimeConfigRepository
	databaseURL  string
}

func NewModuleInstallService(
	endpointRepo repository.EndpointRepository,
	runtimeRepo repository.RuntimeConfigRepository,
	databaseURL string,
) *ModuleInstallService {
	return &ModuleInstallService{
		endpointRepo: endpointRepo,
		runtimeRepo:  runtimeRepo,
		databaseURL:  strings.TrimSpace(databaseURL),
	}
}

func (s *ModuleInstallService) Install(ctx context.Context, req ModuleInstallRequest) (*ModuleInstallResult, error) {
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

	appHost := strings.TrimSpace(req.AppHost)
	if appHost == "" {
		return nil, fmt.Errorf("app_host is required")
	}

	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}

	target, err := buildInstallTarget(scope, req)
	if err != nil {
		return nil, err
	}

	result := &ModuleInstallResult{
		ModuleName: moduleName,
		Scope:      scope,
		Endpoint:   endpoint,
	}

	if err := s.prepareSchemaAndMigrate(ctx, moduleName, result); err != nil {
		return nil, err
	}
	rollbackSchemaName := strings.TrimSpace(result.SchemaName)

	command := strings.TrimSpace(req.InstallCommand)
	if command != "" {
		output, exitCode, installErr := runInstallCommand(ctx, command, target)
		result.InstallExecuted = true
		result.InstallOutput = strings.TrimSpace(output)
		result.InstallExitCode = exitCode
		if installErr != nil {
			return nil, withSchemaRollback(ctx, s.databaseURL, rollbackSchemaName, fmt.Errorf("install command failed: %w", installErr))
		}
	}

	healthcheckOutput, healthErr := ensureCurlAndCheckEndpoint(ctx, target, endpoint)
	result.HealthcheckOutput = strings.TrimSpace(healthcheckOutput)
	if healthErr != nil {
		return nil, withSchemaRollback(ctx, s.databaseURL, rollbackSchemaName, fmt.Errorf("service healthcheck failed: %w", healthErr))
	}
	result.HealthcheckPassed = true

	endpointValue := encodeEndpointValue(target, endpoint)
	result.EndpointValue = endpointValue
	if err := s.endpointRepo.Upsert(ctx, moduleName, endpointValue); err != nil {
		return nil, withSchemaRollback(ctx, s.databaseURL, rollbackSchemaName, err)
	}

	targets, err := s.resolveHostSyncTargets(ctx, target)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("cannot load existing endpoint targets: %v", err))
		targets = []moduleInstallTarget{target}
	}

	hostEntries, entryErr := s.buildHostsEntries(ctx, appHost, target.Host)
	if entryErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("cannot build hosts entries: %v", entryErr))
		hostEntries = []hostsEntry{{Address: normalizeAddress(target.Host), Host: strings.TrimSpace(appHost)}}
	}

	hostsUpdated, warnings := syncHostsForTargets(ctx, hostEntries, targets)
	result.HostsUpdated = hostsUpdated
	result.Warnings = append(result.Warnings, warnings...)
	return result, nil
}

func (s *ModuleInstallService) prepareSchemaAndMigrate(
	ctx context.Context,
	moduleName string,
	result *ModuleInstallResult,
) (err error) {
	source, ok := moduleMigrationSources[moduleName]
	if !ok {
		return nil
	}
	if strings.TrimSpace(s.databaseURL) == "" {
		return fmt.Errorf("database_url is empty for module %s", moduleName)
	}
	if _, err := exec.LookPath("psql"); err != nil {
		return fmt.Errorf("psql client is required: %w", err)
	}

	migrationsDir, migrationSource, cleanup, err := materializeMigrations(ctx, moduleName, source)
	if err != nil {
		return fmt.Errorf("prepare migrations for %s failed: %w", moduleName, err)
	}
	defer cleanup()

	schemaName, err := generateSchemaName(moduleName)
	if err != nil {
		return fmt.Errorf("generate schema for %s failed: %w", moduleName, err)
	}

	schemaCreated := false
	defer func() {
		if err == nil || !schemaCreated {
			return
		}
		err = withSchemaRollback(ctx, s.databaseURL, schemaName, err)
	}()

	if err := createSchemaWithPSQL(ctx, s.databaseURL, schemaName); err != nil {
		return fmt.Errorf("create schema failed: %w", err)
	}
	schemaCreated = true

	schemaKey := schemaKeyPrefix + "/" + moduleName
	result.SchemaKey = schemaKey
	result.SchemaName = schemaName

	if err := s.runtimeRepo.Upsert(ctx, schemaKey, schemaName); err != nil {
		return fmt.Errorf("seed schema key %s failed: %w", schemaKey, err)
	}

	migrationFiles, err := listMigrationUpFiles(migrationsDir)
	if err != nil {
		return err
	}
	if err := runMigrationFilesWithPSQL(ctx, s.databaseURL, schemaName, migrationFiles); err != nil {
		return err
	}

	result.MigrationFiles = make([]string, 0, len(migrationFiles))
	for _, file := range migrationFiles {
		result.MigrationFiles = append(result.MigrationFiles, filepath.Base(file))
	}
	result.MigrationSource = migrationSource
	return nil
}

func materializeMigrations(
	ctx context.Context,
	moduleName string,
	source moduleMigrationSource,
) (string, string, func(), error) {
	rootDir, err := os.MkdirTemp("", "aurora-module-migrations-*")
	if err != nil {
		return "", "", func() {}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(rootDir)
	}

	destDir := filepath.Join(rootDir, moduleName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		cleanup()
		return "", "", func() {}, err
	}

	for _, url := range source.DownloadURLs {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		if err := downloadMigrationZip(ctx, url, destDir); err == nil {
			files, listErr := listMigrationUpFiles(destDir)
			if listErr == nil && len(files) > 0 {
				return destDir, "download:" + url, cleanup, nil
			}
		}
	}

	for _, localDir := range source.LocalDirs {
		localDir = strings.TrimSpace(localDir)
		if localDir == "" {
			continue
		}
		if err := copyMigrationFilesFromLocal(localDir, destDir); err == nil {
			files, listErr := listMigrationUpFiles(destDir)
			if listErr == nil && len(files) > 0 {
				return destDir, "local:" + localDir, cleanup, nil
			}
		}
	}

	cleanup()
	return "", "", func() {}, fmt.Errorf("no migration source available for module %s", moduleName)
}

func downloadMigrationZip(ctx context.Context, url string, destDir string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("download failed with status %d", response.StatusCode)
	}

	payload, err := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return err
	}

	count := 0
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(strings.TrimSpace(file.Name))
		if !strings.Contains(name, "/migrations/") || !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		targetPath := filepath.Join(destDir, filepath.Base(name))
		if err := extractZipFile(file, targetPath); err != nil {
			return err
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("no *.up.sql found in archive")
	}
	return nil
}

func extractZipFile(file *zip.File, destination string) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	out, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, reader); err != nil {
		return err
	}
	return nil
}

func copyMigrationFilesFromLocal(localDir string, destDir string) error {
	candidates := resolveLocalDirCandidates(localDir)
	for _, candidate := range candidates {
		pattern := filepath.Join(candidate, "*.up.sql")
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(files) == 0 {
			continue
		}
		sort.Strings(files)
		for _, sourcePath := range files {
			targetPath := filepath.Join(destDir, filepath.Base(sourcePath))
			if err := copyFile(sourcePath, targetPath); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("no migrations in %s", localDir)
}

func copyFile(sourcePath string, targetPath string) error {
	in, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func listMigrationUpFiles(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no migration file (*.up.sql) found")
	}
	sort.Strings(files)
	return files, nil
}

func generateSchemaName(moduleName string) (string, error) {
	digits, err := randomDigits(schemaDigitsCount)
	if err != nil {
		return "", err
	}
	return normalizeSchemaPrefix(moduleName) + "_" + digits, nil
}

func randomDigits(length int) (string, error) {
	if length <= 0 {
		return "", nil
	}
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	out := strings.Builder{}
	out.Grow(length)
	for _, b := range buffer {
		out.WriteByte('0' + (b % 10))
	}
	return out.String(), nil
}

func normalizeSchemaPrefix(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "/", "_")
	if value == "" {
		return "svc"
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' {
			continue
		}
		value = strings.ReplaceAll(value, string(ch), "_")
	}
	if value[0] < 'a' || value[0] > 'z' {
		value = "svc_" + value
	}
	return value
}

func createSchemaWithPSQL(ctx context.Context, databaseURL string, schema string) error {
	sql := "CREATE SCHEMA IF NOT EXISTS " + quoteSQLIdentifier(schema) + ";"
	command := exec.CommandContext(
		ctx,
		"psql",
		databaseURL,
		"-v", "ON_ERROR_STOP=1",
		"-c", sql,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql create schema failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func dropSchemaWithPSQL(ctx context.Context, databaseURL string, schema string) error {
	cleanSchema := strings.TrimSpace(schema)
	if cleanSchema == "" {
		return nil
	}

	sql := "DROP SCHEMA IF EXISTS " + quoteSQLIdentifier(cleanSchema) + " CASCADE;"
	command := exec.CommandContext(
		ctx,
		"psql",
		databaseURL,
		"-v", "ON_ERROR_STOP=1",
		"-c", sql,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql drop schema failed: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func withSchemaRollback(ctx context.Context, databaseURL string, schema string, baseErr error) error {
	if baseErr == nil {
		return nil
	}
	cleanSchema := strings.TrimSpace(schema)
	if cleanSchema == "" || strings.TrimSpace(databaseURL) == "" {
		return baseErr
	}

	_ = ctx
	rollbackCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if rollbackErr := dropSchemaWithPSQL(rollbackCtx, databaseURL, cleanSchema); rollbackErr != nil {
		return fmt.Errorf("%w; rollback schema %s failed: %v", baseErr, cleanSchema, rollbackErr)
	}
	return fmt.Errorf("%w; schema %s rolled back", baseErr, cleanSchema)
}

func runMigrationFilesWithPSQL(
	ctx context.Context,
	databaseURL string,
	schema string,
	migrationFiles []string,
) error {
	searchPathSQL := "SET search_path TO " + quoteSQLIdentifier(schema) + ", public;"

	for _, file := range migrationFiles {
		command := exec.CommandContext(
			ctx,
			"psql",
			databaseURL,
			"-v", "ON_ERROR_STOP=1",
			"-c", searchPathSQL,
			"-f", file,
		)
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("apply migration %s failed: %s", filepath.Base(file), strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func quoteSQLIdentifier(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.ReplaceAll(value, `"`, `""`)
	return `"` + value + `"`
}

func (s *ModuleInstallService) resolveHostSyncTargets(ctx context.Context, current moduleInstallTarget) ([]moduleInstallTarget, error) {
	items, err := s.endpointRepo.List(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]moduleInstallTarget, 0, len(items)+2)
	out = append(out, current)
	out = append(out, moduleInstallTarget{
		Scope:    ModuleInstallScopeLocal,
		Username: "aurora",
		Host:     "127.0.0.1",
		Port:     22,
	})

	for _, item := range items {
		target, ok := parseEndpointTarget(item.Value)
		if !ok {
			continue
		}
		out = append(out, target)
	}
	return dedupeTargets(out), nil
}

func (s *ModuleInstallService) buildHostsEntries(
	ctx context.Context,
	currentHost string,
	currentAddress string,
) ([]hostsEntry, error) {
	entries := map[string]string{}
	normalizedCurrentHost := strings.TrimSpace(currentHost)
	normalizedCurrentAddress := normalizeAddress(currentAddress)
	if normalizedCurrentHost != "" && normalizedCurrentAddress != "" {
		entries[normalizedCurrentHost] = normalizedCurrentAddress
	}

	items, err := s.endpointRepo.List(ctx)
	if err != nil {
		return mapToHostsEntries(entries), err
	}

	for _, item := range items {
		target, endpoint, ok := parseEndpointTargetAndEndpoint(item.Value)
		if !ok {
			continue
		}
		host := endpointHost(endpoint)
		address := normalizeAddress(target.Host)
		if host == "" || address == "" {
			continue
		}
		old, exists := entries[host]
		if exists && isLoopbackAddress(old) && !isLoopbackAddress(address) {
			entries[host] = address
			continue
		}
		if !exists {
			entries[host] = address
		}
	}

	return mapToHostsEntries(entries), nil
}

func dedupeTargets(items []moduleInstallTarget) []moduleInstallTarget {
	out := make([]moduleInstallTarget, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := strings.Join([]string{
			item.Scope,
			item.Username,
			item.Host,
			strconv.Itoa(int(item.Port)),
			deref(item.Password),
			deref(item.PrivateKey),
		}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Host < out[j].Host
	})
	return out
}

func syncHostsForTargets(
	ctx context.Context,
	entries []hostsEntry,
	targets []moduleInstallTarget,
) ([]string, []string) {
	hostsUpdated := make([]string, 0, len(targets))
	warnings := make([]string, 0)
	if len(entries) == 0 {
		return hostsUpdated, append(warnings, "skip /etc/hosts sync: empty hosts entries")
	}

	for _, target := range targets {
		switch target.Scope {
		case ModuleInstallScopeLocal:
			hasErr := false
			for _, entry := range entries {
				if err := upsertLocalHosts(entry.Address, entry.Host); err != nil {
					hasErr = true
					warnings = append(warnings, fmt.Sprintf("local hosts update failed (%s): %v", entry.Host, err))
					continue
				}
			}
			if hasErr {
				continue
			}
			hostsUpdated = append(hostsUpdated, "local")
		case ModuleInstallScopeRemote:
			hasErr := false
			for _, entry := range entries {
				cmd := buildHostsUpdateCommand(entry.Address, entry.Host)
				runCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
				_, err := sshpkg.Run(runCtx, sshpkg.RunInput{
					Host:       target.Host,
					Port:       target.Port,
					Username:   target.Username,
					Password:   target.Password,
					PrivateKey: target.PrivateKey,
					Timeout:    15 * time.Second,
					Command:    cmd,
				})
				cancel()
				if err != nil {
					hasErr = true
					warnings = append(warnings, fmt.Sprintf("remote hosts update failed (%s/%s): %v", target.Host, entry.Host, err))
				}
			}
			if hasErr {
				continue
			}
			hostsUpdated = append(hostsUpdated, target.Host)
		}
	}

	return hostsUpdated, warnings
}

func upsertLocalHosts(address, host string) error {
	path := "/etc/hosts"
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	updated := rewriteHostsContent(string(data), address, host)
	if updated == string(data) {
		return nil
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return err
	}
	return nil
}

func rewriteHostsContent(raw, address, host string) string {
	lines := strings.Split(raw, "\n")
	filtered := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		if containsHostToken(line, host) {
			continue
		}
		filtered = append(filtered, line)
	}
	filtered = append(filtered, fmt.Sprintf("%s %s", address, host))
	return strings.Join(filtered, "\n")
}

func containsHostToken(line string, host string) bool {
	clean := strings.TrimSpace(line)
	if clean == "" || strings.HasPrefix(clean, "#") {
		return false
	}
	fields := strings.Fields(clean)
	if len(fields) < 2 {
		return false
	}
	for _, token := range fields[1:] {
		if token == host {
			return true
		}
	}
	return false
}

func buildHostsUpdateCommand(address, host string) string {
	script := fmt.Sprintf(
		`tmp="$(mktemp)"; grep -v -E "(^|[[:space:]])%s([[:space:]]|$)" /etc/hosts > "$tmp" || true; printf "%%s %%s\n" %s %s >> "$tmp"; if [ "$(id -u)" -eq 0 ]; then cat "$tmp" > /etc/hosts; else sudo sh -c "cat \"$tmp\" > /etc/hosts"; fi; rm -f "$tmp"`,
		regexpEscape(host),
		shellEscape(address),
		shellEscape(host),
	)
	return "bash -lc " + strconv.Quote(script)
}

func runInstallCommand(ctx context.Context, command string, target moduleInstallTarget) (string, int, error) {
	return runCommandOnTarget(ctx, target, command, 40*time.Second)
}

func runCommandOnTarget(
	ctx context.Context,
	target moduleInstallTarget,
	command string,
	timeout time.Duration,
) (string, int, error) {
	if target.Scope == ModuleInstallScopeLocal {
		cmd := exec.CommandContext(ctx, "bash", "-lc", command)
		out, err := cmd.CombinedOutput()
		exitCode := 0
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return string(out), exitCode, err
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := sshpkg.Run(runCtx, sshpkg.RunInput{
		Host:       target.Host,
		Port:       target.Port,
		Username:   target.Username,
		Password:   target.Password,
		PrivateKey: target.PrivateKey,
		Timeout:    timeout,
		Command:    command,
	})
	if result == nil {
		return "", -1, err
	}
	return result.Output, result.ExitCode, err
}

func buildInstallTarget(scope string, req ModuleInstallRequest) (moduleInstallTarget, error) {
	target := moduleInstallTarget{
		Scope: scope,
		Port:  22,
	}

	switch scope {
	case ModuleInstallScopeLocal:
		target.Username = strings.TrimSpace(req.SSHUsername)
		if target.Username == "" {
			target.Username = "aurora"
		}
		target.Host = normalizeAddress(req.SSHHost)
		if target.Host == "" {
			target.Host = detectLocalIPv4()
		}
		target.Port = normalizePort(req.SSHPort)
		target.Password = normalizeOptionalSecret(req.SSHPassword)
		target.PrivateKey = normalizeOptionalSecret(req.SSHPrivateKey)
		return target, nil
	case ModuleInstallScopeRemote:
		target.Username = strings.TrimSpace(req.SSHUsername)
		target.Host = normalizeAddress(req.SSHHost)
		target.Port = normalizePort(req.SSHPort)
		target.Password = normalizeOptionalSecret(req.SSHPassword)
		target.PrivateKey = normalizeOptionalSecret(req.SSHPrivateKey)
		if target.Username == "" || target.Host == "" {
			return target, fmt.Errorf("ssh_username and ssh_host are required for remote install")
		}
		if target.Password == nil && target.PrivateKey == nil {
			return target, fmt.Errorf("ssh_password or ssh_private_key is required for remote install")
		}
		return target, nil
	default:
		return target, errorvar.ErrModuleInstallScope
	}
}

func encodeEndpointValue(target moduleInstallTarget, endpoint string) string {
	secret := ""
	if target.PrivateKey != nil {
		secret = "key:" + base64.RawStdEncoding.EncodeToString([]byte(*target.PrivateKey))
	} else if target.Password != nil {
		secret = *target.Password
	}

	return fmt.Sprintf(
		"%s(%s|%s|%s|%d):%s",
		target.Scope,
		target.Username,
		target.Host,
		secret,
		target.Port,
		strings.TrimSpace(endpoint),
	)
}

func parseEndpointTarget(raw string) (moduleInstallTarget, bool) {
	target, _, ok := parseEndpointTargetAndEndpoint(raw)
	return target, ok
}

func parseEndpointTargetAndEndpoint(raw string) (moduleInstallTarget, string, bool) {
	value := strings.TrimSpace(raw)
	var out moduleInstallTarget
	if value == "" {
		return out, "", false
	}

	scope, remainder, ok := strings.Cut(value, "(")
	if !ok {
		return out, "", false
	}
	scope = normalizeScope(scope)
	if scope != ModuleInstallScopeLocal && scope != ModuleInstallScopeRemote {
		return out, "", false
	}

	meta, endpoint, ok := strings.Cut(remainder, "):")
	if !ok {
		return out, "", false
	}
	parts := strings.Split(meta, "|")
	if len(parts) < 3 {
		return out, "", false
	}

	out.Scope = scope
	out.Username = strings.TrimSpace(parts[0])
	out.Host = normalizeAddress(parts[1])
	secret := strings.TrimSpace(parts[2])
	out.Port = 22
	if len(parts) >= 4 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(parts[3])); err == nil && parsed > 0 && parsed <= 65535 {
			out.Port = int32(parsed)
		}
	}

	if strings.HasPrefix(secret, "key:") {
		encoded := strings.TrimPrefix(secret, "key:")
		if decoded, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
			val := string(decoded)
			out.PrivateKey = &val
		}
	} else if secret != "" {
		val := secret
		out.Password = &val
	}

	if out.Host == "" {
		return moduleInstallTarget{}, "", false
	}
	return out, strings.TrimSpace(endpoint), true
}

func normalizeOptionalSecret(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizePort(port int32) int32 {
	if port <= 0 || port > 65535 {
		return 22
	}
	return port
}

func normalizeAddress(raw string) string {
	host := strings.TrimSpace(raw)
	if host == "" {
		return ""
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return strings.TrimSpace(parsedHost)
	}
	if strings.Contains(host, "://") {
		host = strings.SplitN(host, "://", 2)[1]
	}
	if cut, _, found := strings.Cut(host, "/"); found {
		host = cut
	}
	if h, _, found := strings.Cut(host, ":"); found {
		host = h
	}
	return strings.Trim(host, "[]")
}

func normalizeScope(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func normalizeModuleName(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func canonicalModuleName(raw string) string {
	normalized := normalizeModuleName(raw)
	if normalized == "" {
		return ""
	}
	if canonical, ok := moduleAliasToCanonical[normalized]; ok {
		return canonical
	}
	return normalized
}

func detectLocalIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if udpAddress, ok := conn.LocalAddr().(*net.UDPAddr); ok && udpAddress.IP != nil {
			if ipv4 := udpAddress.IP.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return "127.0.0.1"
}

func mapToHostsEntries(raw map[string]string) []hostsEntry {
	out := make([]hostsEntry, 0, len(raw))
	for host, address := range raw {
		cleanHost := strings.TrimSpace(host)
		cleanAddress := normalizeAddress(address)
		if cleanHost == "" || cleanAddress == "" {
			continue
		}
		out = append(out, hostsEntry{
			Address: cleanAddress,
			Host:    cleanHost,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Host < out[j].Host
	})
	return out
}

func isLoopbackAddress(raw string) bool {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return false
	}
	if clean == "localhost" {
		return true
	}
	ip := net.ParseIP(clean)
	return ip != nil && ip.IsLoopback()
}

func endpointHost(endpoint string) string {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return ""
	}

	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			return strings.TrimSpace(parsed.Hostname())
		}
	}

	working := raw
	if cut, _, found := strings.Cut(working, "/"); found {
		working = cut
	}

	if strings.Count(working, ":") == 1 {
		if host, _, err := net.SplitHostPort(working); err == nil {
			return normalizeAddress(host)
		}
	}

	return normalizeAddress(working)
}

func endpointPort(endpoint string) string {
	raw := strings.TrimSpace(endpoint)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			return parsed.Port()
		}
	}
	working := raw
	if cut, _, found := strings.Cut(working, "/"); found {
		working = cut
	}
	if host, port, err := net.SplitHostPort(working); err == nil && normalizeAddress(host) != "" {
		return strings.TrimSpace(port)
	}
	return ""
}

func buildHealthcheckCandidates(endpoint string) []string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil
	}

	seen := map[string]struct{}{}
	add := func(urlValue string, out *[]string) {
		cleaned := strings.TrimSpace(urlValue)
		if cleaned == "" {
			return
		}
		if _, ok := seen[cleaned]; ok {
			return
		}
		seen[cleaned] = struct{}{}
		*out = append(*out, cleaned)
	}

	candidates := make([]string, 0, 12)
	if strings.Contains(endpoint, "://") {
		add(endpoint, &candidates)
		add(strings.TrimRight(endpoint, "/")+"/health/readiness", &candidates)
		add(strings.TrimRight(endpoint, "/")+"/health", &candidates)
		return candidates
	}

	httpBase := "http://" + endpoint
	httpsBase := "https://" + endpoint
	add(httpBase+"/health/readiness", &candidates)
	add(httpBase+"/health", &candidates)
	add(httpBase, &candidates)
	add(httpsBase+"/health/readiness", &candidates)
	add(httpsBase+"/health", &candidates)
	add(httpsBase, &candidates)

	port := endpointPort(endpoint)
	if port != "" {
		localHTTP := "http://127.0.0.1:" + port
		add(localHTTP+"/health/readiness", &candidates)
		add(localHTTP+"/health", &candidates)
		add(localHTTP, &candidates)
	}

	return candidates
}

func ensureCurlAndCheckEndpoint(
	ctx context.Context,
	target moduleInstallTarget,
	endpoint string,
) (string, error) {
	candidates := buildHealthcheckCandidates(endpoint)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no healthcheck candidate for endpoint %s", endpoint)
	}

	urlList := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		urlList = append(urlList, shellEscape(candidate))
	}

	healthScript := strings.Join([]string{
		"set -e",
		"ensure_curl(){",
		"  if command -v curl >/dev/null 2>&1; then return 0; fi",
		"  install_cmd=''",
		"  if command -v apt-get >/dev/null 2>&1; then install_cmd='apt-get update -y && apt-get install -y curl'; fi",
		"  if [ -z \"$install_cmd\" ] && command -v dnf >/dev/null 2>&1; then install_cmd='dnf install -y curl'; fi",
		"  if [ -z \"$install_cmd\" ] && command -v yum >/dev/null 2>&1; then install_cmd='yum install -y curl'; fi",
		"  if [ -z \"$install_cmd\" ] && command -v apk >/dev/null 2>&1; then install_cmd='apk add --no-cache curl'; fi",
		"  if [ -z \"$install_cmd\" ] && command -v zypper >/dev/null 2>&1; then install_cmd='zypper --non-interactive install curl'; fi",
		"  if [ -z \"$install_cmd\" ]; then echo 'cannot install curl automatically'; return 1; fi",
		"  if [ \"$(id -u)\" -eq 0 ]; then sh -lc \"$install_cmd\"; return $?; fi",
		"  if command -v sudo >/dev/null 2>&1; then sudo sh -lc \"$install_cmd\"; return $?; fi",
		"  echo 'need root/sudo to install curl'; return 1",
		"}",
		"ensure_curl",
		"for url in " + strings.Join(urlList, " ") + "; do",
		"  if curl -k -f -sS --max-time 8 \"$url\" >/dev/null; then",
		"    echo \"healthcheck_ok:$url\"",
		"    exit 0",
		"  fi",
		"done",
		"echo 'healthcheck_failed'",
		"exit 1",
	}, "\n")

	output, exitCode, err := runCommandOnTarget(ctx, target, "bash -lc "+strconv.Quote(healthScript), 90*time.Second)
	if err != nil {
		return output, fmt.Errorf("exit_code=%d: %w", exitCode, err)
	}
	if exitCode != 0 {
		return output, fmt.Errorf("exit_code=%d", exitCode)
	}
	return output, nil
}

func resolveLocalDirCandidates(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	out := []string{trimmed}
	if filepath.IsAbs(trimmed) {
		return dedupeStringList(out)
	}

	if cwd, err := os.Getwd(); err == nil {
		out = append(out, filepath.Join(cwd, trimmed))
	}
	if executablePath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(executablePath)
		out = append(out, filepath.Join(execDir, trimmed))
		out = append(out, filepath.Join(execDir, "..", trimmed))
	}
	return dedupeStringList(out)
}

func dedupeStringList(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		cleaned := filepath.Clean(strings.TrimSpace(item))
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	return out
}

func shellEscape(raw string) string {
	if raw == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(raw, "'", `'"'"'`) + "'"
}

func regexpEscape(raw string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`.`, `\.`,
		`+`, `\+`,
		`*`, `\*`,
		`?`, `\?`,
		`^`, `\^`,
		`$`, `\$`,
		`(`, `\(`,
		`)`, `\)`,
		`[`, `\[`,
		`]`, `\]`,
		`{`, `\{`,
		`}`, `\}`,
		`|`, `\|`,
	)
	return replacer.Replace(raw)
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
