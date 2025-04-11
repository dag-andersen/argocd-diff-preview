package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// kubectlApply applies a Kubernetes manifest file using kubectl
func kubectlApply(path string, extraArgs ...string) error {
	log.Debug().Str("path", path).Msg("Applying manifest")

	// check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Warn().Str("path", path).Msg("⚠️ File does not exist")
		return fmt.Errorf("file does not exist: %s", path)
	}

	// check if the file is empty
	if fileInfo, err := os.Stat(path); err == nil && fileInfo.Size() == 0 {
		log.Debug().Str("path", path).Msg("Applies 0 resources because file is empty")
		return nil
	}

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

		if err := kubectlApply(filepath.Join(secretsFolder, file.Name()), "-n", namespace); err != nil {
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

// KubectlApplyFromString applies a Kubernetes manifest from a string using kubectl
func KubectlApplyFromString(manifest string, extraArgs ...string) error {
	log.Debug().Msg("Applying manifest from string")

	if strings.TrimSpace(manifest) == "" {
		log.Debug().Msg("Skipping apply because manifest is empty")
		return nil
	}

	// Try to apply the manifest multiple times in case of temporary failures
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command("kubectl", append([]string{"apply", "-f", "-"}, extraArgs...)...)

		// Set stdin to the manifest string
		cmd.Stdin = bytes.NewBufferString(manifest)
		output, err := cmd.CombinedOutput()

		if err == nil {
			log.Debug().Msg("Successfully applied manifest from string")
			return nil
		}

		log.Warn().Err(err).Str("output", string(output)).Msgf("⚠️ Failed to apply manifest from string (attempt %d/%d)", i+1, maxRetries)

		if i < maxRetries-1 {
			time.Sleep(2 * time.Second)
		}
	}

	return fmt.Errorf("failed to apply manifest from string after %d attempts", maxRetries)
}
