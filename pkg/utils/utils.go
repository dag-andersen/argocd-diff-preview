package utils

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
)

// RunCommand executes a command and returns its output
func RunCommand(cmd string) (string, error) {
	command := exec.Command("sh", "-c", cmd)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %s: %w", string(output), err)
	}
	return string(output), nil
}

// WriteFile writes content to a file
func WriteFile(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// WriteApplications writes applications to YAML files in the specified folder
func WriteApplications(
	apps []types.ArgoResource,
	branch *types.Branch,
	folder string,
) error {
	filePath := fmt.Sprintf("%s/%s.yaml", folder, branch.FolderName())
	log.Printf("💾 Writing %d Applications from '%s' to ./%s",
		len(apps), branch.Name, filePath)

	yaml := types.ApplicationsToString(apps)
	if err := WriteFile(filePath, yaml); err != nil {
		return fmt.Errorf("failed to write %s apps: %w", branch.Type(), err)
	}

	return nil
}
