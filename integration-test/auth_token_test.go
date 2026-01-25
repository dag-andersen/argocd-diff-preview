package integration_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Test configuration constants for auth token test
const (
	authTestNamespace   = "argocd-auth-test"
	authTestClusterName = "argocd-auth-test"
	testUsername        = "testuser"
)

// TestAuthToken tests authentication using a locally created ArgoCD user and token.
// This test uses Docker mode only, which provides inherent isolation (clean filesystem
// without any prior ~/.config/argocd/config state), making it a true test of standalone token auth.
//
// This test:
// 1. Creates a kind cluster
// 2. Installs ArgoCD with a local user configured (apiKey capability only)
// 3. Generates an auth token for that user using the admin account
// 4. Runs argocd-diff-preview with the auth token (CLI mode inside container)
// 5. Runs argocd-diff-preview with the auth token (API mode inside container)
//
// Usage:
//
//	cd integration-test && go test -v -timeout 15m -run TestAuthToken ./...
func TestAuthToken(t *testing.T) {
	// Skip if not explicitly running this test
	if os.Getenv("RUN_AUTH_TOKEN_TEST") != "true" && !testing.Verbose() {
		t.Skip("Skipping auth token test. Set RUN_AUTH_TOKEN_TEST=true or use -v to run.")
	}

	// Parse flags if not already done
	if !flag.Parsed() {
		flag.Parse()
	}

	// Setup: delete any existing cluster and create a fresh one
	t.Log("Setting up auth token test environment...")

	// Delete existing test cluster if any
	deleteAuthTestCluster()

	// Clean up on test completion
	t.Cleanup(func() {
		t.Log("Cleaning up auth token test cluster...")
		deleteAuthTestCluster()
	})

	// Step 1: Create kind cluster
	t.Log("Step 1: Creating kind cluster...")
	if err := createAuthTestCluster(); err != nil {
		t.Fatalf("Failed to create kind cluster: %v", err)
	}

	// Step 2: Install ArgoCD with local user configured
	t.Log("Step 2: Installing ArgoCD with local user...")
	if err := installArgoCDWithLocalUser(); err != nil {
		t.Fatalf("Failed to install ArgoCD: %v", err)
	}

	// Step 3: Wait for ArgoCD to be ready
	t.Log("Step 3: Waiting for ArgoCD to be ready...")
	if err := waitForArgoCDReady(); err != nil {
		t.Fatalf("Failed waiting for ArgoCD: %v", err)
	}

	// Step 4: Get admin password
	t.Log("Step 4: Getting admin password...")
	adminPassword, err := getAdminPassword()
	if err != nil {
		t.Fatalf("Failed to get admin password: %v", err)
	}
	t.Logf("Got admin password")

	// Step 5: Generate auth token for testuser using admin credentials
	t.Log("Step 5: Generating auth token for testuser...")
	authToken, err := generateAuthTokenForUser(adminPassword, testUsername)
	if err != nil {
		t.Fatalf("Failed to generate auth token: %v", err)
	}
	t.Logf("Generated auth token: %s...", authToken[:min(20, len(authToken))])

	// Step 6: Clone test branches
	t.Log("Step 6: Cloning test branches...")
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}

	baseBranchDir := filepath.Join(repoRoot, "base-branch")
	targetBranchDir := filepath.Join(repoRoot, "target-branch")
	outputDir := filepath.Join(repoRoot, "output")

	// Clean up directories
	_ = os.RemoveAll(baseBranchDir)
	_ = os.RemoveAll(targetBranchDir)
	_ = os.RemoveAll(outputDir)

	if err := cloneBranch("integration-test/branch-1/base", baseBranchDir); err != nil {
		t.Fatalf("Failed to clone base branch: %v", err)
	}
	if err := cloneBranch("integration-test/branch-1/target", targetBranchDir); err != nil {
		t.Fatalf("Failed to clone target branch: %v", err)
	}

	// Step 7: Run argocd-diff-preview with the auth token (CLI mode)
	t.Log("Step 7: Running argocd-diff-preview with auth token (CLI mode)...")
	if err := runDiffPreviewWithToken(authToken, repoRoot, false /* useAPI */); err != nil {
		t.Fatalf("Failed to run argocd-diff-preview (CLI mode): %v", err)
	}

	// Verify output was created for CLI mode
	mdPath := filepath.Join(outputDir, "diff.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Fatalf("Output file not created (CLI mode): %s", mdPath)
	}
	t.Log("✅ CLI mode with auth token passed!")

	// Clean up output for next test
	_ = os.RemoveAll(outputDir)

	// Step 8: Run argocd-diff-preview with the auth token (API mode)
	t.Log("Step 8: Running argocd-diff-preview with auth token (API mode)...")
	if err := runDiffPreviewWithToken(authToken, repoRoot, true /* useAPI */); err != nil {
		t.Fatalf("Failed to run argocd-diff-preview (API mode): %v", err)
	}

	// Verify output was created for API mode
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Fatalf("Output file not created (API mode): %s", mdPath)
	}
	t.Log("✅ API mode with auth token passed!")

	t.Log("✅ Auth token test passed successfully for both CLI and API modes!")
}

// deleteAuthTestCluster deletes the auth test kind cluster
func deleteAuthTestCluster() {
	cmd := exec.Command("kind", "delete", "cluster", "--name", authTestClusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// createAuthTestCluster creates a kind cluster for auth testing
func createAuthTestCluster() error {
	cmd := exec.Command("kind", "create", "cluster", "--name", authTestClusterName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installArgoCDWithLocalUser installs ArgoCD using Helm with a local user configured
func installArgoCDWithLocalUser() error {
	// Add argo helm repo
	cmd := exec.Command("helm", "repo", "add", "argo", "https://argoproj.github.io/argo-helm")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add helm repo: %w", err)
	}

	// Update helm repo
	cmd = exec.Command("helm", "repo", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update helm repo: %w", err)
	}

	// Get the path to the values file
	valuesPath := filepath.Join(".", "localUserValues.yaml")

	// Install ArgoCD with the local user values
	cmd = exec.Command("helm", "install", "argocd", "argo/argo-cd",
		"--create-namespace",
		"--namespace", authTestNamespace,
		"--values", valuesPath,
		"--wait",
		"--timeout", "5m",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// waitForArgoCDReady waits for ArgoCD server to be ready
func waitForArgoCDReady() error {
	cmd := exec.Command("kubectl", "wait", "--for=condition=ready", "pod",
		"-l", "app.kubernetes.io/name=argocd-server",
		"-n", authTestNamespace,
		"--timeout=300s",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// getAdminPassword retrieves the initial admin password from the secret
func getAdminPassword() (string, error) {
	cmd := exec.Command("kubectl", "get", "secret", "argocd-initial-admin-secret",
		"-n", authTestNamespace,
		"-o", "jsonpath={.data.password}",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to get admin secret: %w", err)
	}

	// Decode base64
	encodedPassword := strings.TrimSpace(out.String())
	cmd = exec.Command("base64", "-d")
	cmd.Stdin = strings.NewReader(encodedPassword)
	var decoded bytes.Buffer
	cmd.Stdout = &decoded
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to decode password: %w", err)
	}

	return strings.TrimSpace(decoded.String()), nil
}

// generateAuthTokenForUser generates an auth token for a user using admin credentials
// This logs in as admin and generates a token for the specified user account
func generateAuthTokenForUser(adminPassword, username string) (string, error) {
	// Start port-forward in background
	portForward := exec.Command("kubectl", "port-forward",
		"svc/argocd-server", "8443:443",
		"-n", authTestNamespace,
	)
	portForward.Stdout = os.Stdout
	portForward.Stderr = os.Stderr
	if err := portForward.Start(); err != nil {
		return "", fmt.Errorf("failed to start port-forward: %w", err)
	}
	defer func() {
		_ = portForward.Process.Kill()
	}()

	// Wait for port-forward to be ready
	time.Sleep(3 * time.Second)

	// Login as admin
	loginCmd := exec.Command("argocd", "login", "localhost:8443",
		"--username", "admin",
		"--password", adminPassword,
		"--insecure",
	)
	loginCmd.Stdout = os.Stdout
	loginCmd.Stderr = os.Stderr
	if err := loginCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to login as admin: %w", err)
	}

	// Generate token for the specified user (admin can generate tokens for any user)
	cmd := exec.Command("argocd", "account", "generate-token",
		"--account", username,
		"--server", "localhost:8443",
		"--insecure",
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to generate token for %s: %w", username, err)
	}

	return strings.TrimSpace(out.String()), nil
}

// runDiffPreviewWithToken runs argocd-diff-preview Docker image with the provided auth token.
// Docker mode is inherently isolated - the container has a clean filesystem
// without any prior ~/.config/argocd/config state, making it a true test of standalone token auth.
func runDiffPreviewWithToken(authToken string, repoRoot string, useAPI bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	args := []string{
		"run",
		"--rm",
		"--network=host",
		"-v", fmt.Sprintf("%s/.kube:/root/.kube", homeDir),
		"-v", fmt.Sprintf("%s/base-branch:/base-branch", repoRoot),
		"-v", fmt.Sprintf("%s/target-branch:/target-branch", repoRoot),
		"-v", fmt.Sprintf("%s/output:/output", repoRoot),
		"-e", "BASE_BRANCH=integration-test/branch-1/base",
		"-e", "TARGET_BRANCH=integration-test/branch-1/target",
		"-e", "REPO=dag-andersen/argocd-diff-preview",
		"-e", fmt.Sprintf("ARGOCD_NAMESPACE=%s", authTestNamespace),
		"-e", fmt.Sprintf("ARGOCD_AUTH_TOKEN=%s", authToken),
		"-e", "CREATE_CLUSTER=false",
		"-e", "TIMEOUT=120",
		"-e", "FILES_CHANGED=examples/helm/applications/nginx.yaml",
	}

	if useAPI {
		args = append(args, "-e", "USE_ARGOCD_API=true")
	}

	args = append(args, *dockerImage)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
