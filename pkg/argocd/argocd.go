package argocd

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type ArgoCDInstallation struct {
	Namespace  string
	Version    string
	ConfigPath string
}

func New(namespace string, version string, configPath string) *ArgoCDInstallation {
	if configPath == "" {
		configPath = "argocd-config"
	}
	return &ArgoCDInstallation{
		Namespace:  namespace,
		Version:    version,
		ConfigPath: configPath,
	}
}

func (a *ArgoCDInstallation) createNamespace() error {
	log.Printf("🦑 Creating namespace: %s", a.Namespace)

	// Check if namespace exists
	cmd := fmt.Sprintf("kubectl get namespace %s", a.Namespace)
	if err := runCommand("sh", "-c", cmd); err == nil {
		log.Printf("✨ Namespace %s already exists", a.Namespace)
		return nil
	}

	// Create namespace
	if err := runCommand("kubectl", "create", "namespace", a.Namespace); err != nil {
		log.Printf("❌ Failed to create namespace %s", a.Namespace)
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	log.Printf("✨ Created namespace: %s", a.Namespace)
	return nil
}

func (a *ArgoCDInstallation) Install(debug bool) error {
	log.Printf("🦑 Installing Argo CD Helm Chart version: '%s'", a.Version)

	// Create namespace if it doesn't exist
	if err := a.createNamespace(); err != nil {
		return err
	}

	// Check for values files
	values, valuesOverride, err := a.findValuesFiles()
	if err != nil {
		log.Printf("📂 Folder '%s' doesn't exist. Installing Argo CD Helm Chart with default configuration", a.ConfigPath)
	}

	// Add argo repo to helm
	if err := runCommand("helm", "repo", "add", "argo", "https://argoproj.github.io/argo-helm"); err != nil {
		log.Printf("❌ Failed to add argo repo")
		return fmt.Errorf("failed to add argo repo: %w", err)
	}

	// Update helm repos
	if err := runCommand("helm", "repo", "update"); err != nil {
		log.Printf("❌ Failed to update helm repo")
		return fmt.Errorf("failed to update helm repo: %w", err)
	}

	// Construct helm install command
	args := []string{
		"install", "argocd", "argo/argo-cd",
		"-n", a.Namespace,
	}
	if values != "" {
		args = append(args, "-f", values)
	}
	if valuesOverride != "" {
		args = append(args, "-f", valuesOverride)
	}
	if a.Version != "" {
		args = append(args, "--version", a.Version)
	}

	// Install ArgoCD
	if err := runCommand("helm", args...); err != nil {
		log.Printf("❌ Failed to install Argo CD")
		return fmt.Errorf("failed to install argo cd: %w", err)
	}

	// Wait for argocd-server to be ready
	log.Printf("🦑 Waiting for Argo CD to start...")
	if err := runCommand("kubectl", "wait", "--for=condition=available",
		"deployment/argocd-server", "-n", a.Namespace, "--timeout=300s"); err != nil {
		log.Printf("❌ Failed to wait for argocd-server")
		return fmt.Errorf("failed to wait for argocd-server: %w", err)
	}
	log.Printf("🦑 Argo CD is now available")

	// Get installed versions
	if err := a.logInstalledVersions(); err != nil {
		log.Printf("❌ Failed to get installed versions: %v", err)
	}

	// Login to ArgoCD
	if err := a.login(); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	return nil
}

func (a *ArgoCDInstallation) findValuesFiles() (string, string, error) {
	values := fmt.Sprintf("%s/values.yaml", a.ConfigPath)
	valuesOverride := fmt.Sprintf("%s/values-override.yaml", a.ConfigPath)

	if _, err := exec.Command("test", "-f", values).Output(); err != nil {
		values = ""
	}
	if _, err := exec.Command("test", "-f", valuesOverride).Output(); err != nil {
		valuesOverride = ""
	}

	return values, valuesOverride, nil
}

func (a *ArgoCDInstallation) runArgocdCommand(args ...string) error {
	cmd := exec.Command("argocd", args...)
	// Set ARGOCD_OPTS environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("ARGOCD_OPTS=--port-forward --port-forward-namespace=%s", a.Namespace))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("argocd command failed: %s: %w", string(output), err)
	}
	return nil
}

func (a *ArgoCDInstallation) login() error {
	log.Printf("🦑 Logging in to Argo CD through CLI...")

	// Get initial admin password
	password, err := a.getInitialPassword()
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)

	// Login to ArgoCD
	if err := a.runArgocdCommand("login", "localhost:8080", "--insecure", "--username", "admin", "--password", password); err != nil {
		log.Printf("❌ Failed to login to argocd")
		return fmt.Errorf("failed to login: %w", err)
	}

	// Verify login by listing apps
	if err := a.runArgocdCommand("app", "list"); err != nil {
		log.Printf("❌ Failed to list applications")
		return fmt.Errorf("failed to list applications: %w", err)
	}

	return nil
}

func (a *ArgoCDInstallation) getInitialPassword() (string, error) {
	secretName := "argocd-initial-admin-secret"
	cmd := fmt.Sprintf("kubectl -n %s get secret %s -o jsonpath={.data.password}",
		a.Namespace, secretName)

	var password []byte
	for retries := 0; retries < 5; retries++ {
		output, err := exec.Command("sh", "-c", cmd).Output()
		if err == nil {
			password = output
			break
		}
		if retries == 4 {
			return "", fmt.Errorf("failed to get secret %s: %w", secretName, err)
		}
		log.Printf("⏳ Retrying to get secret %s", secretName)
		time.Sleep(2 * time.Second)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(password))
	if err != nil {
		return "", fmt.Errorf("failed to decode password: %w", err)
	}

	return string(decoded), nil
}

func (a *ArgoCDInstallation) logInstalledVersions() error {
	_, err := exec.Command("helm", "list", "-A", "-o", "yaml").Output()
	if err != nil {
		return err
	}

	// Parse YAML and log versions
	// TODO: Implement YAML parsing similar to Rust version
	log.Printf("🦑 Helm release created successfully")
	return nil
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s: %w", string(output), err)
	}
	return nil
}

// AppsetGenerate runs 'argocd appset generate' on a file and returns the output
func (a *ArgoCDInstallation) AppsetGenerate(appSetPath string) (string, error) {
	cmd := exec.Command("argocd", "appset", "generate", appSetPath, "-o", "yaml")
	cmd.Env = append(os.Environ(), fmt.Sprintf("ARGOCD_OPTS=--port-forward --port-forward-namespace=%s", a.Namespace))

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("argocd appset generate failed: %s: %w", string(exitErr.Stderr), err)
		}
		return "", fmt.Errorf("failed to run argocd appset generate: %w", err)
	}

	return string(output), nil
}

func (a *ArgoCDInstallation) GetManifests(appName string) (string, error) {
	cmd := exec.Command("argocd", "app", "manifests", appName)
	cmd.Env = append(os.Environ(), fmt.Sprintf("ARGOCD_OPTS=--port-forward --port-forward-namespace=%s", a.Namespace))

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get manifests: %w", err)
	}

	return string(output), nil
}

func (a *ArgoCDInstallation) RefreshApp(appName string) error {
	cmd := exec.Command("argocd", "app", "get", appName, "--refresh")
	cmd.Env = append(os.Environ(), fmt.Sprintf("ARGOCD_OPTS=--port-forward --port-forward-namespace=%s", a.Namespace))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to refresh app: %w", err)
	}

	return nil
}
