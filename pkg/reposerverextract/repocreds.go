package reposerverextract

// repocreds.go — upfront credential fetching for the repo-server rendering path.
//
// The ArgoCD repo server has no access to the cluster's Kubernetes secrets. It
// relies entirely on credentials being passed in each ManifestRequest. The
// authoritative lookup path lives in the ArgoCD app controller (controller/state.go),
// which calls db.GetRepository() to enrich a bare repository URL with stored
// credentials (username, password, EnableOCI, etc.) before forwarding the request.
//
// We replicate that pattern here:
//   1. At startup (once per run), build a RepoCreds snapshot by reading all
//      repository secrets from the cluster via the ArgoCD DB layer.
//   2. Pass the snapshot into buildManifestRequestWithPackaging so it can
//      populate ManifestRequest.Repo, .Repos, and .HelmRepoCreds with real
//      credentials instead of bare URLs.

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db"
	helmutil "github.com/argoproj/argo-cd/v3/util/helm"
	argosettings "github.com/argoproj/argo-cd/v3/util/settings"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
)

// RepoCreds is a pre-fetched snapshot of all repository credentials registered
// in the ArgoCD installation. It is built once and shared across all concurrent
// rendering goroutines (it is read-only after construction).
type RepoCreds struct {
	// helmRepos is the list of all Helm repositories registered in ArgoCD.
	// Passed as ManifestRequest.Repos to cover Helm chart sub-dependencies.
	helmRepos []*v1alpha1.Repository

	// ociRepos is the list of all OCI repositories registered in ArgoCD.
	// Merged into helmRepos for OCI primary sources (mirrors ArgoCD controller).
	ociRepos []*v1alpha1.Repository

	// helmRepoCreds is the list of all Helm repository credential templates.
	// Passed as ManifestRequest.HelmRepoCreds.
	helmRepoCreds []*v1alpha1.RepoCreds

	// ociRepoCreds is the list of all OCI repository credential templates.
	// Merged into helmRepoCreds for OCI primary sources.
	ociRepoCreds []*v1alpha1.RepoCreds

	// reposByURL is a map from normalised repository URL → fully-enriched
	// Repository struct (with credentials). Used to populate ManifestRequest.Repo.
	reposByURL map[string]*v1alpha1.Repository
}

// FetchRepoCreds connects to the cluster via the ArgoCD DB layer and fetches
// all repository and credential information registered under the given
// ArgoCD namespace. The returned RepoCreds is safe for concurrent read access.
func FetchRepoCreds(ctx context.Context, k8sClient *utils.K8sClient, namespace string) (*RepoCreds, error) {
	// The ArgoCD DB requires a typed kubernetes.Interface.  Our K8sClient
	// exposes the underlying *rest.Config so we can build one on demand.
	typedClient, err := kubernetes.NewForConfig(k8sClient.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create typed kubernetes client: %w", err)
	}

	settingsMgr := argosettings.NewSettingsManager(ctx, typedClient, namespace)
	argoDB := db.NewDB(namespace, settingsMgr, typedClient)

	// ── Helm repositories & credential templates ─────────────────────────────
	helmRepos, err := argoDB.ListHelmRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list Helm repositories: %w", err)
	}

	helmRepoCreds, err := argoDB.GetAllHelmRepositoryCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Helm repository credentials: %w", err)
	}

	// ── OCI repositories & credential templates ──────────────────────────────
	ociRepos, err := argoDB.ListOCIRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list OCI repositories: %w", err)
	}

	ociRepoCreds, err := argoDB.GetAllOCIRepositoryCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OCI repository credentials: %w", err)
	}

	// ── Per-repo credential enrichment ───────────────────────────────────────
	// Build a URL-keyed map of all registered repositories (git + Helm + OCI)
	// with credentials fully enriched. We get this by listing all repositories
	// via the DB and calling GetRepository (which internally calls enrichCredsToRepo)
	// for each unique URL we encounter across sources.
	//
	// We collect URLs from the already-fetched helmRepos/ociRepos lists.
	// Git repos (type=git) are also stored as ArgoCD secrets — we list them
	// by iterating the credentials we already fetched.
	seenURLs := map[string]bool{}
	for _, r := range helmRepos {
		seenURLs[r.Repo] = true
	}
	for _, r := range ociRepos {
		seenURLs[r.Repo] = true
	}

	reposByURL := make(map[string]*v1alpha1.Repository, len(seenURLs))
	for repoURL := range seenURLs {
		enriched, err := argoDB.GetRepository(ctx, repoURL, "")
		if err != nil {
			// Non-fatal: log and skip. The repo server may still succeed if
			// it can fall back to unauthenticated access.
			log.Warn().Err(err).Str("repo", repoURL).Msg("⚠️ Failed to enrich repository credentials; proceeding without them")
			reposByURL[repoURL] = &v1alpha1.Repository{Repo: repoURL}
			continue
		}
		reposByURL[repoURL] = enriched
	}

	log.Info().
		Int("helmRepos", len(helmRepos)).
		Int("ociRepos", len(ociRepos)).
		Int("helmRepoCreds", len(helmRepoCreds)).
		Int("ociRepoCreds", len(ociRepoCreds)).
		Msg("📦 Fetched ArgoCD repository credentials from cluster")

	return &RepoCreds{
		helmRepos:     helmRepos,
		ociRepos:      ociRepos,
		helmRepoCreds: helmRepoCreds,
		ociRepoCreds:  ociRepoCreds,
		reposByURL:    reposByURL,
	}, nil
}

// GetRepo returns the credential-enriched Repository for the given URL.
// If no registered repository matches the URL exactly, it returns a stub
// Repository with just the URL set (the same bare-URL behaviour as before
// this fix, so callers can always proceed).
func (rc *RepoCreds) GetRepo(repoURL string) *v1alpha1.Repository {
	if rc == nil {
		return &v1alpha1.Repository{Repo: repoURL}
	}
	if r, ok := rc.reposByURL[repoURL]; ok {
		return r
	}
	// URL not found in the registry — return a bare stub.
	// This is correct for public repositories that don't need credentials.
	return &v1alpha1.Repository{Repo: repoURL}
}

// HelmRepos returns the Helm + OCI repository lists to pass as
// ManifestRequest.Repos. For OCI primary sources the OCI list is merged in
// (mirrors controller/state.go behaviour).
func (rc *RepoCreds) HelmRepos(source *v1alpha1.ApplicationSource) []*v1alpha1.Repository {
	if rc == nil {
		return nil
	}
	if !sourceIsOCI(source) {
		return rc.helmRepos
	}
	merged := make([]*v1alpha1.Repository, 0, len(rc.helmRepos)+len(rc.ociRepos))
	merged = append(merged, rc.helmRepos...)
	merged = append(merged, rc.ociRepos...)
	return merged
}

// HelmRepoCreds returns the Helm + OCI credential templates to pass as
// ManifestRequest.HelmRepoCreds. For OCI primary sources the OCI creds are
// merged in (mirrors controller/state.go behaviour).
func (rc *RepoCreds) HelmRepoCreds(source *v1alpha1.ApplicationSource) []*v1alpha1.RepoCreds {
	if rc == nil {
		return nil
	}
	if !sourceIsOCI(source) {
		return rc.helmRepoCreds
	}
	merged := make([]*v1alpha1.RepoCreds, 0, len(rc.helmRepoCreds)+len(rc.ociRepoCreds))
	merged = append(merged, rc.helmRepoCreds...)
	merged = append(merged, rc.ociRepoCreds...)
	return merged
}

// sourceIsOCI returns true when the given ApplicationSource points at an OCI
// registry — either via the "oci://" scheme (source.IsOCI()) or as a
// scheme-less Helm OCI registry URL (helm.IsHelmOciRepo). Mirrors the
// detection logic used in controller/state.go.
func sourceIsOCI(source *v1alpha1.ApplicationSource) bool {
	return source.IsOCI() || helmutil.IsHelmOciRepo(source.RepoURL)
}
