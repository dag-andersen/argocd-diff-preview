package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// KubectlApply applies a Kubernetes manifest file using kubectl
func KubectlApply(path string, extraArgs ...string) error {
	log.Debug().Str("path", path).Msg("Applying manifest")

	// Try to apply the manifest multiple times in case of temporary failures
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command("kubectl", append([]string{"apply", "-f", path}, extraArgs...)...)
		output, err := cmd.CombinedOutput()

		if err == nil {
			log.Debug().Str("path", path).Msg("Successfully applied manifest")
			return nil
		}

		log.Warn().Err(err).Str("path", path).Str("output", string(output)).Msgf("⚠️ Failed to apply manifest (attempt %d/%d)", i+1, maxRetries)

		if i < maxRetries-1 {
			time.Sleep(2 * time.Second)
		}
	}

	return fmt.Errorf("failed to apply manifest after %d attempts: %s", maxRetries, path)
}

// DeleteManifest deletes a Kubernetes manifest file using kubectl
func DeleteManifest(filePath string) error {
	log.Debug().Str("path", filePath).Msg("Deleting manifest")

	cmd := exec.Command("kubectl", "delete", "-f", filePath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to delete manifest: %s\nError: %s", filePath, string(output))
	}

	log.Debug().Str("path", filePath).Msg("Successfully deleted manifest")
	return nil
}

// DeleteApplications deletes all Argo CD applications
func DeleteApplications() error {
	log.Info().Msg("🧼 Removing applications")

	verifyNoApps := func() bool {
		cmd := exec.Command("kubectl", "get", "applications", "-A", "--no-headers")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return false
		}
		return len(strings.TrimSpace(string(output))) == 0 || strings.Contains(string(output), "No resources found")
	}

	retryCount := 3
	for i := 0; i < retryCount; i++ {
		log.Info().Msgf("🗑 Deleting Applications (attempt %d/%d)", i+1, retryCount)

		cmd := exec.Command("kubectl", "delete", "applications.argoproj.io", "--all", "-A", "--timeout", "10s")
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Warn().Err(err).Str("output", string(output)).Msg("⚠️ Delete command output")
		}

		if verifyNoApps() {
			log.Info().Msg("🧼 Removed applications successfully")
			return nil
		}

		if i < retryCount-1 {
			log.Warn().Msg("⚠️ Failed to delete applications. Retrying...")
			time.Sleep(5 * time.Second)
		}
	}

	return fmt.Errorf("failed to delete applications after %d retries", retryCount)
}

// ApplySecretsFromFolder applies all secrets from a folder using kubectl
func ApplySecretsFromFolder(secretsFolder string, namespace string) error {
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

		if err := KubectlApply(filepath.Join(secretsFolder, file.Name()), "-n", namespace); err != nil {
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
