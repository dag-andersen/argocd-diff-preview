package argocd

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"
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
	log.Debug().Msgf("Creating namespace: %s", a.Namespace)

	// Check if namespace exists
	cmd := fmt.Sprintf("kubectl get namespace %s", a.Namespace)
	if err := runCommand("sh", "-c", cmd); err == nil {
		log.Debug().Msgf("Namespace %s already exists", a.Namespace)
		return nil
	}

	// Create namespace
	if err := runCommand("kubectl", "create", "namespace", a.Namespace); err != nil {
		log.Error().Msgf("❌ Failed to create namespace %s", a.Namespace)
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	log.Debug().Msgf("Created namespace: %s", a.Namespace)
	return nil
}

func (a *ArgoCDInstallation) Install(debug bool, secretsFolder string) error {
	if a.Version != "" {
		log.Info().Msgf("🦑 Installing Argo CD Helm Chart version: '%s'", a.Version)
	} else {
		log.Info().Msg("🦑 Installing Argo CD Helm Chart version: 'latest'")
	}

	// Create namespace if it doesn't exist
	if err := a.createNamespace(); err != nil {
		return err
	}

	// Apply secrets before installing ArgoCD
	if err := utils.ApplySecretsFromFolder(secretsFolder, a.Namespace); err != nil {
		return fmt.Errorf("failed to apply secrets: %w", err)
	}

	// Install ArgoCD using Helm
	if err := a.installWithHelm(); err != nil {
		return err
	}

	// Wait for argocd-server to be ready
	log.Info().Msgf("🦑 Waiting for Argo CD to start...")
	if err := runCommand("kubectl", "wait", "--for=condition=available",
		"deployment/argocd-server", "-n", a.Namespace, "--timeout=300s"); err != nil {
		log.Error().Msgf("❌ Failed to wait for argocd-server")
		return fmt.Errorf("failed to wait for argocd-server: %w", err)
	}
	log.Info().Msg("🦑 Argo CD is now available")

	// Get installed versions
	if err := a.logInstalledVersions(); err != nil {
		log.Error().Msgf("❌ Failed to get installed versions: %v", err)
	}

	// Login to ArgoCD
	if err := a.login(); err != nil {
		return fmt.Errorf("failed to login: %w", err)
	}

	if debug {
		// Get ConfigMaps
		cmd := fmt.Sprintf("kubectl get configmap -n %s -o yaml argocd-cmd-params-cm argocd-cm", a.Namespace)
		output, err := utils.RunCommand(cmd)
		if err != nil {
			log.Error().Err(err).Msg("❌ Failed to get ConfigMaps")
			return fmt.Errorf("failed to get ConfigMaps: %w", err)
		}
		log.Debug().Msgf("🔧 ConfigMap argocd-cmd-params-cm and argocd-cm:\n%s", output)
	}

	// Add extra permissions to the default AppProject
	if _, err := a.runArgocdCommand("proj", "add-source-namespace", "default", "*"); err != nil {
		log.Error().Err(err).Msg("❌ Failed to add extra permissions to the default AppProject")
		return fmt.Errorf("failed to add extra permissions to the default AppProject: %w", err)
	}

	log.Info().Msgf("🦑 Argo CD installed successfully")

	return nil
}

// installWithHelm installs ArgoCD using Helm
func (a *ArgoCDInstallation) installWithHelm() error {
	// Check for values files
	valuesFiles, err := a.findValuesFiles()
	if err != nil {
		log.Info().Msgf("📂 Folder '%s' doesn't exist. Installing Argo CD Helm Chart with default configuration", a.ConfigPath)
	}

	// Add argo repo to helm
	if err := runCommand("helm", "repo", "add", "argo", "https://argoproj.github.io/argo-helm"); err != nil {
		log.Error().Msgf("❌ Failed to add argo repo")
		return fmt.Errorf("failed to add argo repo: %w", err)
	}

	// Update helm repos
	if err := runCommand("helm", "repo", "update"); err != nil {
		log.Error().Msgf("❌ Failed to update helm repo")
		return fmt.Errorf("failed to update helm repo: %w", err)
	}

	// Construct helm install command
	args := []string{
		"install", "argocd", "argo/argo-cd",
		"-n", a.Namespace,
	}
	for _, valuesFile := range valuesFiles {
		args = append(args, "-f", valuesFile)
	}
	if a.Version != "" {
		args = append(args, "--version", a.Version)
	}

	// Install ArgoCD
	if err := runCommand("helm", args...); err != nil {
		log.Error().Msgf("❌ Failed to install Argo CD")
		return fmt.Errorf("failed to install argo cd: %w", err)
	}

	return nil
}

func (a *ArgoCDInstallation) findValuesFiles() ([]string, error) {

	log.Debug().Msgf("📂 Files in folder '%s':", a.ConfigPath)

	files, err := os.ReadDir(a.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read folder: %w", err)
	}

	var foundValues bool
	var foundValuesOverride bool

	for _, file := range files {
		log.Debug().Msgf("- 📄 %s", file.Name())

		name := file.Name()
		if name == "values.yaml" {
			foundValues = true
		}
		if name == "values-override.yaml" {
			foundValuesOverride = true
		}
	}

	valuesFiles := []string{}
	if foundValues {
		valuesFiles = append(valuesFiles, fmt.Sprintf("%s/values.yaml", a.ConfigPath))
	}
	if foundValuesOverride {
		valuesFiles = append(valuesFiles, fmt.Sprintf("%s/values-override.yaml", a.ConfigPath))
	}

	return valuesFiles, nil
}

func (a *ArgoCDInstallation) runArgocdCommand(args ...string) (string, error) {
	cmd := exec.Command("argocd", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("ARGOCD_OPTS=--port-forward --port-forward-namespace=%s", a.Namespace))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("argocd command failed: %s: %w", string(output), err)
	}
	return string(output), nil
}

func (a *ArgoCDInstallation) login() error {
	log.Info().Msgf("🦑 Logging in to Argo CD through CLI...")

	// Get initial admin password
	password, err := a.getInitialPassword()
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)

	// Login to ArgoCD
	if _, err := a.runArgocdCommand("login", "localhost:8080", "--insecure", "--username", "admin", "--password", password); err != nil {
		log.Error().Msgf("❌ Failed to login to argocd")
		return fmt.Errorf("failed to login: %w", err)
	}

	// Verify login by listing apps
	if _, err := a.runArgocdCommand("app", "list"); err != nil {
		log.Error().Msgf("❌ Failed to list applications")
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
		log.Info().Msgf("⏳ Retrying to get secret %s", secretName)
		time.Sleep(2 * time.Second)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(password))
	if err != nil {
		return "", fmt.Errorf("failed to decode password: %w", err)
	}

	return string(decoded), nil
}

func (a *ArgoCDInstallation) logInstalledVersions() error {
	output, err := exec.Command("helm", "list", "-A", "-o", "yaml").Output()
	if err != nil {
		return fmt.Errorf("failed to list helm charts: %w", err)
	}

	var helmList []map[string]interface{}
	if err := yaml.Unmarshal(output, &helmList); err != nil {
		return fmt.Errorf("failed to parse helm list output: %w", err)
	}

	if len(helmList) > 0 {
		chartVersion := helmList[0]["chart"]
		appVersion := helmList[0]["app_version"]
		if chartVersion != nil && appVersion != nil {
			log.Info().Msgf("🦑 Installed Chart version: '%v' and App version: '%v'",
				chartVersion, appVersion)
		} else {
			log.Error().Msgf("❌ Failed to get chart version")
		}
	} else {
		log.Error().Msgf("❌ Failed to get chart version")
	}

	return nil
}

func runCommand(name string, args ...string) error {
	log.Debug().Msgf("Running command: %s %s", name, strings.Join(args, " "))
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
