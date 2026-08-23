package argocd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dag-andersen/argocd-diff-preview/pkg/k8s"
	"github.com/rs/zerolog/log"
)

// ApplyPreinstallFromFolder applies Kubernetes manifests needed by the ephemeral
// cluster before Argo CD is installed and applications are rendered.
func ApplyPreinstallFromFolder(client *k8s.Client, preinstallFolder string, namespace string) error {
	count, found, err := applyManifestsFromFolder(preinstallFolder, "preinstall", func(path string) (int, error) {
		return client.ApplyManifestFromFile(path, namespace)
	})
	if err != nil {
		return err
	}
	if count > 0 {
		log.Info().Msgf("📦 Applied %d preinstall manifests", count)
	} else if found {
		log.Info().Msgf("🤷 No preinstall manifests found in %s", preinstallFolder)
	}
	return nil
}

// ApplySecretsFromFolder applies all secret manifests from a folder using the Kubernetes API.
func ApplySecretsFromFolder(client *k8s.Client, secretsFolder string, namespace string) error {
	count, found, err := applyManifestsFromFolder(secretsFolder, "secret", func(path string) (int, error) {
		return client.ApplyManifestFromFile(path, namespace)
	})
	if err != nil {
		return err
	}
	if !found {
		log.Info().Msgf("🤷 No secrets folder found at %s", secretsFolder)
	} else if count > 0 {
		log.Info().Msgf("🤫 Applied %d secrets", count)
	} else {
		log.Info().Msgf("🤷 No secrets found in %s", secretsFolder)
	}
	return nil
}

// returns the number of manifests applied, whether the folder was found, and an error if any.
func applyManifestsFromFolder(folder string, manifestType string, apply func(path string) (int, error)) (int, bool, error) {
	if _, err := os.Stat(folder); err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to access %s folder: %w", manifestType, err)
	}

	files, err := os.ReadDir(folder)
	if err != nil {
		return 0, true, fmt.Errorf("failed to read %s folder: %w", manifestType, err)
	}

	manifestCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		count, err := apply(filepath.Join(folder, file.Name()))
		if err != nil {
			return manifestCount, true, fmt.Errorf("failed to apply %s %s: %w", manifestType, file.Name(), err)
		}
		manifestCount += count
	}

	return manifestCount, true, nil
}
