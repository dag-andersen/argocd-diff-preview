package reposerverextract

// chartpull.go - pulling remote Helm charts to a local directory.
//
// Some multi-source Applications use a remote Helm chart (an external Helm
// registry, HTTP or OCI) whose value files come from a $ref source that lives
// in the same repository being compared (e.g. cert-manager + an
// envs/<env>/values.yaml file checked out in the base or target branch folder).
//
// The repo server's file-streaming RPC (GenerateManifestWithFiles) resolves the
// chart from the streamed tarball and never pulls it from a registry. To render
// such an Application with the checked-out value files we therefore pull the
// remote chart ourselves and place it inside the streamed tree alongside the
// ref directories. From there the repo server renders it as an ordinary local
// path chart. See buildManifestRequestForSource for how the result is used.

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/rs/zerolog/log"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/registry"
)

// chartPuller fetches a remote Helm chart and extracts it into destDir,
// returning the path to the extracted chart directory (the directory that
// contains Chart.yaml). Implementations must place all chart files under
// destDir so the caller can stream them to the repo server.
type chartPuller interface {
	Pull(source v1alpha1.ApplicationSource, creds *RepoCreds, destDir string) (chartDir string, err error)
}

// helmChartPuller is the production chartPuller. It uses the Helm SDK's pull
// action to download and untar a chart from an HTTP(S) or OCI Helm registry.
// It is stateless, so a zero value is safe to use concurrently (each Pull call
// works in its own destDir with an isolated Helm configuration).
type helmChartPuller struct{}

// Pull downloads source.Chart at source.TargetRevision from source.RepoURL and
// untars it under destDir. Credentials for the chart registry are looked up in
// creds (which may be nil for public registries).
func (helmChartPuller) Pull(source v1alpha1.ApplicationSource, creds *RepoCreds, destDir string) (string, error) {
	if source.Chart == "" {
		return "", fmt.Errorf("application source has no chart to pull (repoURL %q)", source.RepoURL)
	}

	// Isolate the Helm configuration (repositories.yaml, registry config, index
	// cache) inside destDir so we never read or mutate the user's ~/.config/helm
	// or ~/.cache/helm.
	helmHome := filepath.Join(destDir, ".helm")
	if err := os.MkdirAll(helmHome, 0o755); err != nil {
		return "", fmt.Errorf("failed to create helm home: %w", err)
	}
	settings := cli.New()
	settings.RepositoryConfig = filepath.Join(helmHome, "repositories.yaml")
	settings.RepositoryCache = filepath.Join(helmHome, "cache")
	settings.RegistryConfig = filepath.Join(helmHome, "registry-config.json")

	repo := creds.GetRepo(source.RepoURL)

	untarDir := filepath.Join(destDir, "chart")
	if err := os.MkdirAll(untarDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create chart dir: %w", err)
	}

	actionConfig := &action.Configuration{}
	pull := action.NewPullWithOpts(action.WithConfig(actionConfig))
	pull.Settings = settings
	pull.Version = source.TargetRevision
	pull.Username = repo.Username
	pull.Password = repo.Password
	pull.Untar = true
	pull.DestDir = untarDir
	pull.UntarDir = untarDir

	chartRef := source.Chart
	if sourceIsOCI(&source) {
		registryClient, err := registry.NewClient(
			registry.ClientOptDebug(settings.Debug),
			registry.ClientOptWriter(io.Discard),
			registry.ClientOptCredentialsFile(settings.RegistryConfig),
		)
		if err != nil {
			return "", fmt.Errorf("failed to create OCI registry client: %w", err)
		}
		actionConfig.RegistryClient = registryClient
		pull.SetRegistryClient(registryClient)

		if repo.Username != "" && repo.Password != "" {
			host := ociRegistryHost(source.RepoURL)
			log.Debug().Str("registry", host).Msg("Logging in to OCI registry to pull chart")
			if err := registryClient.Login(host,
				registry.LoginOptBasicAuth(repo.Username, repo.Password),
			); err != nil {
				return "", fmt.Errorf("failed to log in to OCI registry %s: %w", host, err)
			}
		}
		chartRef = ociChartRef(source.RepoURL, source.Chart)
	} else {
		// Classic HTTP(S) repository: the pull action resolves the chart through
		// the repo index when RepoURL is set, so no repositories.yaml entry is
		// required up front.
		pull.RepoURL = source.RepoURL
	}

	log.Debug().
		Str("chart", source.Chart).
		Str("repoURL", source.RepoURL).
		Str("version", source.TargetRevision).
		Msg("Pulling remote Helm chart for local rendering")

	if out, err := pull.Run(chartRef); err != nil {
		return "", fmt.Errorf("failed to pull chart %q from %q (version %q): %w: %s",
			source.Chart, source.RepoURL, source.TargetRevision, err, strings.TrimSpace(out))
	}

	chartDir, err := findChartDir(untarDir)
	if err != nil {
		return "", fmt.Errorf("failed to locate pulled chart %q: %w", source.Chart, err)
	}
	return chartDir, nil
}

// findChartDir returns the directory under root that contains a Chart.yaml.
// helm pull --untar expands the chart into a subdirectory named after the
// chart, so the common case is a single child directory. A recursive walk is
// used as a fallback for unusual layouts.
func findChartDir(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(root, e.Name())
		if _, statErr := os.Stat(filepath.Join(candidate, "Chart.yaml")); statErr == nil {
			return candidate, nil
		}
	}

	var found string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "Chart.yaml" {
			found = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	if found != "" {
		return found, nil
	}
	return "", fmt.Errorf("no Chart.yaml found under %s after pulling chart", root)
}

// ociChartRef builds an oci:// chart reference from an Argo CD OCI Helm source,
// where repoURL is the registry/repository base and chart is the chart name,
// e.g. ("ghcr.io/org/charts", "podinfo") -> "oci://ghcr.io/org/charts/podinfo".
// A repoURL that already carries the oci:// scheme is preserved.
func ociChartRef(repoURL, chart string) string {
	base := strings.TrimSuffix(strings.TrimSpace(repoURL), "/")
	base = "oci://" + strings.TrimPrefix(base, "oci://")
	return base + "/" + chart
}

// ociRegistryHost extracts the registry host from an OCI repository URL,
// e.g. "oci://ghcr.io/org/charts" or "ghcr.io/org/charts" -> "ghcr.io".
func ociRegistryHost(repoURL string) string {
	rest := strings.TrimPrefix(strings.TrimSpace(repoURL), "oci://")
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}
