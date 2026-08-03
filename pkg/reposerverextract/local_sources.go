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
	if primarySource.Helm == nil {
		for _, ref := range refSources {
			if ok, reason := localPathExists(localRefSourceRoot(branchFolder, ref), fmt.Sprintf("ref source %q", ref.Ref)); !ok {
				return false, reason
			}
		}
		return true, ""
	}

	refsByName := make(map[string]v1alpha1.ApplicationSource, len(refSources))
	for _, ref := range refSources {
		refsByName[ref.Ref] = ref
	}

	checkedRefs := map[string]bool{}
	for _, valueFile := range primarySource.Helm.ValueFiles {
		refName, refPath, ok := splitRefPath(valueFile)
		if !ok {
			continue
		}
		ref, found := refsByName[refName]
		if !found {
			continue
		}
		checkedRefs[refName] = true
		if ok, reason := localPathExists(filepath.Join(localRefSourceRoot(branchFolder, ref), refPath), fmt.Sprintf("ref value file %q", valueFile)); !ok {
			// With ignoreMissingValueFiles the file is optional by declaration, and
			// Argo CD skips it when rendering. The branch folder is a full checkout
			// of the very commit being rendered, so absent here means absent there —
			// falling back to the remote RPC cannot produce the file, it only costs
			// a round trip per app. Common shape: an overrides/<cluster>.yaml that
			// exists for a few clusters and not the rest.
			if primarySource.Helm.IgnoreMissingValueFiles {
				continue
			}
			return false, reason
		}
	}

	for _, ref := range refSources {
		if checkedRefs[ref.Ref] {
			continue
		}
		if ok, reason := localPathExists(localRefSourceRoot(branchFolder, ref), fmt.Sprintf("ref source %q", ref.Ref)); !ok {
			return false, reason
		}
	}

	return true, ""
}

func localRefSourceRoot(branchFolder string, ref v1alpha1.ApplicationSource) string {
	if ref.Path == "" {
		return branchFolder
	}
	return filepath.Join(branchFolder, ref.Path)
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
