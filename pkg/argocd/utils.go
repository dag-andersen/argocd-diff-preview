package argocd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
)

// ApplySecretsFromFolder applies all secret manifests from a folder using the Kubernetes API
func ApplySecretsFromFolder(client *utils.K8sClient, secretsFolder string, namespace string) error {
	// Check if folder exists
	if _, err := os.Stat(secretsFolder); os.IsNotExist(err) {
		log.Info().Msgf("🤷 No secrets folder found at %s", secretsFolder)
		return nil
	}

	// Apply all files in the secrets folder
	files, err := os.ReadDir(secretsFolder)
	if err != nil {
		return fmt.Errorf("failed to read secrets folder: %w", err)
	}

	secretCount := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Use the existing ApplyManifestFromFile method to apply each secret
		if err := client.ApplyManifestFromFile(filepath.Join(secretsFolder, file.Name()), namespace); err != nil {
			return fmt.Errorf("failed to apply secret %s: %w", file.Name(), err)
		}
		secretCount++
	}

	if secretCount > 0 {
		log.Info().Msgf("🤫 Applied %d secrets", secretCount)
	} else {
		log.Info().Msgf("🤷 No secrets found in %s", secretsFolder)
	}

	return nil
}
