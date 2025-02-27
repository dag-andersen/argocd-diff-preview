package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
)

const (
	dirMode = os.ModePerm // 0755 - read/write/execute for owner, read/execute for group and others
)

func YamlToString(input *yaml.Node) string {
	bytes, err := yaml.Marshal(input)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func YamlEqual(a, b *yaml.Node) bool {
	aStr, err := yaml.Marshal(a)
	if err != nil {
		return false
	}
	bStr, err := yaml.Marshal(b)
	if err != nil {
		return false
	}
	return string(aStr) == string(bStr)
}

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

// Create folder (clear its content if it exists)
func CreateFolder(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to delete folder: %v", err)
	}
	return os.MkdirAll(path, dirMode)
}

// WriteApplications writes applications to YAML files in the specified folder
func WriteApplications(
	apps []types.ArgoResource,
	branch *types.Branch,
	folder string,
) error {
	filePath := fmt.Sprintf("%s/%s.yaml", folder, branch.FolderName())
	log.Info().Msgf("💾 Writing %d Applications from '%s' to ./%s",
		len(apps), branch.Name, filePath)

	yaml := types.ApplicationsToString(apps)
	if err := WriteFile(filePath, yaml); err != nil {
		return fmt.Errorf("failed to write %s apps: %w", branch.Type(), err)
	}

	return nil
}
