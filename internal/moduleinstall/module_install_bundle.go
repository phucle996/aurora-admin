package moduleinstall

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type moduleBundleRelease struct {
	Version          string
	ArtifactURL      string
	ArtifactChecksum string
}

type githubLatestReleaseResponse struct {
	TagName string `json:"tag_name"`
}

func (s *ModuleInstallService) installModuleViaAgentBundle(
	ctx context.Context,
	target moduleInstallTarget,
	moduleName string,
	release *moduleBundleRelease,
	appHost string,
	appPort int32,
	endpoint string,
	adminRPCEndpoint string,
	logFn InstallLogFn,
) (*agentInstallModuleResponse, *moduleTLSBundle, error) {
	if strings.TrimSpace(adminRPCEndpoint) == "" {
		return nil, nil, fmt.Errorf("admin rpc endpoint is required for module %s", canonicalModuleName(moduleName))
	}

	if release == nil {
		var err error
		release, err = s.resolveLatestModuleBundleRelease(ctx, moduleName, target.Architecture)
		if err != nil {
			return nil, nil, err
		}
	}
	logInstall(
		logFn,
		"install",
		"resolved bundle artifact module=%s version=%s arch=%s",
		canonicalModuleName(moduleName),
		release.Version,
		normalizeBundleArch(target.Architecture),
	)

	tlsResult, tlsErr := installModuleTLSOnTarget(ctx, target, moduleName, appHost, endpoint, logFn)
	if tlsErr != nil {
		return nil, nil, fmt.Errorf("install tls materials failed: %w", tlsErr)
	}
	tlsBundle := tlsResult

	callCtx, cancel := context.WithTimeout(ctx, installCommandTimeout+10*time.Second)
	defer cancel()
	env := map[string]string{
		"ADMIN_RPC_ENDPOINT": strings.TrimSpace(adminRPCEndpoint),
	}
	if moduleUsesClientCSRBootstrap(moduleName) {
		if s == nil || s.moduleBootstrapTokenIssuer == nil {
			return nil, nil, fmt.Errorf("module bootstrap token issuer is unavailable for module %s", canonicalModuleName(moduleName))
		}
		token, err := s.moduleBootstrapTokenIssuer.IssueModuleBootstrapToken(callCtx, moduleName)
		if err != nil {
			return nil, nil, fmt.Errorf("issue module bootstrap token failed: %w", err)
		}
		env["ADMIN_RPC_BOOTSTRAP_TOKEN"] = strings.TrimSpace(token)
	}
	agentReq := agentInstallModuleRequest{
		APIVersion:       installerRPCVersionV1,
		RequestID:        newAgentOperationRequestID("install", moduleName),
		Module:           canonicalModuleName(moduleName),
		Version:          release.Version,
		ArtifactURL:      release.ArtifactURL,
		ArtifactChecksum: release.ArtifactChecksum,
		AppHost:          strings.TrimSpace(appHost),
		AppPort:          appPort,
		Env:              env,
	}
	var (
		res *agentInstallModuleResponse
		err error
	)
	if logFn != nil {
		res, err = streamInstallModuleOnAgent(callCtx, target, agentReq, func(event agentInstallModuleStreamEvent) {
			stage := strings.TrimSpace(event.Stage)
			if stage == "" {
				stage = "install"
			}
			message := strings.TrimSpace(event.Message)
			if message == "" && strings.EqualFold(strings.TrimSpace(event.Type), "result") {
				message = "module install completed"
			}
			if message != "" {
				logInstall(logFn, "agent", "[%s] %s", stage, message)
			}
		})
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "unimplemented") {
			logInstall(logFn, "agent", "[warn] agent stream install rpc is unavailable, fallback to unary install rpc")
			res, err = installModuleOnAgent(callCtx, target, agentReq)
		}
	} else {
		res, err = installModuleOnAgent(callCtx, target, agentReq)
		if res != nil {
			for _, entry := range res.Logs {
				stage := strings.TrimSpace(entry.Stage)
				if stage == "" {
					stage = "install"
				}
				logInstall(logFn, "agent", "[%s] %s", stage, strings.TrimSpace(entry.Message))
			}
		}
	}
	if err != nil {
		return res, tlsBundle, err
	}
	return res, tlsBundle, nil
}

func shouldUseAgentBundleInstall(moduleName string, target moduleInstallTarget) bool {
	if !moduleSupportsAgentBundleInstall(moduleName) {
		return false
	}
	return strings.TrimSpace(target.AgentGRPCEndpoint) != ""
}

func moduleSupportsAgentBundleInstall(moduleName string) bool {
	_, ok := moduleArtifactSourceFor(moduleName)
	return ok
}

func (s *ModuleInstallService) resolveLatestModuleBundleRelease(
	ctx context.Context,
	moduleName string,
	targetArch string,
) (*moduleBundleRelease, error) {
	source, ok := moduleArtifactSourceFor(moduleName)
	if !ok {
		return nil, fmt.Errorf("bundle artifact is not configured for module %s", canonicalModuleName(moduleName))
	}

	tag, err := fetchLatestGitHubReleaseTag(ctx, source.RepoSlug)
	if err != nil {
		return nil, err
	}
	assetName, err := bundleAssetName(source, targetArch)
	if err != nil {
		return nil, err
	}
	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", strings.TrimSpace(source.RepoSlug), strings.TrimSpace(tag))
	checksum, err := fetchReleaseChecksum(ctx, baseURL+"/checksums.txt", assetName)
	if err != nil {
		return nil, err
	}

	return &moduleBundleRelease{
		Version:          strings.TrimSpace(tag),
		ArtifactURL:      baseURL + "/" + assetName,
		ArtifactChecksum: "sha256:" + checksum,
	}, nil
}

func bundleAssetName(source moduleArtifactSource, targetArch string) (string, error) {
	arch := normalizeBundleArch(targetArch)
	if arch == "" {
		return "", fmt.Errorf("target agent architecture is unknown; reconnect agent with latest aurora-agent to report architecture")
	}
	return fmt.Sprintf("%s_linux_%s_bundle.tar.gz", strings.TrimSpace(source.BundleAssetBase), arch), nil
}

func fetchLatestGitHubReleaseTag(ctx context.Context, repoSlug string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", strings.TrimSpace(repoSlug))
	var payload githubLatestReleaseResponse
	if err := fetchJSON(ctx, url, &payload); err != nil {
		return "", fmt.Errorf("resolve latest release tag failed for %s: %w", strings.TrimSpace(repoSlug), err)
	}
	tag := strings.TrimSpace(payload.TagName)
	if tag == "" {
		return "", fmt.Errorf("resolve latest release tag failed for %s: empty tag_name", strings.TrimSpace(repoSlug))
	}
	return tag, nil
}

func fetchReleaseChecksum(ctx context.Context, url string, assetName string) (string, error) {
	body, err := fetchText(ctx, url)
	if err != nil {
		return "", fmt.Errorf("download release checksums failed: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		candidate := strings.TrimSpace(strings.TrimPrefix(fields[len(fields)-1], "dist/"))
		if candidate != strings.TrimSpace(assetName) {
			continue
		}
		sum := strings.TrimSpace(fields[0])
		if sum != "" {
			return sum, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan release checksums failed: %w", err)
	}
	return "", fmt.Errorf("checksum not found for asset %s", strings.TrimSpace(assetName))
}

func fetchJSON(ctx context.Context, url string, out any) error {
	body, err := fetchText(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(body), out); err != nil {
		return fmt.Errorf("decode json failed: %w", err)
	}
	return nil
}

func fetchText(ctx context.Context, url string) (string, error) {
	cacheKey := strings.TrimSpace(url)
	if cached, ok := loadReleaseMetadataCache(cacheKey); ok {
		return cached, nil
	}

	callCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, strings.TrimSpace(url), nil)
	if err != nil {
		return "", fmt.Errorf("build request failed: %w", err)
	}
	resp, err := releaseHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}
	body := strings.TrimSpace(string(bytes))
	storeReleaseMetadataCache(cacheKey, body)
	return body, nil
}

func normalizeBundleArch(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "amd64", "x86_64":
		return "amd64"
	case "arm64", "aarch64":
		return "arm64"
	default:
		return ""
	}
}
