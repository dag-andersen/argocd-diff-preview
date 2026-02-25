package extract

import (
	"crypto/sha256"
	"fmt"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// addApplicationPrefix prefixes the application name with the branch name and a unique ID.
// For Applications that use a CMP plugin, it also injects DIFF_NAME as a plugin env var
// set to the original name. ArgoCD exposes plugin env vars with an ARGOCD_ENV_ prefix,
// so CMP commands can reference $ARGOCD_ENV_DIFF_NAME to get the original app name.
func addApplicationPrefix(a *argoapplication.ArgoResource, prefix string) error {
	if a.Branch == "" {
		log.Warn().Str(a.Kind.ShortName(), a.GetLongName()).Msg("⚠️ Can't prefix application name with prefix because branch is empty")
		return nil
	}

	originalName := a.Name

	branchShortName := a.Branch.ShortName()

	maxKubernetesNameLength := 53

	prefixSize := len(prefix) + len(branchShortName) + len("--")
	var newId string
	if prefixSize+len(a.Id) > maxKubernetesNameLength {
		// hash id so it becomes shorter
		hashedId := fmt.Sprintf("%x", sha256.Sum256([]byte(a.Id)))
		hashPart := hashedId[:53-prefixSize]
		log.Debug().Msgf("Application name too long. Renamed '%s' to '%s'", a.Id, hashPart)
		newId = fmt.Sprintf("%s-%s-%s", prefix, branchShortName, hashPart)
	} else {
		newId = fmt.Sprintf("%s-%s-%s", prefix, branchShortName, a.Id)
	}

	a.Id = newId
	a.Yaml.SetName(newId)

	if a.Kind == argoapplication.Application {
		injected, err := injectDiffNameEnv(a, originalName)
		if err != nil {
			return fmt.Errorf("failed to inject DIFF_NAME into plugin env: %w", err)
		}
		if injected {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Injected DIFF_NAME=%s into plugin env", originalName)
		}
	}

	return nil
}

// injectDiffNameEnv adds a DIFF_NAME env var into any plugin source on the Application,
// so CMP plugins can reference $ARGOCD_ENV_DIFF_NAME to get the original app name.
// Returns true if at least one plugin source was found and modified.
func injectDiffNameEnv(a *argoapplication.ArgoResource, originalName string) (bool, error) {
	if a.Yaml == nil {
		return false, nil
	}

	envEntry := map[string]any{
		"name":  "DIFF_NAME",
		"value": originalName,
	}

	injected := false

	// Handle single source: spec.source.plugin
	if pluginMap, found, _ := unstructured.NestedMap(a.Yaml.Object, "spec", "source", "plugin"); found {
		pluginMap["env"] = append(nestedSliceOrNil(pluginMap, "env"), envEntry)
		if err := unstructured.SetNestedMap(a.Yaml.Object, pluginMap, "spec", "source", "plugin"); err != nil {
			return false, fmt.Errorf("failed to set plugin env: %w", err)
		}
		injected = true
	}

	// Handle multi-source: spec.sources[*].plugin
	sourcesSlice, found, _ := unstructured.NestedSlice(a.Yaml.Object, "spec", "sources")
	if !found {
		return injected, nil
	}

	modified := false
	for i, srcInterface := range sourcesSlice {
		srcMap, ok := srcInterface.(map[string]any)
		if !ok {
			continue
		}
		pluginMap, ok := srcMap["plugin"].(map[string]any)
		if !ok {
			continue
		}
		pluginMap["env"] = append(nestedSliceOrNil(pluginMap, "env"), envEntry)
		srcMap["plugin"] = pluginMap
		sourcesSlice[i] = srcMap
		modified = true
	}

	if modified {
		if err := unstructured.SetNestedSlice(a.Yaml.Object, sourcesSlice, "spec", "sources"); err != nil {
			return false, fmt.Errorf("failed to set sources slice: %w", err)
		}
		injected = true
	}

	return injected, nil
}

// nestedSliceOrNil returns the []any at key in m, or nil if absent/wrong type.
func nestedSliceOrNil(m map[string]any, key string) []any {
	s, _ := m[key].([]any)
	return s
}
