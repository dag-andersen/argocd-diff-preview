package argocd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

var (
	maxMajorVersionDriftAllowed = 0
	maxMinorVersionDriftAllowed = 3
)

// Check Argo CD CLI version vs Argo CD Server version
func (a *ArgoCDInstallation) CheckArgoCDCLIVersionVsServerVersion() error {
	out, err := a.runArgocdCommand("version", "-o", "json")
	if err != nil {
		return fmt.Errorf("failed to check argocd cli version vs server version: %w", err)
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
