package argocd

import (
	"fmt"
	"strings"
)

var (
	maxMajorVersionDriftAllowed = 0
	maxMinorVersionDriftAllowed = 3
)

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
