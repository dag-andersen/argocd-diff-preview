package argocd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

// Common resource GVRs
var (
	// ApplicationGVR is the GroupVersionResource for ArgoCD applications
	ApplicationGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}
)

type ArgoCDInstallation struct {
	K8sClient  *utils.K8sClient
	Namespace  string
	Version    string
	ConfigPath string
	ChartName  string
	ChartURL   string
}

func New(client *utils.K8sClient, namespace string, version string, repoName string, repoURL string) *ArgoCDInstallation {
	return &ArgoCDInstallation{
		K8sClient:  client,
		Namespace:  namespace,
		Version:    version,
		ConfigPath: "argocd-config",
		ChartName:  repoName,
		ChartURL:   repoURL,
	}
}

func (a *ArgoCDInstallation) Install(debug bool, secretsFolder string) (time.Duration, error) {
	startTime := time.Now()
	log.Debug().Msgf("Creating namespace: %s", a.Namespace)

	// Check if namespace exists
	created, err := a.K8sClient.CreateNamespace(a.Namespace)
	if err != nil {
		log.Error().Msgf("❌ Failed to create namespace %s", a.Namespace)
		return time.Since(startTime), fmt.Errorf("failed to create namespace: %w", err)
	}

	if created {
		log.Debug().Msgf("Created namespace: %s", a.Namespace)
	} else {
		log.Debug().Msgf("Namespace already exists: %s", a.Namespace)
	}

	// Apply secrets before installing ArgoCD
	if err := ApplySecretsFromFolder(a.K8sClient, secretsFolder, a.Namespace); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to apply secrets from folder: %s: %w", secretsFolder, err)
	}

	// Install ArgoCD using Helm
	if err := a.installWithHelm(); err != nil {
		return time.Since(startTime), err
	}

	// Login to ArgoCD
	if err := a.login(); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to login: %w", err)
	}

	if debug {
		// Get ConfigMaps
		configMaps, err := a.K8sClient.GetConfigMaps(a.Namespace, "argocd-cmd-params-cm", "argocd-cm")
		if err != nil {
			log.Error().Err(err).Msg("❌ Failed to get ConfigMaps")
			return time.Since(startTime), fmt.Errorf("failed to get ConfigMaps: %w", err)
		}
		log.Debug().Msgf("🔧 ConfigMap argocd-cmd-params-cm and argocd-cm:\n%s", configMaps)
	}

	// Add extra permissions to the default AppProject
	if _, err := a.runArgocdCommand("proj", "add-source-namespace", "default", "*"); err != nil {
		log.Error().Err(err).Msg("❌ Failed to add extra permissions to the default AppProject")
		return time.Since(startTime), fmt.Errorf("failed to add extra permissions to the default AppProject: %w", err)
	} else {
		log.Debug().Msgf("Argo CD extra permissions added successfully")
	}

	duration := time.Since(startTime)
	log.Info().Msgf("🦑 Argo CD installed successfully in %s", duration.Round(time.Second))

	return duration, nil
}

func (a *ArgoCDInstallation) OnlyLogin() (time.Duration, error) {
	startTime := time.Now()

	// Login to ArgoCD
	if err := a.login(); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to login: %w", err)
	}

	log.Info().Msg("🦑 Logged in to Argo CD successfully")

	return time.Since(startTime), nil
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

	// Try to add the repo first
	repoFile := settings.RepositoryConfig

	// Create repository config if it doesn't exist
	if _, err := os.Stat(repoFile); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(repoFile), 0755); err != nil {
			return fmt.Errorf("failed to create repository directory: %w", err)
		}

		// Create a new repository file
		r := repo.NewFile()
		r.Add(&repo.Entry{
			Name: a.ChartName,
			URL:  a.ChartURL,
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

		if !r.Has(a.ChartName) {
			r.Add(&repo.Entry{
				Name: a.ChartName,
				URL:  a.ChartURL,
			})

			if err := r.WriteFile(repoFile, 0644); err != nil {
				return fmt.Errorf("failed to update repository file: %w", err)
			}
		}
	}

	// Update repository
	repoEntry := &repo.Entry{
		Name: a.ChartName,
		URL:  a.ChartURL,
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

	timeout := 300 * time.Second

	// Create the install action
	helmClient := action.NewInstall(actionConfig)
	helmClient.Namespace = a.Namespace
	helmClient.ReleaseName = "argocd"
	helmClient.CreateNamespace = false // We already created the namespace
	helmClient.Wait = false
	helmClient.WaitForJobs = false
	helmClient.Timeout = timeout

	if chartVersion != "" {
		helmClient.Version = chartVersion
	}

	// Locate chart
	chartName := fmt.Sprintf("%s/argo-cd", a.ChartName)
	chartPath, err := helmClient.LocateChart(chartName, settings)
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

	log.Debug().Msgf("Installing Argo CD Helm Chart with timeout: %s", timeout)

	// Install chart in go routine
	go func() {
		_, err = helmClient.Run(chart, chartValues)
		if err != nil {
			log.Error().Err(err).Msgf("❌ Failed to install chart")
		}
	}()

	// Wait for deployment to be ready
	if err := a.EnsureArgoCdIsReady(); err != nil {
		return fmt.Errorf("failed to wait for deployments to be ready: %w", err)
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
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("argocd command failed: %s: %w", string(exitErr.Stderr), err)
		}
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

	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("Login attempt %d/%d to Argo CD...", attempt, maxAttempts)
		out, err := a.runArgocdCommand("login", "--insecure", "--username", "admin", "--password", password)
		if err == nil {
			log.Debug().Msgf("Login successful on attempt %d. Output: %s", attempt, out)
			break
		}

		if attempt >= maxAttempts {
			log.Error().Err(err).Msgf("❌ Failed to login to Argo CD after %d attempts", maxAttempts)
			return fmt.Errorf("failed to login after %d attempts", maxAttempts)
		}

		log.Debug().Msgf("Waiting 1s before next login attempt (%d/%d)...", attempt+1, maxAttempts)
		log.Warn().Err(err).Msgf("Argo CD login attempt %d/%d failed.", attempt, maxAttempts)
		time.Sleep(1 * time.Second)
	}

	log.Debug().Msg("Verifying login by listing applications...")
	if _, errList := a.runArgocdCommand("app", "list"); errList != nil {
		log.Error().Err(errList).Msg("❌ Failed to list applications after login (verification step).")
		return fmt.Errorf("login verification failed (unable to list applications): %w", errList)
	}

	return nil
}

func (a *ArgoCDInstallation) getInitialPassword() (string, error) {

	secret, err := a.K8sClient.GetSecretValue(a.Namespace, "argocd-initial-admin-secret", "password")
	if err != nil {
		log.Error().Msgf("❌ Failed to get secret: %s", err)
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	return secret, nil
}

// AppsetGenerate runs 'argocd appset generate' on a file and returns the output
func (a *ArgoCDInstallation) AppsetGenerate(appSetPath string) (string, error) {
	out, err := a.runArgocdCommand("appset", "generate", appSetPath, "-o", "yaml")
	if err != nil {
		return "", fmt.Errorf("failed to run argocd appset generate: %w", err)
	}

	return out, nil
}

// AppsetGenerateWithRetry runs 'argocd appset generate' on a file and returns the output with retry
func (a *ArgoCDInstallation) AppsetGenerateWithRetry(appSetPath string, maxAttempts int) (string, error) {

	var err error
	var out string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("AppsetGenerateWithRetry attempt %d/%d to Argo CD...", attempt, maxAttempts)
		out, err = a.AppsetGenerate(appSetPath)
		if err == nil {
			return out, nil
		}

		if attempt < maxAttempts {
			log.Debug().Msgf("Waiting 1s before next appset generate attempt (%d/%d)...", attempt+1, maxAttempts)
			log.Warn().Err(err).Msgf("Appset generate attempt %d/%d failed.", attempt, maxAttempts)
			time.Sleep(1 * time.Second)
		}
	}

	return "", err
}

// GetManifests returns the manifests for an application
func (a *ArgoCDInstallation) GetManifests(appName string) (string, bool, error) {
	out, err := a.runArgocdCommand("app", "manifests", appName)
	if err != nil {
		exists, _ := a.K8sClient.CheckIfResourceExists(ApplicationGVR, a.Namespace, appName)
		if !exists {
			log.Warn().Msgf("App %s does not exist", appName)
		}

		return "", exists, fmt.Errorf("failed to get manifests for app: %w", err)
	}

	return out, true, nil
}

// GetManifestsWithRetry returns the manifests for an application with retry
func (a *ArgoCDInstallation) GetManifestsWithRetry(appName string, maxAttempts int) (string, bool, error) {

	var err error
	var exists bool
	var out string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		log.Debug().Msgf("GetManifestsWithRetry attempt %d/%d to Argo CD...", attempt, maxAttempts)
		out, exists, err = a.GetManifests(appName)
		if err == nil {
			return out, exists, nil
		}

		if attempt < maxAttempts {
			log.Debug().Msgf("Waiting 1s before next get manifests attempt (%d/%d)...", attempt+1, maxAttempts)
			log.Warn().Err(err).Msgf("⚠️ Get manifests attempt %d/%d failed.", attempt, maxAttempts)
			time.Sleep(1 * time.Second)
		}
	}

	return out, exists, err
}

func (a *ArgoCDInstallation) RefreshApp(appName string) error {
	_, err := a.runArgocdCommand("app", "get", appName, "--refresh")
	if err != nil {
		return fmt.Errorf("failed to refresh app: %w", err)
	}

	return nil
}

func (a *ArgoCDInstallation) EnsureArgoCdIsReady() error {
	timeout := 300 * time.Second
	// Wait for deployment to be ready
	if err := a.K8sClient.WaitForDeploymentReady(a.Namespace, "argocd-server", int(timeout.Seconds())); err != nil {
		return fmt.Errorf("failed to wait for argocd-server to be ready: %w", err)
	}

	if err := a.K8sClient.WaitForDeploymentReady(a.Namespace, "argocd-repo-server", int(timeout.Seconds())); err != nil {
		return fmt.Errorf("failed to wait for argocd-repo-server to be ready: %w", err)
	}

	return nil
}
