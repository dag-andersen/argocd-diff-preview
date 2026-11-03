package reposerverextract

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func isLocalSourceMissing(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func localContentAndRefSourcesAvailable(branchFolder string, primarySource v1alpha1.ApplicationSource, refSources []v1alpha1.ApplicationSource) (bool, string) {
	if primarySource.Path != "" {
		if ok, reason := localPathExists(filepath.Join(branchFolder, primarySource.Path), "content source"); !ok {
			return false, reason
		}
	}
	return localRefSourcesAvailable(branchFolder, primarySource, refSources)
}

func localRefSourcesAvailable(branchFolder string, primarySource v1alpha1.ApplicationSource, refSources []v1alpha1.ApplicationSource) (bool, string) {
	// All local refs resolve from the repository root. A ref source's Path
	// selects its own manifests; it does not change the root for Helm refs.
	if len(refSources) > 0 {
		if ok, reason := localPathExists(branchFolder, "ref repository"); !ok {
			return false, reason
		}
	}
	if primarySource.Helm == nil {
		return true, ""
	}

	refsByName := make(map[string]struct{}, len(refSources))
	for _, ref := range refSources {
		refsByName[ref.Ref] = struct{}{}
	}

	for _, valueFile := range primarySource.Helm.ValueFiles {
		refName, refPath, ok := splitRefPath(valueFile)
		if !ok {
			continue
		}
		_, found := refsByName[refName]
		if !found {
			continue
		}
		valueFilePath := filepath.Join(branchFolder, refPath)
		if _, err := os.Stat(valueFilePath); err != nil {
			// With ignoreMissingValueFiles a nonexistent file is optional by
			// declaration, and Argo CD skips it when rendering. The branch folder is
			// a full checkout of the commit being rendered, so falling back to the
			// remote RPC cannot make the file appear.
			if primarySource.Helm.IgnoreMissingValueFiles && errors.Is(err, os.ErrNotExist) {
				continue
			}
			if errors.Is(err, os.ErrNotExist) {
				return false, fmt.Sprintf("ref value file %q path %q does not exist", valueFile, valueFilePath)
			}
			return false, fmt.Sprintf("failed to inspect ref value file %q path %q: %v", valueFile, valueFilePath, err)
		}
	}

	return true, ""
}

func localPathExists(path, description string) (bool, string) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, fmt.Sprintf("%s path %q does not exist", description, path)
		}
		return false, fmt.Sprintf("failed to inspect %s path %q: %v", description, path, err)
	}
	return true, ""
}
