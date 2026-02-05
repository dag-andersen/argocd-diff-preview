package integration_test

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// Flags for test configuration
var (
	update        = flag.Bool("update", false, "update expected output files with actual output")
	useDocker     = flag.Bool("docker", false, "use Docker instead of Go binary")
	debug         = flag.Bool("debug", false, "enable debug mode for the tool")
	createCluster = flag.Bool("create-cluster", false, "force creation of a new cluster (deletes existing one)")
	useArgocdApi  = flag.Bool("use-argocd-api", false, "force all tests to use the Argo CD API instead of CLI")
	binaryPath    = flag.String("binary", "./bin/argocd-diff-preview", "path to the Go binary (relative to repo root)")
	dockerImage   = flag.String("image", "argocd-diff-preview", "Docker image name to use")
)

// Test configuration constants
const (
	defaultGitHubOrg  = "dag-andersen"
	defaultGitOpsRepo = "argocd-diff-preview"
	defaultTimeout    = "90"
	defaultLineCount  = "10"
	defaultMaxDiffLen = "65536"
	defaultTitle      = "Argo CD Diff Preview"
	argocdNamespace   = "argocd-diff-preview"
)

// TestCase defines a single integration test case
type TestCase struct {
	Name                       string
	TargetBranch               string
	BaseBranch                 string
	Suffix                     string // Used for multiple test variations on same branch
	LineCount                  string
	DiffIgnore                 string
	FilesChanged               string
	Selector                   string
	FileRegex                  string
	Title                      string
	KindOptions                string
	CreateCluster              string
	MaxDiffLength              string
	WatchIfNoWatchPatternFound string
	AutoDetectFilesChanged     string
	IgnoreInvalidWatchPattern  string
	HideDeletedAppDiff         string
	IgnoreResources            string
	ArgocdLoginOptions         string
	ArgocdAuthToken            string // Auth token for Argo CD (if set, will be used instead of login)
	UseArgocdApi               string // "true" = force enable, "false" = force disable, "" = use global flag
	DisableClusterRoles        string // Mount createClusterRoles.yaml (sets createClusterRoles: false)
	ArgocdUIURL                string // Argo CD URL for generating application links in diff output
	ExpectFailure              bool   // If true, the test is expected to fail
}

// testCases defines all integration test cases matching the Makefile
var testCases = []TestCase{
	{
		Name:          "branch-1/target-1",
		TargetBranch:  "integration-test/branch-1/target",
		BaseBranch:    "integration-test/branch-1/base",
		Suffix:        "-1",
		LineCount:     "3",
		KindOptions:   "--name tests --config=./kind-config/options.yaml",
		CreateCluster: "true",
	},
	{
		Name:         "branch-1/target-2",
		TargetBranch: "integration-test/branch-1/target",
		BaseBranch:   "integration-test/branch-1/base",
		Suffix:       "-2",
		DiffIgnore:   "image",
	},
	{
		Name:               "branch-1/target-3",
		TargetBranch:       "integration-test/branch-1/target",
		BaseBranch:         "integration-test/branch-1/base",
		Suffix:             "-3",
		HideDeletedAppDiff: "true",
		ArgocdLoginOptions: "--insecure",
		ArgocdUIURL:        "https://argocd.example.com",
	},
	{
		Name:         "branch-2/target",
		TargetBranch: "integration-test/branch-2/target",
		BaseBranch:   "integration-test/branch-2/base",
	},
	{
		Name:         "branch-3/target",
		TargetBranch: "integration-test/branch-3/target",
		BaseBranch:   "integration-test/branch-3/base",
	},
	{
		Name:         "branch-4/target",
		TargetBranch: "integration-test/branch-4/target",
		BaseBranch:   "integration-test/branch-4/base",
		Title:        "integration-test/branch-4",
	},
	{
		Name:         "branch-5/target-1",
		TargetBranch: "integration-test/branch-5/target",
		BaseBranch:   "integration-test/branch-5/base",
		Suffix:       "-1",
		FilesChanged: "examples/helm/values/filtered.yaml",
	},
	{
		Name:         "branch-5/target-2",
		TargetBranch: "integration-test/branch-5/target",
		BaseBranch:   "integration-test/branch-5/base",
		Suffix:       "-2",
		FilesChanged: "examples/helm/applications/watch-pattern/valid-regex.yaml",
	},
	{
		Name:         "branch-5/target-3",
		TargetBranch: "integration-test/branch-5/target",
		BaseBranch:   "integration-test/branch-5/base",
		Suffix:       "-3",
		FilesChanged: "something/else.yaml",
	},
	{
		Name:         "branch-5/target-4",
		TargetBranch: "integration-test/branch-5/target",
		BaseBranch:   "integration-test/branch-5/base",
		Suffix:       "-4",
		Selector:     "team=my-team",
		ArgocdUIURL:  "https://argocd.example.com",
	},
	{
		Name:         "branch-5/target-5",
		TargetBranch: "integration-test/branch-5/target",
		BaseBranch:   "integration-test/branch-5/base",
		Suffix:       "-5",
		Selector:     "team=other-team",
		Title:        "integration-test/branch-5",
	},
	{
		Name:         "branch-5/target-6",
		TargetBranch: "integration-test/branch-5/target",
		BaseBranch:   "integration-test/branch-5/base",
		Suffix:       "-6",
		FileRegex:    ".*labels\\.yaml",
	},
	{
		Name:         "branch-5/target-7",
		TargetBranch: "integration-test/branch-5/target",
		BaseBranch:   "integration-test/branch-5/base",
		Suffix:       "-7",
		FileRegex:    "this-does-not-exist\\.yaml",
	},
	{
		Name:                       "branch-5/target-8",
		TargetBranch:               "integration-test/branch-5/target",
		BaseBranch:                 "integration-test/branch-5/base",
		Suffix:                     "-8",
		WatchIfNoWatchPatternFound: "true",
		FilesChanged:               "something/else.yaml",
	},
	{
		Name:                       "branch-5/target-9",
		TargetBranch:               "integration-test/branch-5/target",
		BaseBranch:                 "integration-test/branch-5/base",
		Suffix:                     "-9",
		WatchIfNoWatchPatternFound: "true",
		AutoDetectFilesChanged:     "true",
		UseArgocdApi:               "true",
	},
	{
		Name:         "branch-6/target",
		TargetBranch: "integration-test/branch-6/target",
		BaseBranch:   "integration-test/branch-6/base",
	},
	{
		Name:          "branch-7/target",
		TargetBranch:  "integration-test/branch-7/target",
		BaseBranch:    "integration-test/branch-7/base",
		FilesChanged:  "examples/helm/values/filtered.yaml",
		CreateCluster: "true",
	},
	{
		Name:         "branch-8/target",
		TargetBranch: "integration-test/branch-8/target",
		BaseBranch:   "integration-test/branch-8/base",
		FilesChanged: "examples/git-generator/resources/folder2/deployment.yaml,examples/git-generator/resources/folder3/deployment.yaml",
		ArgocdUIURL:  "https://argocd.example.com",
	},
	{
		Name:          "branch-9/target-1",
		TargetBranch:  "integration-test/branch-9/target",
		BaseBranch:    "integration-test/branch-9/base",
		Suffix:        "-1",
		MaxDiffLength: "10000",
	},
	{
		Name:          "branch-9/target-2",
		TargetBranch:  "integration-test/branch-9/target",
		BaseBranch:    "integration-test/branch-9/base",
		Suffix:        "-2",
		FilesChanged:  "examples/external-chart/nginx.yaml",
		MaxDiffLength: "900",
	},
	{
		Name:         "branch-10/target-1",
		TargetBranch: "integration-test/branch-10/target",
		BaseBranch:   "integration-test/branch-10/base",
		Suffix:       "-1",
		FilesChanged: "examples/ignore-differences/app.yaml",
	},
	{
		Name:                   "branch-11/target-1",
		TargetBranch:           "integration-test/branch-11/target",
		BaseBranch:             "integration-test/branch-11/base",
		Suffix:                 "-1",
		AutoDetectFilesChanged: "true",
	},
	{
		Name:                   "branch-12/target-1",
		TargetBranch:           "integration-test/branch-12/target",
		BaseBranch:             "integration-test/branch-12/base",
		Suffix:                 "-1",
		AutoDetectFilesChanged: "true",
		DiffIgnore:             "annotations",
	},
	{
		Name:                   "branch-12/target-2",
		TargetBranch:           "integration-test/branch-12/target",
		BaseBranch:             "integration-test/branch-12/base",
		Suffix:                 "-2",
		AutoDetectFilesChanged: "true",
		DiffIgnore:             "annotations",
		IgnoreResources:        "*:CustomResourceDefinition:*,:ConfigMap:argocd-cm",
	},
	{
		Name:         "branch-13/target-1",
		TargetBranch: "integration-test/branch-13/target",
		BaseBranch:   "integration-test/branch-13/base",
		Suffix:       "-1",
	},
	{
		Name:         "branch-13/target-2",
		TargetBranch: "integration-test/branch-13/target",
		BaseBranch:   "integration-test/branch-13/base",
		Suffix:       "-2",
		Selector:     "team=your-team",
	},
	// This test verifies that disabling cluster roles without using the API fails.
	// When createClusterRoles: false is set but --use-argocd-api is not used,
	// the tool should fail because it can't access cluster resources via CLI.
	// NOTE: This test MUST create a new cluster because the role changes only take
	// effect when ArgoCD is installed during cluster creation.
	// NOTE: UseArgocdApi is explicitly set to "false" to override the global flag,
	// because this test specifically tests the failure case without the API.
	{
		Name:                   "branch-1/target-no-cluster-roles",
		TargetBranch:           "integration-test/branch-1/target",
		BaseBranch:             "integration-test/branch-1/base",
		Suffix:                 "-no-cluster-roles",
		DisableClusterRoles:    "true",
		CreateCluster:          "true",
		UseArgocdApi:           "false",
		AutoDetectFilesChanged: "true",
		ExpectFailure:          true,
	},
	// Test that an invalid auth token causes the tool to fail.
	// This tests the token authentication path instead of username/password login.
	{
		Name:            "branch-1/target-invalid-token",
		TargetBranch:    "integration-test/branch-1/target",
		BaseBranch:      "integration-test/branch-1/base",
		Suffix:          "-invalid-token",
		ArgocdAuthToken: "abc",
		FilesChanged:    "examples/helm/applications/nginx.yaml",
		ExpectFailure:   true,
	},
}

// timePattern matches timing information in output that varies between runs
// Matches patterns like "1m10s]", "24s]", "110s]"
var timePattern = regexp.MustCompile(`\d+m?\d*s\]`)

// normalizeOutput removes timing information that varies between runs
func normalizeOutput(s string) string {
	return timePattern.ReplaceAllString(s, "Xs]")
}

// TestIntegration runs all integration tests
func TestIntegration(t *testing.T) {
	// Ensure we're in the integration-test directory
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Check if we're in the integration-test directory, if not, change to it
	if !strings.HasSuffix(testDir, "/integration-test") && !strings.HasSuffix(testDir, "\\integration-test") {
		testDir = filepath.Join(testDir, "integration-test")
		if err := os.Chdir(testDir); err != nil {
			t.Fatalf("Failed to change to integration-test directory: %v", err)
		}
	}

	// Build docker image if using docker mode
	if *useDocker {
		t.Log("Building Docker image...")
		if err := buildDockerImage(); err != nil {
			t.Fatalf("Failed to build Docker image: %v", err)
		}
	}

	if err := deleteKindCluster(); err != nil {
		t.Logf("Warning: failed to delete kind cluster: %v", err)
	}

	// Clean up cluster after all tests complete
	t.Cleanup(func() {
		t.Log("Cleaning up: deleting kind cluster...")
		if err := deleteKindCluster(); err != nil {
			t.Logf("Warning: failed to delete kind cluster: %v", err)
		}
	})

	// Create a copy of testCases to shuffle
	shuffledCases := make([]TestCase, len(testCases))
	copy(shuffledCases, testCases)

	// Shuffle using time-based seed
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	rng.Shuffle(len(shuffledCases), func(i, j int) {
		shuffledCases[i], shuffledCases[j] = shuffledCases[j], shuffledCases[i]
	})

	t.Logf("Running %d tests in randomized order", len(shuffledCases))

	// Track how many tests since last cluster creation
	testsSinceClusterCreation := 0

	// Run each test case
	for i, tc := range shuffledCases {
		// Determine if this test needs cluster roles disabled:
		// - DisableClusterRoles explicitly set, OR
		// - UseArgocdApi explicitly set to true, OR
		// - Global useArgocdApi flag is set AND test doesn't explicitly disable it
		testNeedsRolesDisabled := tc.DisableClusterRoles == "true" || tc.UseArgocdApi == "true" || (tc.UseArgocdApi != "false" && *useArgocdApi)

		// Check current cluster state
		clusterExists := kindClusterExists()

		// Check for RBAC mismatch (only relevant if cluster exists)
		if clusterExists {
			clusterHasRoles := clusterHasArgocdClusterRoles()
			// Mismatch if: test wants roles disabled but cluster has them, OR
			//              test wants roles enabled but cluster doesn't have them
			rbacMismatch := testNeedsRolesDisabled == clusterHasRoles

			if rbacMismatch {
				printToTTY("🔄 Deleting cluster due to RBAC configuration mismatch...\n")
				_ = deleteKindCluster()
				clusterExists = false
			}
		}

		// Create cluster if: every 8th test, no cluster exists, or test explicitly requires it
		createCluster := testsSinceClusterCreation >= 8 || !clusterExists || tc.CreateCluster == "true"
		if createCluster {
			testsSinceClusterCreation = 0
		}

		// Print separator to TTY for visibility between test runs
		runMode := "go"
		if *useDocker {
			runMode = "docker"
		}
		printToTTY(fmt.Sprintf("\n\n========== 🧪 TEST %d/%d: %s (createCluster = %v, mode = %s) ==========\n\n", i+1, len(shuffledCases), tc.Name, createCluster, runMode))
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc, createCluster)
		})

		testsSinceClusterCreation++

		// Stop on first failure (fail fast)
		if t.Failed() {
			printToTTY("\n❌ Stopping test run due to failure\n")
			break
		}
	}
}

// kindClusterExists checks if the kind cluster exists
func kindClusterExists() bool {
	cmd := exec.Command("kind", "get", "clusters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	// Check if our cluster name is in the list
	return slices.Contains(strings.Split(strings.TrimSpace(string(output)), "\n"), "argocd-diff-preview")
}

// clusterHasArgocdClusterRoles checks if the cluster has ArgoCD cluster roles installed.
// This is used to detect if the current cluster was installed with createClusterRoles enabled.
func clusterHasArgocdClusterRoles() bool {
	cmd := exec.Command("kubectl", "get", "clusterroles",
		"-l", "app.kubernetes.io/part-of=argocd",
		"--no-headers")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// printToTTY prints directly to TTY for real-time visibility (bypasses Go test output capture)
func printToTTY(msg string) {
	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		_, _ = tty.WriteString(msg)
		_ = tty.Close()
	}
}

// deleteKindCluster deletes the kind cluster used for testing
func deleteKindCluster() error {
	cmd := exec.Command("kind", "delete", "cluster", "--name", "argocd-diff-preview")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runTestCase executes a single test case
// createCluster indicates whether this test should create a new cluster (true for first test)
// Both Go binary and Docker run from repo root for consistency
func runTestCase(t *testing.T, tc TestCase, createCluster bool) {
	// Force cluster creation if the test case requires it (e.g., for testing role changes)
	if tc.CreateCluster == "true" {
		createCluster = true
	}

	// All directories are at repo root (parent of integration-test/)
	baseBranchDir := "../base-branch"
	targetBranchDir := "../target-branch"
	outputDir := "../output"

	// Clean up from previous runs
	cleanup(baseBranchDir, targetBranchDir, outputDir)

	// Clone the repositories to repo root
	if err := cloneBranch(tc.BaseBranch, baseBranchDir); err != nil {
		t.Fatalf("Failed to clone base branch: %v", err)
	}
	if err := cloneBranch(tc.TargetBranch, targetBranchDir); err != nil {
		t.Fatalf("Failed to clone target branch: %v", err)
	}

	// Run the tool
	var err error
	if *useDocker {
		err = runWithDocker(tc, createCluster)
	} else {
		err = runWithGoBinary(tc, createCluster)
	}

	// Handle expected failures
	if tc.ExpectFailure {
		if err != nil {
			t.Logf("Test failed as expected: %v", err)
			return // Success - we expected it to fail
		}
		t.Fatalf("Expected test to fail, but it succeeded")
	}

	if err != nil {
		t.Fatalf("Failed to run tool: %v", err)
	}

	// Check if output files were created (at repo root)
	mdPath := filepath.Join(outputDir, "diff.md")
	htmlPath := filepath.Join(outputDir, "diff.html")

	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Fatalf("Tool completed but did not create output file: %s (this may indicate the tool exited early without generating output)", mdPath)
	}
	if _, err := os.Stat(htmlPath); os.IsNotExist(err) {
		t.Fatalf("Tool completed but did not create output file: %s (this may indicate the tool exited early without generating output)", htmlPath)
	}

	// Compare outputs
	expectedDir := getExpectedDir(tc)
	compareOutput(t, tc, expectedDir, outputDir)
}

// cleanup removes directories from previous test runs
func cleanup(dirs ...string) {
	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
	}
}

// cloneBranch clones a specific branch from the repository
func cloneBranch(branch, targetDir string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", defaultGitHubOrg, defaultGitOpsRepo)

	// Clone with depth=1, with retries for transient network errors
	maxAttempts := 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cmd := exec.Command("git", "clone", repoURL, "--depth=1", "--branch", branch, "repo")
		cmd.Dir = targetDir
		output, err := cmd.CombinedOutput()
		if err == nil {
			break // Success
		}
		lastErr = fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
		if attempt < maxAttempts {
			// Clean up failed clone attempt before retrying
			_ = os.RemoveAll(filepath.Join(targetDir, "repo"))
			time.Sleep(time.Duration(attempt) * 2 * time.Second) // Exponential backoff: 2s, 4s
			continue
		}
		return lastErr
	}

	// Copy contents up and clean up
	repoDir := filepath.Join(targetDir, "repo")

	// Copy all files from repo to targetDir
	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return fmt.Errorf("failed to read repo dir: %w", err)
	}

	for _, entry := range entries {
		src := filepath.Join(repoDir, entry.Name())
		dst := filepath.Join(targetDir, entry.Name())

		if entry.Name() == ".git" {
			continue // Skip .git directory
		}

		if entry.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", entry.Name(), err)
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", entry.Name(), err)
			}
		}
	}

	// Remove the cloned repo directory
	_ = os.RemoveAll(repoDir)

	// Create .gitignore with "*"
	if err := os.WriteFile(filepath.Join(targetDir, ".gitignore"), []byte("*\n"), 0644); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// buildDockerImage builds the Docker image
func buildDockerImage() error {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		return fmt.Errorf("failed to get repo root: %w", err)
	}
	cmd := exec.Command("docker", "build", "-f", "Dockerfile", "-t", *dockerImage, ".")
	cmd.Dir = repoRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runWithGoBinary executes the test using the Go binary
// Runs from repo root directory so it can find argocd-config/
func runWithGoBinary(tc TestCase, createCluster bool) error {
	args := buildArgs(tc, createCluster)

	// Get repo root (parent of integration-test/)
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		return fmt.Errorf("failed to get repo root: %w", err)
	}

	// If DisableClusterRoles is set OR we're using the API mode, temporarily replace
	// argocd-config/values.yaml with the createClusterRoles.yaml file (which sets
	// createClusterRoles: false). This tests the API mode in a more restrictive environment.
	if tc.DisableClusterRoles == "true" || tc.UseArgocdApi == "true" || *useArgocdApi {
		valuesPath := filepath.Join(repoRoot, "argocd-config", "values.yaml")
		createClusterRolesPath := filepath.Join(repoRoot, "integration-test", "createClusterRoles.yaml")

		// Backup original values.yaml
		originalContent, err := os.ReadFile(valuesPath)
		if err != nil {
			return fmt.Errorf("failed to read original values.yaml: %w", err)
		}

		// Copy createClusterRoles.yaml to values.yaml
		newContent, err := os.ReadFile(createClusterRolesPath)
		if err != nil {
			return fmt.Errorf("failed to read createClusterRoles.yaml: %w", err)
		}
		if err := os.WriteFile(valuesPath, newContent, 0644); err != nil {
			return fmt.Errorf("failed to write values.yaml: %w", err)
		}

		// Restore original values.yaml after the test
		defer func() {
			_ = os.WriteFile(valuesPath, originalContent, 0644)
		}()
	}

	// Binary path is relative to repo root
	absBinaryPath := filepath.Join(repoRoot, *binaryPath)

	cmd := exec.Command(absBinaryPath, args...)
	cmd.Dir = repoRoot // Run from repo root so it finds argocd-config/

	// Try to get TTY for real-time output (Go test captures stdout/stderr)
	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		cmd.Stdout = tty
		cmd.Stderr = tty
		defer func() { _ = tty.Close() }()
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// getDockerAPIVersion detects if Docker client is too new for server
// and returns the maximum supported API version, or empty string if not needed
func getDockerAPIVersion() string {
	// First check if DOCKER_API_VERSION is already set in the environment
	if version := os.Getenv("DOCKER_API_VERSION"); version != "" {
		return version
	}

	cmd := exec.Command("docker", "version")
	// Use CombinedOutput to capture both stdout and stderr
	// Note: This command may fail with non-zero exit code when API versions mismatch,
	// but the error message still contains the maximum supported version
	output, _ := cmd.CombinedOutput()

	// Look for "Maximum supported API version is X.XX" in output
	re := regexp.MustCompile(`Maximum supported API version is ([0-9.]+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// runWithDocker executes the test using Docker
// Mounts volumes from repo root directory
func runWithDocker(tc TestCase, createCluster bool) error {
	// Remove any existing container (ignore error - container may not exist)
	_ = exec.Command("docker", "rm", "-f", "argocd-diff-preview").Run()

	// Get repo root (parent of integration-test/) for volume mounts
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		return fmt.Errorf("failed to get repo root: %w", err)
	}

	// Get home directory for .kube mount
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Detect Docker API version mismatch
	dockerAPIVersion := getDockerAPIVersion()

	args := []string{
		"run",
		"--network=host",
		"--name=argocd-diff-preview",
		"-v", fmt.Sprintf("%s/.kube:/root/.kube", homeDir),
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-v", fmt.Sprintf("%s/base-branch:/base-branch", repoRoot),
		"-v", fmt.Sprintf("%s/target-branch:/target-branch", repoRoot),
		"-v", fmt.Sprintf("%s/output:/output", repoRoot),
		"-v", fmt.Sprintf("%s/secrets:/secrets", repoRoot),
		"-v", fmt.Sprintf("%s/temp:/temp", repoRoot),
		"-v", fmt.Sprintf("%s/kind-config:/kind-config", repoRoot),
	}

	// Mount argocd-config: when use_argocd_api is true or DisableClusterRoles is set,
	// mount the special createClusterRoles.yaml file (which sets createClusterRoles: false).
	// This tests the API mode in a more restrictive environment without cluster-wide permissions.
	if tc.UseArgocdApi == "true" || *useArgocdApi || tc.DisableClusterRoles == "true" {
		args = append(args, "-v", fmt.Sprintf("%s/integration-test/createClusterRoles.yaml:/argocd-config/values.yaml", repoRoot))
	}

	// Pass Docker API version if needed (when client is newer than server)
	if dockerAPIVersion != "" {
		args = append(args, "-e", fmt.Sprintf("DOCKER_API_VERSION=%s", dockerAPIVersion))
	}

	// Add environment variables
	args = append(args, "-e", fmt.Sprintf("BASE_BRANCH=%s", tc.BaseBranch))
	args = append(args, "-e", fmt.Sprintf("TARGET_BRANCH=%s", tc.TargetBranch))
	args = append(args, "-e", fmt.Sprintf("REPO=%s/%s", defaultGitHubOrg, defaultGitOpsRepo))
	args = append(args, "-e", fmt.Sprintf("TIMEOUT=%s", defaultTimeout))
	args = append(args, "-e", fmt.Sprintf("LINE_COUNT=%s", getLineCount(tc)))
	args = append(args, "-e", fmt.Sprintf("MAX_DIFF_LENGTH=%s", getMaxDiffLength(tc)))
	args = append(args, "-e", fmt.Sprintf("TITLE=%s", getTitle(tc)))
	args = append(args, "-e", fmt.Sprintf("ARGOCD_NAMESPACE=%s", argocdNamespace))
	args = append(args, "-e", "DISABLE_CLIENT_THROTTLING=true")

	if tc.FileRegex != "" {
		args = append(args, "-e", fmt.Sprintf("FILE_REGEX=%s", tc.FileRegex))
	}
	if tc.DiffIgnore != "" {
		args = append(args, "-e", fmt.Sprintf("DIFF_IGNORE=%s", tc.DiffIgnore))
	}
	if tc.FilesChanged != "" {
		args = append(args, "-e", fmt.Sprintf("FILES_CHANGED=%s", tc.FilesChanged))
	}
	if tc.Selector != "" {
		args = append(args, "-e", fmt.Sprintf("SELECTOR=%s", tc.Selector))
	}
	if tc.WatchIfNoWatchPatternFound != "" {
		args = append(args, "-e", fmt.Sprintf("WATCH_IF_NO_WATCH_PATTERN_FOUND=%s", tc.WatchIfNoWatchPatternFound))
	}
	if tc.AutoDetectFilesChanged != "" {
		args = append(args, "-e", fmt.Sprintf("AUTO_DETECT_FILES_CHANGED=%s", tc.AutoDetectFilesChanged))
	}
	if tc.IgnoreInvalidWatchPattern != "" {
		args = append(args, "-e", fmt.Sprintf("IGNORE_INVALID_WATCH_PATTERN=%s", tc.IgnoreInvalidWatchPattern))
	}
	if tc.HideDeletedAppDiff != "" {
		args = append(args, "-e", fmt.Sprintf("HIDE_DELETED_APP_DIFF=%s", tc.HideDeletedAppDiff))
	}
	if tc.IgnoreResources != "" {
		args = append(args, "-e", fmt.Sprintf("IGNORE_RESOURCES=%s", tc.IgnoreResources))
	}
	if tc.KindOptions != "" {
		args = append(args, "-e", fmt.Sprintf("KIND_OPTIONS=%s", tc.KindOptions))
	}
	// UseArgocdApi: "true" = force enable, "false" = force disable, "" = use global flag
	if tc.UseArgocdApi == "true" || (tc.UseArgocdApi != "false" && *useArgocdApi) {
		args = append(args, "-e", "USE_ARGOCD_API=true")
	}

	// Set CREATE_CLUSTER based on the createCluster parameter (overrides tc.CreateCluster)
	args = append(args, "-e", fmt.Sprintf("CREATE_CLUSTER=%t", createCluster))

	// Keep cluster alive unless test expects failure (cluster may be in broken state)
	if !tc.ExpectFailure {
		args = append(args, "-e", "KEEP_CLUSTER_ALIVE=true")
	}

	if tc.ArgocdLoginOptions != "" {
		args = append(args, "-e", fmt.Sprintf("ARGOCD_LOGIN_OPTIONS=%s", tc.ArgocdLoginOptions))
	}

	if tc.ArgocdAuthToken != "" {
		args = append(args, "-e", fmt.Sprintf("ARGOCD_AUTH_TOKEN=%s", tc.ArgocdAuthToken))
	}

	if tc.ArgocdUIURL != "" {
		args = append(args, "-e", fmt.Sprintf("ARGOCD_UI_URL=%s", tc.ArgocdUIURL))
	}

	// Add image (no additional args needed - all config is via env vars)
	args = append(args, *dockerImage)

	cmd := exec.Command("docker", args...)

	// Try to get TTY for real-time output (Go test captures stdout/stderr)
	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		cmd.Stdout = tty
		cmd.Stderr = tty
		defer func() { _ = tty.Close() }()
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}

// buildArgs constructs command line arguments for the Go binary
func buildArgs(tc TestCase, createCluster bool) []string {
	args := []string{
		"--base-branch", tc.BaseBranch,
		"--target-branch", tc.TargetBranch,
		"--repo", fmt.Sprintf("%s/%s", defaultGitHubOrg, defaultGitOpsRepo),
		"--argocd-namespace", argocdNamespace,
		"--timeout", defaultTimeout,
		"--line-count", getLineCount(tc),
		"--max-diff-length", getMaxDiffLength(tc),
		"--title", getTitle(tc),
		"--keep-cluster-alive",
		"--disable-client-throttling",
	}

	// Don't keep cluster alive for tests that expect failure (cluster may be in broken state)
	if !tc.ExpectFailure {
		args = append(args, "--keep-cluster-alive")
	}

	if *debug {
		args = append(args, "--debug")
	}

	if tc.FileRegex != "" {
		args = append(args, "--file-regex", tc.FileRegex)
	}
	if tc.DiffIgnore != "" {
		args = append(args, "--diff-ignore", tc.DiffIgnore)
	}
	if tc.FilesChanged != "" {
		args = append(args, "--files-changed", tc.FilesChanged)
	}
	if tc.Selector != "" {
		args = append(args, "--selector", tc.Selector)
	}
	if tc.WatchIfNoWatchPatternFound == "true" {
		args = append(args, "--watch-if-no-watch-pattern-found")
	}
	if tc.AutoDetectFilesChanged == "true" {
		args = append(args, "--auto-detect-files-changed")
	}
	if tc.IgnoreInvalidWatchPattern == "true" {
		args = append(args, "--ignore-invalid-watch-pattern")
	}
	if tc.HideDeletedAppDiff == "true" {
		args = append(args, "--hide-deleted-app-diff")
	}
	if tc.IgnoreResources != "" {
		args = append(args, "--ignore-resources", tc.IgnoreResources)
	}
	if tc.KindOptions != "" {
		args = append(args, "--kind-options", tc.KindOptions)
	}
	// UseArgocdApi: "true" = force enable, "false" = force disable, "" = use global flag
	if tc.UseArgocdApi == "true" || (tc.UseArgocdApi != "false" && *useArgocdApi) {
		args = append(args, "--use-argocd-api")
	}

	// Set --create-cluster based on the createCluster parameter (overrides tc.CreateCluster)
	args = append(args, fmt.Sprintf("--create-cluster=%t", createCluster))

	if tc.ArgocdLoginOptions != "" {
		args = append(args, "--argocd-login-options", tc.ArgocdLoginOptions)
	}

	if tc.ArgocdAuthToken != "" {
		args = append(args, "--argocd-auth-token", tc.ArgocdAuthToken)
	}

	if tc.ArgocdUIURL != "" {
		args = append(args, "--argocd-ui-url", tc.ArgocdUIURL)
	}

	return args
}

// getLineCount returns the line count for a test case
func getLineCount(tc TestCase) string {
	if tc.LineCount != "" {
		return tc.LineCount
	}
	return defaultLineCount
}

// getTitle returns the title for a test case
func getTitle(tc TestCase) string {
	if tc.Title != "" {
		return tc.Title
	}
	return defaultTitle
}

// getMaxDiffLength returns the max diff length for a test case
func getMaxDiffLength(tc TestCase) string {
	if tc.MaxDiffLength != "" {
		return tc.MaxDiffLength
	}
	return defaultMaxDiffLen
}

// getExpectedDir returns the directory containing expected output files
func getExpectedDir(tc TestCase) string {
	// Extract branch name from target branch (e.g., "integration-test/branch-1/target" -> "branch-1")
	parts := strings.Split(tc.TargetBranch, "/")
	branchName := parts[1] // "branch-1", "branch-2", etc.

	// Build the expected directory path (relative to integration-test/ folder)
	suffix := tc.Suffix
	if suffix == "" {
		suffix = ""
	}
	return filepath.Join(branchName, "target"+suffix)
}

// compareOutput compares actual output with expected output
func compareOutput(t *testing.T, tc TestCase, expectedDir, outputDir string) {
	// Read and normalize actual outputs
	actualMD, err := os.ReadFile(filepath.Join(outputDir, "diff.md"))
	if err != nil {
		t.Fatalf("Failed to read actual diff.md: %v", err)
	}
	actualHTML, err := os.ReadFile(filepath.Join(outputDir, "diff.html"))
	if err != nil {
		t.Fatalf("Failed to read actual diff.html: %v", err)
	}

	// Normalize timing information
	normalizedMD := normalizeOutput(string(actualMD))
	normalizedHTML := normalizeOutput(string(actualHTML))

	expectedMDPath := filepath.Join(expectedDir, "output.md")
	expectedHTMLPath := filepath.Join(expectedDir, "output.html")

	if *update {
		// Update mode: write actual output to expected files
		if err := os.MkdirAll(expectedDir, 0755); err != nil {
			t.Fatalf("Failed to create expected directory: %v", err)
		}
		if err := os.WriteFile(expectedMDPath, []byte(normalizedMD), 0644); err != nil {
			t.Fatalf("Failed to write expected MD: %v", err)
		}
		if err := os.WriteFile(expectedHTMLPath, []byte(normalizedHTML), 0644); err != nil {
			t.Fatalf("Failed to write expected HTML: %v", err)
		}
		t.Logf("Updated expected files for %s", tc.Name)
		return
	}

	// Compare mode: check actual against expected
	expectedMD, err := os.ReadFile(expectedMDPath)
	if err != nil {
		t.Fatalf("Failed to read expected diff.md: %v", err)
	}
	expectedHTML, err := os.ReadFile(expectedHTMLPath)
	if err != nil {
		t.Fatalf("Failed to read expected diff.html: %v", err)
	}

	// Compare MD
	if normalizedMD != string(expectedMD) {
		t.Errorf("Markdown output mismatch for %s\n", tc.Name)
		showDiff(t, "diff.md", string(expectedMD), normalizedMD)
	}

	// Compare HTML
	if normalizedHTML != string(expectedHTML) {
		t.Errorf("HTML output mismatch for %s\n", tc.Name)
		showDiff(t, "diff.html", string(expectedHTML), normalizedHTML)
	}

	if normalizedMD == string(expectedMD) && normalizedHTML == string(expectedHTML) {
		t.Logf("Test passed: %s", tc.Name)
	}
}

// showDiff displays a simple diff between expected and actual content
func showDiff(t *testing.T, filename, expected, actual string) {
	// Write to temp files and use diff command for better output
	tmpExpected, err := os.CreateTemp("", "expected-*")
	if err != nil {
		t.Logf("Expected:\n%s", expected)
		t.Logf("Actual:\n%s", actual)
		return
	}
	defer func() { _ = os.Remove(tmpExpected.Name()) }()

	tmpActual, err := os.CreateTemp("", "actual-*")
	if err != nil {
		t.Logf("Expected:\n%s", expected)
		t.Logf("Actual:\n%s", actual)
		return
	}
	defer func() { _ = os.Remove(tmpActual.Name()) }()

	_, _ = tmpExpected.WriteString(expected)
	_, _ = tmpActual.WriteString(actual)
	_ = tmpExpected.Close()
	_ = tmpActual.Close()

	cmd := exec.Command("diff", "-u", tmpExpected.Name(), tmpActual.Name())
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // Ignore error - diff returns non-zero when files differ

	t.Logf("Diff for %s:\n%s", filename, out.String())
}

// TestSingleCase allows running a single test case by name
// Usage: TEST_CASE="branch-1/target-1" go test -run TestSingleCase ./...
// Reuses existing cluster if one exists, unless -create-cluster flag is set
func TestSingleCase(t *testing.T) {
	caseName := os.Getenv("TEST_CASE")
	if caseName == "" {
		t.Skip("TEST_CASE environment variable not set")
	}

	// Delete existing cluster if -create-cluster flag is set
	if *createCluster {
		t.Log("Flag -create-cluster set, deleting existing cluster if any...")
		_ = deleteKindCluster()
	}

	// Check if cluster already exists
	clusterExists := kindClusterExists()
	if clusterExists {
		t.Log("Using existing kind cluster 'argocd-diff-preview'")
	} else {
		t.Log("No existing cluster found, will create a new one")
	}

	// Only clean up cluster if we created it (i.e., it didn't exist before)
	t.Cleanup(func() {
		if !clusterExists {
			t.Log("Cleaning up: deleting kind cluster...")
			if err := deleteKindCluster(); err != nil {
				t.Logf("Warning: failed to delete kind cluster: %v", err)
			}
		} else {
			t.Log("Keeping existing cluster alive")
		}
	})

	for _, tc := range testCases {
		if tc.Name == caseName {
			// Create cluster only if one doesn't already exist
			runTestCase(t, tc, !clusterExists)
			return
		}
	}

	t.Fatalf("Test case not found: %s", caseName)
}
