package argocd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	maxMajorVersionDriftAllowed = 0
	maxMinorVersionDriftAllowed = 3
)

// getArgoCDLibVersion returns the version of the ArgoCD library from go.mod.
// Returns "unknown" if the version cannot be determined.
func getArgoCDLibVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	for _, dep := range info.Deps {
		// Match github.com/argoproj/argo-cd/v2 or v3, etc.
		if strings.HasPrefix(dep.Path, "github.com/argoproj/argo-cd/") {
			return dep.Version
		}
	}

	return "unknown"
}

// Check Argo CD CLI version vs Argo CD Server version
func (a *ArgoCDInstallation) CheckArgoCDCLIVersionVsServerVersion() error {
	var out string
	var err error
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		out, err = a.runArgocdCommand("version", "-o", "json")
		if err == nil {
			break
		}
		if attempt < maxRetries {
			log.Debug().Msgf("argocd version command failed (attempt %d/%d), retrying in 1s: %v", attempt, maxRetries, err)
			time.Sleep(1 * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("command 'argocd version -o json' failed: %w", err)
	}

	type versionInfo struct {
		Version string `json:"Version"`
	}

	type argocdVersionOutput struct {
		Client versionInfo `json:"client"`
		Server versionInfo `json:"server"`
	}

	var versionOutput argocdVersionOutput
	if err := json.Unmarshal([]byte(out), &versionOutput); err != nil {
		return fmt.Errorf("failed to parse argocd version output: %w", err)
	}

	log.Debug().Msgf("Argo CD Version: [CLI: '%s', Server: '%s']", versionOutput.Client.Version, versionOutput.Server.Version)

	clientMajor, clientMinor, err := extractMajorMinorVersion(versionOutput.Client.Version)
	if err != nil {
		return fmt.Errorf("failed to extract major minor version from cli version: %w", err)
	}
	serverMajor, serverMinor, err := extractMajorMinorVersion(versionOutput.Server.Version)
	if err != nil {
		return fmt.Errorf("failed to extract major minor version from server version: %w", err)
	}

	majorDrift, minorDrift := checkVersionDrift(clientMajor, clientMinor, serverMajor, serverMinor)
	if majorDrift {
		log.Warn().Msgf("⚠️ Argo CD CLI major version (%d.%d) differs from server major version (%d.%d). This may cause compatibility issues.", clientMajor, clientMinor, serverMajor, serverMinor)
	} else if minorDrift {
		log.Warn().Msgf("⚠️ Argo CD CLI minor version (%d.%d) differs significantly from server minor version (%d.%d). This may cause compatibility issues.", clientMajor, clientMinor, serverMajor, serverMinor)
	}

	return nil
}

// extractMajorMinorVersion extracts the major and minor version from a version string like "v3.2.2+8d0dde1"
func extractMajorMinorVersion(version string) (int, int, error) {
	// Remove leading 'v' if present
	version = strings.TrimPrefix(version, "v")

	// Split by '.' and parse major and minor
	parts := strings.Split(version, ".")
	var major, minor int
	if len(parts) >= 1 {
		if _, err := fmt.Sscanf(parts[0], "%d", &major); err != nil {
			return 0, 0, fmt.Errorf("failed to parse major version from string '%s': %w", parts[0], err)
		}
	}
	if len(parts) >= 2 {
		if _, err := fmt.Sscanf(parts[1], "%d", &minor); err != nil {
			return 0, 0, fmt.Errorf("failed to parse minor version from string '%s': %w", parts[1], err)
		}
	}
	return major, minor, nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// checkVersionDrift checks if there's a significant version drift between client and server
// Returns: majorDrift (bool), minorDrift (bool)
func checkVersionDrift(clientMajor, clientMinor, serverMajor, serverMinor int) (bool, bool) {
	majorDrift := abs(clientMajor-serverMajor) > maxMajorVersionDriftAllowed
	minorDrift := abs(clientMinor-serverMinor) > maxMinorVersionDriftAllowed
	return majorDrift, minorDrift
}

// CheckArgoCDLibVersionVsServerVersion compares the ArgoCD library version (from go.mod)
// against the ArgoCD server version. This is used when running in API mode instead of CLI mode.
func (a *ArgoCDInstallation) CheckArgoCDLibVersionVsServerVersion() error {
	libVersion := getArgoCDLibVersion()
	if libVersion == "unknown" {
		return fmt.Errorf("failed to determine ArgoCD library version from build info")
	}

	serverVersion, err := a.getServerVersion()
	if err != nil {
		return fmt.Errorf("failed to get server version: %w", err)
	}

	log.Error().Msgf("Argo CD Version: [Lib: '%s', Server: '%s']", libVersion, serverVersion)

	libMajor, libMinor, err := extractMajorMinorVersion(libVersion)
	if err != nil {
		return fmt.Errorf("failed to extract major minor version from lib version: %w", err)
	}
	serverMajor, serverMinor, err := extractMajorMinorVersion(serverVersion)
	if err != nil {
		return fmt.Errorf("failed to extract major minor version from server version: %w", err)
	}

	majorDrift, minorDrift := checkVersionDrift(libMajor, libMinor, serverMajor, serverMinor)
	if majorDrift {
		log.Warn().Msgf("⚠️ Argo CD library major version (%d.%d) differs from server major version (%d.%d). This may cause compatibility issues.", libMajor, libMinor, serverMajor, serverMinor)
	} else if minorDrift {
		log.Warn().Msgf("⚠️ Argo CD library minor version (%d.%d) differs significantly from server minor version (%d.%d). This may cause compatibility issues.", libMajor, libMinor, serverMajor, serverMinor)
	}

	return nil
}

// getServerVersion fetches the ArgoCD server version via the API
func (a *ArgoCDInstallation) getServerVersion() (string, error) {
	// Ensure port forward is active
	if err := a.portForwardToArgoCD(); err != nil {
		return "", fmt.Errorf("failed to set up port forward: %w", err)
	}

	url := fmt.Sprintf("%s/api/version", a.ArgoCDApiConnection.apiServerURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var versionResponse struct {
		Version string `json:"Version"`
	}

	if err := json.Unmarshal(body, &versionResponse); err != nil {
		return "", fmt.Errorf("failed to parse version response: %w", err)
	}

	return versionResponse.Version, nil
}
