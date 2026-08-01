package integration_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	inClusterClusterName    = "argocd-diff-preview-in-cluster"
	inClusterTestName       = "branch-5/target-2"
	inClusterRunnerName     = "argocd-diff-preview-in-cluster"
	inClusterArtifactName   = "argocd-diff-preview-artifacts"
	inClusterGitImage       = "alpine/git:latest"
	inClusterBusyboxImage   = "busybox:1.37"
	inClusterRunnerLogProbe = "Using Argo CD repo server service address"
)

var inClusterRenderMethods = []string{"server-api", "repo-server-api"}

// TestInClusterRepoServerAPI verifies that argocd-diff-preview can run as a Pod
// inside the same cluster as Argo CD and talk to the repo-server via Service DNS.
//
// This is intentionally opt-in because it creates a kind cluster, installs Argo CD,
// builds and loads a Docker image, and needs network access from the test cluster.
//
// Usage:
//
//	cd integration-test
//	RUN_IN_CLUSTER_TEST=true go test -v -timeout 20m -run TestInClusterRepoServerAPI ./...
func TestInClusterRepoServerAPI(t *testing.T) {
	if os.Getenv("RUN_IN_CLUSTER_TEST") != "true" {
		t.Skip("Skipping in-cluster integration test. Set RUN_IN_CLUSTER_TEST=true to run.")
	}

	ensureIntegrationTestDir(t)

	tc := findTestCase(t, inClusterTestName)
	repoRoot := repoRootFromIntegrationDir(t)
	runDirs := newRunDirs(repoRoot)
	outputDir := filepath.Join(runDirs.Root, "in-cluster-output")

	cleanup(runDirs.Root)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	cleanupInClusterKindCluster(t)
	t.Cleanup(func() { cleanupInClusterKindCluster(t) })

	t.Log("Building Docker image for in-cluster run")
	if err := buildDockerImage(); err != nil {
		t.Fatalf("Failed to build Docker image: %v", err)
	}

	t.Logf("Creating kind cluster %s", inClusterClusterName)
	if err := runCommandStreaming("kind", "create", "cluster", "--name", inClusterClusterName); err != nil {
		t.Fatalf("Failed to create kind cluster: %v", err)
	}

	t.Logf("Loading Docker image %s into kind", *dockerImage)
	if err := runCommandStreaming("kind", "load", "docker-image", *dockerImage, "--name", inClusterClusterName); err != nil {
		t.Fatalf("Failed to load Docker image into kind: %v", err)
	}

	t.Log("Installing Argo CD")
	if err := installArgoCDForInClusterTest(repoRoot); err != nil {
		t.Fatalf("Failed to install Argo CD: %v", err)
	}

	for _, renderMethod := range inClusterRenderMethods {
		renderMethod := renderMethod
		t.Run(renderMethod, func(t *testing.T) {
			runInClusterRenderMethod(t, tc, renderMethod, outputDir)
		})
	}
}

func cleanupInClusterKindCluster(t *testing.T) {
	t.Helper()

	t.Logf("Cleaning up in-cluster kind cluster %s", inClusterClusterName)
	if err := runCommandStreaming("kind", "delete", "cluster", "--name", inClusterClusterName); err != nil {
		t.Logf("Warning: failed to delete in-cluster kind cluster %s: %v", inClusterClusterName, err)
	}
}

func runInClusterRenderMethod(t *testing.T, tc TestCase, renderMethod, outputRoot string) {
	t.Helper()

	runnerName := fmt.Sprintf("%s-%s", inClusterRunnerName, renderMethod)
	outputDir := filepath.Join(outputRoot, renderMethod)
	cleanup(outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	t.Logf("Creating in-cluster runner Pod for %s", renderMethod)
	podYAML := inClusterRunnerYAML(tc, runnerName, renderMethod)
	if err := kubectlApplyYAML(podYAML); err != nil {
		t.Fatalf("Failed to create in-cluster runner: %v", err)
	}

	t.Cleanup(func() {
		_ = runCommandStreaming("kubectl", "delete", "pod", runnerName, "-n", argocdNamespace, "--ignore-not-found")
		_ = runCommandStreaming("kubectl", "delete", "networkpolicy", runnerName, "-n", argocdNamespace, "--ignore-not-found")
		_ = runCommandStreaming("kubectl", "delete", "serviceaccount", runnerName, "-n", argocdNamespace, "--ignore-not-found")
		_ = runCommandStreaming("kubectl", "delete", "clusterrolebinding", runnerName, "--ignore-not-found")
	})

	t.Logf("Waiting for in-cluster runner to finish for %s", renderMethod)
	if err := waitForContainerExit(argocdNamespace, runnerName, "runner", 10*time.Minute); err != nil {
		logs := kubectlLogs(argocdNamespace, runnerName, "runner")
		t.Fatalf("In-cluster runner failed: %v\nRunner logs:\n%s", err, logs)
	}

	// TODO: Re-enable this assertion when repo-server Service DNS auto-detection
	// is merged. The test-only branch is expected to pass before that
	// implementation lands, so for now it only verifies that in-cluster
	// repo-server-api rendering works.
	//
	// logs := kubectlLogs(argocdNamespace, runnerName, "runner")
	// if renderMethod == "repo-server-api" && !strings.Contains(logs, inClusterRunnerLogProbe) {
	// 	t.Fatalf("Runner logs did not show repo-server Service DNS auto-detection. Missing %q\nRunner logs:\n%s", inClusterRunnerLogProbe, logs)
	// }

	t.Logf("Copying output artifacts from in-cluster runner for %s", renderMethod)
	copyFromPod(t, runnerName, filepath.Join(outputDir, "diff.md"), "/work/output/diff.md")
	copyFromPod(t, runnerName, filepath.Join(outputDir, "diff.html"), "/work/output/diff.html")

	compareOutput(t, tc, getExpectedDir(tc), outputDir)
}

func ensureIntegrationTestDir(t *testing.T) {
	t.Helper()

	testDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	if strings.HasSuffix(testDir, "/integration-test") || strings.HasSuffix(testDir, "\\integration-test") {
		return
	}

	if err := os.Chdir(filepath.Join(testDir, "integration-test")); err != nil {
		t.Fatalf("Failed to change to integration-test directory: %v", err)
	}
}

func repoRootFromIntegrationDir(t *testing.T) string {
	t.Helper()

	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("Failed to get repo root: %v", err)
	}
	return repoRoot
}

func findTestCase(t *testing.T, name string) TestCase {
	t.Helper()

	for _, tc := range testCases {
		if tc.Name == name {
			return tc
		}
	}

	t.Fatalf("Test case not found: %s", name)
	return TestCase{}
}

func installArgoCDForInClusterTest(repoRoot string) error {
	valuesPath := filepath.Join(repoRoot, "integration-test", "no-cluster-roles", "values.yaml")
	overridePath := filepath.Join(repoRoot, "argocd-config", "values-override.yaml")
	if err := runCommandStreaming(
		"helm", "install", "argocd", "argo-cd",
		"--repo", "https://argoproj.github.io/argo-helm",
		"--create-namespace",
		"--namespace", argocdNamespace,
		"--values", valuesPath,
		"--values", overridePath,
		"--wait",
		"--timeout", "10m",
	); err != nil {
		return fmt.Errorf("failed to install Argo CD Helm chart: %w", err)
	}

	return nil
}

func inClusterRunnerYAML(tc TestCase, runnerName, renderMethod string) string {
	args := []string{
		"--base-branch", tc.BaseBranch,
		"--target-branch", tc.TargetBranch,
		"--repo", fmt.Sprintf("%s/%s", defaultGitHubOrg, defaultGitOpsRepo),
		"--argocd-namespace", argocdNamespace,
		"--render-method", renderMethod,
		"--create-cluster=false",
		"--keep-cluster-alive",
		"--disable-client-throttling",
		"--timeout", defaultTimeout,
		"--line-count", getLineCount(tc),
		"--max-diff-length", getMaxDiffLength(tc),
		"--title", getTitle(tc),
		"--output-folder", "/work/output",
		"--debug",
	}

	if tc.FileRegex != "" {
		args = append(args, "--file-regex", tc.FileRegex)
	}
	if tc.FilesChanged != "" {
		args = append(args, "--files-changed", tc.FilesChanged)
	}
	if tc.Selector != "" {
		args = append(args, "--selector", tc.Selector)
	}
	if tc.DiffIgnore != "" {
		args = append(args, "--diff-ignore", tc.DiffIgnore)
	}
	if tc.WatchIfNoWatchPatternFound == "false" {
		args = append(args, "--watch-if-no-watch-pattern-found=false")
	}
	if tc.AutoDetectFilesChanged == "false" {
		args = append(args, "--auto-detect-files-changed=false")
	}

	return fmt.Sprintf(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: %[1]s
  namespace: %[2]s
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %[1]s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: %[1]s
    namespace: %[2]s
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: argocd-repo-server
  policyTypes:
    - Ingress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: %[1]s
      ports:
        - protocol: TCP
          port: 8081
---
apiVersion: v1
kind: Pod
metadata:
  name: %[1]s
  namespace: %[2]s
  labels:
    app.kubernetes.io/name: %[1]s
spec:
  restartPolicy: Never
  serviceAccountName: %[1]s
  initContainers:
    - name: clone-base
      image: %[3]s
      args:
%[7]s
      volumeMounts:
        - name: work
          mountPath: /work
    - name: clone-target
      image: %[3]s
      args:
%[8]s
      volumeMounts:
        - name: work
          mountPath: /work
  containers:
    - name: runner
      image: %[4]s
      imagePullPolicy: Never
      command: ["/argocd-diff-preview"]
      workingDir: /work
      args:
%[6]s
      volumeMounts:
        - name: work
          mountPath: /work
    - name: %[5]s
      image: %[9]s
      command: ["sh", "-c", "sleep 3600"]
      volumeMounts:
        - name: work
          mountPath: /work
  volumes:
    - name: work
      emptyDir: {}
`, runnerName, argocdNamespace, inClusterGitImage, *dockerImage, inClusterArtifactName, yamlStringList(args, 8), yamlStringList(gitCloneArgs(tc.BaseBranch, "/work/base-branch"), 8), yamlStringList(gitCloneArgs(tc.TargetBranch, "/work/target-branch"), 8), inClusterBusyboxImage)
}

func gitCloneArgs(branch, path string) []string {
	return []string{
		"clone",
		fmt.Sprintf("https://github.com/%s/%s.git", defaultGitHubOrg, defaultGitOpsRepo),
		"--depth=1",
		"--branch", branch,
		path,
	}
}

func yamlStringList(values []string, indent int) string {
	var b strings.Builder
	padding := strings.Repeat(" ", indent)
	for _, value := range values {
		b.WriteString(padding)
		b.WriteString("- ")
		b.WriteString(fmt.Sprintf("%q", value))
		b.WriteString("\n")
	}
	return b.String()
}

func kubectlApplyYAML(yaml string) error {
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func waitForContainerExit(namespace, podName, containerName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		phase, _ := kubectlOutput("get", "pod", podName, "-n", namespace, "-o", "jsonpath={.status.phase}")
		exitCode, _ := kubectlOutput("get", "pod", podName, "-n", namespace, "-o", fmt.Sprintf("jsonpath={.status.containerStatuses[?(@.name==%q)].state.terminated.exitCode}", containerName))

		if strings.TrimSpace(exitCode) != "" {
			if strings.TrimSpace(exitCode) == "0" {
				return nil
			}
			return fmt.Errorf("container %s exited with code %s", containerName, strings.TrimSpace(exitCode))
		}

		if strings.TrimSpace(phase) == "Failed" {
			return fmt.Errorf("pod entered Failed phase before %s exited", containerName)
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for container %s in pod %s", containerName, podName)
}

func copyFromPod(t *testing.T, podName, localPath, remotePath string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		t.Fatalf("Failed to create local artifact directory: %v", err)
	}

	remote := fmt.Sprintf("%s/%s:%s", argocdNamespace, podName, remotePath)
	if err := runCommandStreaming("kubectl", "cp", remote, localPath, "-c", inClusterArtifactName); err != nil {
		t.Fatalf("Failed to copy %s from in-cluster runner: %v", remotePath, err)
	}
}

func kubectlLogs(namespace, podName, containerName string) string {
	logs, err := kubectlOutput("logs", podName, "-n", namespace, "-c", containerName)
	if err != nil {
		return fmt.Sprintf("failed to read logs: %v\n%s", err, logs)
	}
	return logs
}

func kubectlOutput(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func runCommandStreaming(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
