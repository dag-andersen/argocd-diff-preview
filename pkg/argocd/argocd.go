package argocd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

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

const (
	// remotePort is the port that the ArgoCD server pod listens on
	remotePort = 8080
)

type ArgoCDInstallation struct {
	K8sClient            *utils.K8sClient
	Namespace            string
	Version              string
	ConfigPath           string
	ChartName            string
	ChartURL             string
	Username             string
	Password             string
	portForwardActive    bool
	portForwardMutex     sync.Mutex
	portForwardStopChan  chan struct{}
	portForwardLocalPort int    // Local port for port forwarding (e.g., 8081)
	apiServerURL         string // Constructed API server URL (e.g., "http://localhost:8081")
	authToken            string // Cached authentication token
}

func New(client *utils.K8sClient, namespace string, version string, repoName string, repoURL string, username string, password string) *ArgoCDInstallation {
	localPort := 8081
	return &ArgoCDInstallation{
		K8sClient:            client,
		Namespace:            namespace,
		Version:              version,
		ConfigPath:           "argocd-config",
		ChartName:            repoName,
		ChartURL:             repoURL,
		Username:             username,
		Password:             password,
		portForwardLocalPort: localPort,
		apiServerURL:         fmt.Sprintf("http://localhost:%d", localPort),
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
		return time.Since(startTime), fmt.Errorf("failed to apply secrets: %w from folder: %s", err, secretsFolder)
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

// GetManifests returns the manifests for an application using the ArgoCD API
func (a *ArgoCDInstallation) GetManifests(appName string) (string, error) {
	// Ensure port forward is active
	if err := a.portForwardToArgoCD(); err != nil {
		return "", err
	}

	// Make API request to get manifests
	url := fmt.Sprintf("%s/api/v1/applications/%s/manifests", a.apiServerURL, appName)

	log.Debug().Msgf("Getting manifests for app %s from API: %s", appName, url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set authorization header with bearer token
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.authToken))

	// Create HTTP client with TLS config to handle redirects
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		// Check if app exists
		exists, _ := a.K8sClient.CheckIfResourceExists(ApplicationGVR, a.Namespace, appName)
		if !exists {
			log.Warn().Msgf("App %s does not exist", appName)
		}
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode == 404 {
		log.Warn().Msgf("App %s does not exist (404)", appName)
		return "", fmt.Errorf("application not found: %s", appName)
	}

	if resp.StatusCode != http.StatusOK {

		var response struct {
			Error string `json:"error"`
		}

		if err := json.Unmarshal(body, &response); err == nil {
			return "", fmt.Errorf("ArgoCD API returned error: %s", response.Error)
		}

		return "", fmt.Errorf("ArgoCD API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response to extract manifests
	// The API returns manifests as an array of JSON strings, not objects
	var manifestResponse struct {
		Manifests []string `json:"manifests"`
	}

	if err := json.Unmarshal(body, &manifestResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal manifests response: %w", err)
	}

	// Convert manifests to YAML format with --- separators
	// Each manifest is already a JSON string, we need to convert each to YAML
	var manifestsYAML strings.Builder
	for i, manifestStr := range manifestResponse.Manifests {
		// The manifest is a JSON string, convert it to YAML
		manifestYAML, err := yaml.JSONToYAML([]byte(manifestStr))
		if err != nil {
			return "", fmt.Errorf("failed to convert manifest %d to YAML: %w", i, err)
		}

		// Write separator between manifests (except for the first one)
		if i > 0 {
			manifestsYAML.WriteString("---\n")
		}

		// Write the YAML manifest
		manifestsYAML.Write(manifestYAML)
	}

	log.Debug().Msgf("Successfully retrieved %d manifests for app %s", len(manifestResponse.Manifests), appName)
	return manifestsYAML.String(), nil
}

// GetManifestsWithRetry returns the manifests for an application with retry
func (a *ArgoCDInstallation) GetManifestsWithRetry(appName string, maxAttempts int) (string, error) {

	var err error
	var out string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			log.Debug().Msgf("GetManifestsWithRetry attempt %d/%d to Argo CD...", attempt, maxAttempts)
		} else {
			log.Debug().Msgf("GetManifestsWithRetry to Argo CD...")
		}
		out, err = a.GetManifests(appName)
		if err == nil {
			return out, nil
		}

		if attempt < maxAttempts {
			log.Debug().Msgf("Waiting 1s before next get manifests attempt (%d/%d)...", attempt+1, maxAttempts)
			log.Warn().Err(err).Msgf("⚠️ Get manifests attempt %d/%d failed.", attempt, maxAttempts)
			time.Sleep(1 * time.Second)
		}
	}

	return out, err
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
