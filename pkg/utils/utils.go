package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
