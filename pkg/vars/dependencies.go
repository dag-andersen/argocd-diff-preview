package vars

import (
	"runtime/debug"
	"strings"
)

// GetArgoCDModuleVersion returns the version of the ArgoCD module used by this build.
// It extracts this from Go's embedded build info at runtime.
// Returns "unknown" if the version cannot be determined.
func GetArgoCDModuleVersion() string {
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

// GetModuleVersion returns the version of a specific module used by this build.
// Returns "unknown" if the module is not found.
func GetModuleVersion(modulePath string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	for _, dep := range info.Deps {
		if dep.Path == modulePath {
			return dep.Version
		}
	}

	return "unknown"
}
