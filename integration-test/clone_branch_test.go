package integration_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCloneBranchFromLocalFixtureDir(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	targetDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcDir, "nested"), 0o755); err != nil {
		t.Fatalf("failed to create nested source dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(srcDir, ".git"), 0o755); err != nil {
		t.Fatalf("failed to create fake git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "app.yaml"), []byte("kind: Application\n"), 0o644); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "nested", "config.yaml"), []byte("kind: ConfigMap\n"), 0o644); err != nil {
		t.Fatalf("failed to write nested source file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("failed to write fake git metadata: %v", err)
	}

	if err := cloneBranch("dir:"+srcDir, targetDir); err != nil {
		t.Fatalf("cloneBranch returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "app.yaml")); err != nil {
		t.Fatalf("expected app.yaml to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "nested", "config.yaml")); err != nil {
		t.Fatalf("expected nested/config.yaml to be copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".gitignore")); err != nil {
		t.Fatalf("expected .gitignore to be created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(targetDir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected .git directory to be skipped, got err=%v", err)
	}
}
