package argocd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	K8sClient         *utils.K8sClient
	Namespace         string
	Version           string
	ConfigPath        string
	ChartName         string
	ChartURL          string
	ChartRepoUsername string
	ChartRepoPassword string
	LoginOptions      string
	useAPI            bool       // true if using API mode, false for CLI mode
	operations        Operations // CLI or API implementation
}

func New(client *utils.K8sClient, namespace string, version string, repoName string, repoURL string, repoUsername string, repoPassword string, loginOptions string, useAPI bool, authToken string) *ArgoCDInstallation {
	return &ArgoCDInstallation{
		K8sClient:         client,
		Namespace:         namespace,
		Version:           version,
		ConfigPath:        "argocd-config",
		ChartName:         repoName,
		ChartURL:          repoURL,
		ChartRepoUsername: repoUsername,
		ChartRepoPassword: repoPassword,
		LoginOptions:      loginOptions,
		useAPI:            useAPI,
		operations:        NewOperations(useAPI, client, namespace, loginOptions, authToken),
	}
}

func (a *ArgoCDInstallation) UseAPI() bool {
	return a.useAPI
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
	if err := a.operations.Login(); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to login: %w", err)
	}

	// Check Argo CD version compatibility
	if err := a.operations.CheckVersionCompatibility(); err != nil {
		log.Error().Err(err).Msgf("❌ Failed to detect Argo CD version compatibility. Can't verify if the client version is compatible with the server version.")
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

	if err := a.operations.AddSourceNamespaceToDefaultAppProject(); err != nil {
		log.Error().Err(err).Msg("❌ Failed to add extra permissions to the default AppProject")
		return time.Since(startTime), fmt.Errorf("failed to add extra permissions to the default AppProject: %w", err)
	} else {
		log.Debug().Msgf("Argo CD extra permissions added successfully")
	}

	duration := time.Since(startTime)
	log.Info().Msgf("🦑 Argo CD installed successfully in %s", duration.Round(time.Second))

	return duration, nil
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
			Name:     a.ChartName,
			URL:      a.ChartURL,
			Username: a.ChartRepoUsername,
			Password: a.ChartRepoPassword,
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
				Name:     a.ChartName,
				URL:      a.ChartURL,
				Username: a.ChartRepoUsername,
				Password: a.ChartRepoPassword,
			})

			if err := r.WriteFile(repoFile, 0644); err != nil {
				return fmt.Errorf("failed to update repository file: %w", err)
			}
		}
	}

	// Update repository
	repoEntry := &repo.Entry{
		Name:     a.ChartName,
		URL:      a.ChartURL,
		Username: a.ChartRepoUsername,
		Password: a.ChartRepoPassword,
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

	// look for 'createClusterRoles' in chartValues
	if result, ok := chartValues["createClusterRoles"]; ok {
		if result == "false" {
			log.Info().Msgf("Installing with 'createClusterRoles: %s'", result)
			if !a.UseAPI() {
				log.Warn().Msgf("⚠️ Running Argo CD in locked-down mode. This will not work unless you use '--use-argocd-api=true'")
			}
		}
	}

	// convert chartValues to a string
	chartValuesBytes, err := yaml.Marshal(chartValues)
	if err != nil {
		return fmt.Errorf("failed to marshal chart values: %w", err)
	}
	chartValuesString := string(chartValuesBytes)

	log.Debug().Msgf("Chart values: \n%s", chartValuesString)

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

// OnlyLogin performs only the login step without installing ArgoCD
func (a *ArgoCDInstallation) OnlyLogin() (time.Duration, error) {
	startTime := time.Now()

	// Login to ArgoCD
	if err := a.operations.Login(); err != nil {
		return time.Since(startTime), fmt.Errorf("failed to login: %w", err)
	}

	log.Info().Msg("🦑 Logged in to Argo CD successfully")

	// Check Argo CD version compatibility
	if err := a.operations.CheckVersionCompatibility(); err != nil {
		log.Error().Err(err).Msgf("❌ Failed to detect Argo CD version compatibility. Can't verify if the client version is compatible with the server version.")
	}

	return time.Since(startTime), nil
}

// AppsetGenerate generates applications from an ApplicationSet
func (a *ArgoCDInstallation) AppsetGenerate(appSetPath string) (string, error) {
	return a.operations.AppsetGenerate(appSetPath)
}

// AppsetGenerateWithRetry runs AppsetGenerate with retry logic
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
			time.Sleep(1 * time.Second)
		}
	}

	return "", err
}

// GetManifests returns the manifests for an application
func (a *ArgoCDInstallation) GetManifests(appName string) ([]unstructured.Unstructured, bool, error) {
	return a.operations.GetManifests(appName)
}

// RefreshApp triggers a refresh of an application by setting the refresh annotation
func (a *ArgoCDInstallation) RefreshApp(appName string) error {
	return a.K8sClient.SetArgoCDAppRefreshAnnotation(a.Namespace, appName)
}

// EnsureArgoCdIsReady waits for ArgoCD deployments to be ready
func (a *ArgoCDInstallation) EnsureArgoCdIsReady() error {
	timeout := 300 * time.Second
	// Wait for argocd-server deployment to be ready
	// Use component label to support nameOverride configurations
	if err := a.K8sClient.WaitForDeploymentReady(a.Namespace, "app.kubernetes.io/component=server,app.kubernetes.io/part-of=argocd", int(timeout.Seconds())); err != nil {
		return fmt.Errorf("failed to wait for argocd-server to be ready: %w", err)
	}

	// Wait for argocd-repo-server deployment to be ready
	if err := a.K8sClient.WaitForDeploymentReady(a.Namespace, "app.kubernetes.io/component=repo-server,app.kubernetes.io/part-of=argocd", int(timeout.Seconds())); err != nil {
		return fmt.Errorf("failed to wait for argocd-repo-server to be ready: %w", err)
	}

	return nil
}

// Cleanup performs any necessary cleanup (e.g., stopping port forwards).
// This delegates to the operations implementation.
func (a *ArgoCDInstallation) Cleanup() {
	a.operations.Cleanup()
}

// IsExpectedError checks if an error message is expected for the current mode.
// In API mode, certain errors are expected when running with 'createClusterRoles: false'.
// In CLI mode, this always returns false.
// Returns: (isExpected, reason)
func (a *ArgoCDInstallation) IsExpectedError(errorMessage string) (bool, string) {
	return a.operations.IsExpectedError(errorMessage)
}
