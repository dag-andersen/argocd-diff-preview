package tests

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Flags for test configuration
var (
	update      = flag.Bool("update", false, "update golden files with actual output")
	useDocker   = flag.Bool("docker", false, "use Docker instead of Go binary")
	debug       = flag.Bool("debug", false, "enable debug mode for the tool")
	binaryPath  = flag.String("binary", "../bin/argocd-diff-preview", "path to the Go binary")
	dockerImage = flag.String("image", "argocd-diff-preview", "Docker image name to use")
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
		Name:         "branch-11/target-1",
		TargetBranch: "integration-test/branch-11/target",
		BaseBranch:   "integration-test/branch-11/base",
		Suffix:       "-1",
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
	// Ensure we're in the tests directory
	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Check if we're in the tests directory, if not, change to it
	if !strings.HasSuffix(testDir, "/tests") && !strings.HasSuffix(testDir, "\\tests") {
		testDir = filepath.Join(testDir, "tests")
		if err := os.Chdir(testDir); err != nil {
			t.Fatalf("Failed to change to tests directory: %v", err)
		}
	}

	// Build docker image if using docker mode
	if *useDocker {
		t.Log("Building Docker image...")
		if err := buildDockerImage(); err != nil {
			t.Fatalf("Failed to build Docker image: %v", err)
		}
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

	// Run each test case
	// Every 8th test creates a new cluster (tests 0, 8, 16, etc.)
	for i, tc := range shuffledCases {
		createCluster := i%8 == 0
		t.Run(tc.Name, func(t *testing.T) {
			runTestCase(t, tc, createCluster)
		})
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
	// All directories are at repo root (parent of tests/)
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

	// Clone with depth=1
	cmd := exec.Command("git", "clone", repoURL, "--depth=1", "--branch", branch, "repo")
	cmd.Dir = targetDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, output)
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
	cmd := exec.Command("docker", "build", "-f", "../Dockerfile", "-t", *dockerImage, "..")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runWithGoBinary executes the test using the Go binary
// Runs from repo root directory so it can find argocd-config/
func runWithGoBinary(tc TestCase, createCluster bool) error {
	args := buildArgs(tc, createCluster)

	// Get repo root (parent of tests/)
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		return fmt.Errorf("failed to get repo root: %w", err)
	}

	// Convert binary path to absolute since we're changing working directory
	absBinaryPath, err := filepath.Abs(*binaryPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute binary path: %w", err)
	}

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
	cmd := exec.Command("docker", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

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

	// Get repo root (parent of tests/) for volume mounts
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
		"-v", fmt.Sprintf("%s/argocd-config:/argocd-config", repoRoot),
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

	// Set CREATE_CLUSTER based on the createCluster parameter (overrides tc.CreateCluster)
	args = append(args, "-e", fmt.Sprintf("CREATE_CLUSTER=%t", createCluster))

	// Always keep cluster alive (tests reuse the cluster)
	args = append(args, "-e", "KEEP_CLUSTER_ALIVE=true")

	// Add image and namespace argument
	args = append(args, *dockerImage)
	args = append(args, fmt.Sprintf("--argocd-namespace=%s", argocdNamespace))

	if tc.ArgocdLoginOptions != "" {
		args = append(args, fmt.Sprintf("--argocd-login-options=%s", tc.ArgocdLoginOptions))
	}

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

	// Set --create-cluster based on the createCluster parameter (overrides tc.CreateCluster)
	args = append(args, fmt.Sprintf("--create-cluster=%t", createCluster))

	if tc.ArgocdLoginOptions != "" {
		args = append(args, "--argocd-login-options", tc.ArgocdLoginOptions)
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

	// Build the expected directory path
	suffix := tc.Suffix
	if suffix == "" {
		suffix = ""
	}
	return filepath.Join("integration-test", branchName, "target"+suffix)
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
// Creates a new cluster by default, deletes it after the test
func TestSingleCase(t *testing.T) {
	caseName := os.Getenv("TEST_CASE")
	if caseName == "" {
		t.Skip("TEST_CASE environment variable not set")
	}

	// Clean up cluster after test completes
	t.Cleanup(func() {
		t.Log("Cleaning up: deleting kind cluster...")
		if err := deleteKindCluster(); err != nil {
			t.Logf("Warning: failed to delete kind cluster: %v", err)
		}
	})

	for _, tc := range testCases {
		if tc.Name == caseName {
			// When running a single test, create a new cluster
			runTestCase(t, tc, true)
			return
		}
	}

	t.Fatalf("Test case not found: %s", caseName)
}
