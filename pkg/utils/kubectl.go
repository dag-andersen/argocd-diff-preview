package utils

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

// ApplyManifest applies a Kubernetes manifest file using kubectl
func ApplyManifest(filePath string) error {
	log.Printf("🚀 Applying manifest: %s", filePath)

	// Try to apply the manifest multiple times in case of temporary failures
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		cmd := exec.Command("kubectl", "apply", "-f", filePath)
		output, err := cmd.CombinedOutput()

		if err == nil {
			log.Printf("✅ Successfully applied manifest: %s", filePath)
			return nil
		}

		log.Printf("⚠️ Failed to apply manifest (attempt %d/%d): %s\nError: %s",
			i+1, maxRetries, filePath, string(output))

		if i < maxRetries-1 {
			time.Sleep(2 * time.Second)
		}
	}

	return fmt.Errorf("failed to apply manifest after %d attempts: %s", maxRetries, filePath)
}

// DeleteManifest deletes a Kubernetes manifest file using kubectl
func DeleteManifest(filePath string) error {
	log.Printf("🗑️ Deleting manifest: %s", filePath)

	cmd := exec.Command("kubectl", "delete", "-f", filePath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("failed to delete manifest: %s\nError: %s", filePath, string(output))
	}

	log.Printf("✅ Successfully deleted manifest: %s", filePath)
	return nil
}

// DeleteApplications deletes all Argo CD applications
func DeleteApplications() error {
	log.Printf("🧼 Removing applications")

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
		log.Printf("🗑 Deleting Applications (attempt %d/%d)", i+1, retryCount)

		cmd := exec.Command("kubectl", "delete", "applications.argoproj.io", "--all", "-A", "--timeout", "10s")
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("⚠️ Delete command output: %s", string(output))
		}

		if verifyNoApps() {
			log.Printf("🧼 Removed applications successfully")
			return nil
		}

		if i < retryCount-1 {
			log.Printf("⚠️ Failed to delete applications. Retrying...")
			time.Sleep(5 * time.Second)
		}
	}

	return fmt.Errorf("failed to delete applications after %d retries", retryCount)
}
