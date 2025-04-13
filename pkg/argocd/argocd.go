package argocd

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
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
	if err := runCommand("kubectl", "get", "namespace", a.Namespace); err == nil {
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
	installLatest := strings.TrimSpace(a.Version) == "" || strings.TrimSpace(a.Version) == "latest"
	chartVersion := ""
	if !installLatest {
		chartVersion = a.Version
		log.Info().Msgf("🦑 Installing Argo CD Helm Chart version: '%s'", a.Version)
	} else {
		log.Info().Msg("🦑 Installing Argo CD Helm Chart version: 'latest'")
	}

	// Check for values files
	valuesFiles, err := a.findValuesFiles()
	if err != nil {
		log.Info().Msgf("📂 Folder '%s' doesn't exist. Installing Argo CD Helm Chart with default configuration", a.ConfigPath)
	}

	// Initialize Helm client settings
	settings := cli.New()

	// Setup repository
	repoName := "argo"
	repoURL := "https://argoproj.github.io/argo-helm"

	// Try to add the repo first
	repoFile := settings.RepositoryConfig

	// Create repository config if it doesn't exist
	if _, err := os.Stat(repoFile); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(repoFile), 0755)

		// Create a new repository file
		r := repo.NewFile()
		r.Add(&repo.Entry{
			Name: repoName,
			URL:  repoURL,
		})

		if err := r.WriteFile(repoFile, 0644); err != nil {
			return fmt.Errorf("failed to write repository file: %w", err)
		}
	} else {
		// Update existing repository
		r, err := repo.LoadFile(repoFile)
		if err != nil {
			return fmt.Errorf("failed to load repository file: %w", err)
		}

		if !r.Has(repoName) {
			r.Add(&repo.Entry{
				Name: repoName,
				URL:  repoURL,
			})

			if err := r.WriteFile(repoFile, 0644); err != nil {
				return fmt.Errorf("failed to update repository file: %w", err)
			}
		}
	}

	// Update repository
	repoEntry := &repo.Entry{
		Name: repoName,
		URL:  repoURL,
	}

	chartRepo, err := repo.NewChartRepository(repoEntry, getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to create chart repository: %w", err)
	}

	if _, err := chartRepo.DownloadIndexFile(); err != nil {
		return fmt.Errorf("failed to download index file: %w", err)
	}

	// Initialize the action configuration
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), a.Namespace, os.Getenv("HELM_DRIVER"), log.Debug().Msgf); err != nil {
		return fmt.Errorf("failed to initialize helm configuration: %w", err)
	}

	// Create the install action
	client := action.NewInstall(actionConfig)
	client.Namespace = a.Namespace
	client.ReleaseName = "argocd"
	client.CreateNamespace = false // We already created the namespace
	client.Wait = true
	client.Timeout = 300 * time.Second

	if chartVersion != "" {
		client.Version = chartVersion
	}

	// Locate chart
	chartName := fmt.Sprintf("%s/argo-cd", repoName)
	chartPath, err := client.ChartPathOptions.LocateChart(chartName, settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	// Load chart
	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Load values from files
	valueOpts := &values.Options{
		ValueFiles: valuesFiles,
	}
	chartValues, err := valueOpts.MergeValues(getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to merge values: %w", err)
	}

	// Install chart
	_, err = client.Run(chart, chartValues)
	if err != nil {
		return fmt.Errorf("failed to install chart: %w", err)
	}

	// Log installed versions
	log.Info().Msgf("🦑 Installed Chart version: '%s' and App version: '%s'",
		chart.Metadata.Version, chart.Metadata.AppVersion)

	log.Info().Msg("🦑 Argo CD Helm chart installed successfully")
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
		valuesFiles = append(valuesFiles, filepath.Join(a.ConfigPath, "values.yaml"))
	}
	if foundValuesOverride {
		valuesFiles = append(valuesFiles, filepath.Join(a.ConfigPath, "values-override.yaml"))
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
	cmd_split := strings.Split(cmd, " ")

	var password []byte
	for retries := 0; retries < 5; retries++ {
		output, err := exec.Command(cmd_split[0], cmd_split[1:]...).Output()
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

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to refresh app: %s", output)
	}

	return nil
}
