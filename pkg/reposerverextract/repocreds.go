package reposerverextract

// repocreds.go - upfront credential fetching for the repo-server rendering path.
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
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db"
	helmutil "github.com/argoproj/argo-cd/v3/util/helm"
	argosettings "github.com/argoproj/argo-cd/v3/util/settings"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/kubernetes"

	"github.com/dag-andersen/argocd-diff-preview/pkg/k8s"
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
//
// appRepoURLs is the set of repository URLs referenced by all Applications that
// will be rendered. For each URL, FetchRepoCreds calls argoDB.GetRepository()
// which—unlike ListRepositories—also inherits credentials from "repo-creds"
// type secrets (credential templates) via prefix matching. This mirrors the
// enrichment path used by the ArgoCD app controller in controller/state.go.
//
// Without this, users who only configure a repo-creds secret (common for
// GitHub token authentication across many repositories) would get bare stubs
// with no credentials, causing "authentication required" errors from the repo
// server.
func FetchRepoCreds(ctx context.Context, k8sClient *k8s.Client, namespace string, appRepoURLs []string) (*RepoCreds, error) {
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
	// ListRepositories returns all registered repos (git + Helm + OCI) with
	// credentials already enriched via enrichCredsToRepos - one API call
	// instead of a per-URL loop.
	allRepos, err := argoDB.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	reposByURL := make(map[string]*v1alpha1.Repository, len(allRepos))
	for _, r := range allRepos {
		reposByURL[normalizeRepoURL(r.Repo)] = r
	}

	// ── Credential-template inheritance for app repo URLs ────────────────────
	// ListRepositories only returns repos that have an explicit "repository"
	// type secret. Users who rely solely on "repo-creds" (credential templates)
	// won't have their URLs in the list above. For those URLs we call
	// argoDB.GetRepository() which creates a bare Repository and then enriches
	// it via the repo-creds prefix-matching path — exactly what the ArgoCD app
	// controller does in controller/state.go before calling the repo server.
	repoCredTemplates := 0
	for _, rawURL := range appRepoURLs {
		key := normalizeRepoURL(rawURL)
		if _, exists := reposByURL[key]; exists {
			continue // already have credentials from a "repository" secret
		}
		repo, err := argoDB.GetRepository(ctx, rawURL, "")
		if err != nil {
			log.Warn().Err(err).Str("repoURL", rawURL).
				Msg("⚠️ Failed to look up repository credentials for app repo URL")
			continue
		}
		if repo.HasCredentials() {
			reposByURL[key] = repo
			repoCredTemplates++
		}
	}

	if len(helmRepos)+len(ociRepos)+len(helmRepoCreds)+len(ociRepoCreds)+len(allRepos)+repoCredTemplates > 0 {
		log.Info().
			Int("helmRepos", len(helmRepos)).
			Int("ociRepos", len(ociRepos)).
			Int("helmRepoCreds", len(helmRepoCreds)).
			Int("ociRepoCreds", len(ociRepoCreds)).
			Int("repos", len(allRepos)).
			Int("repoCredTemplates", repoCredTemplates).
			Msg("📦 Fetched Argo CD repository credentials from cluster")
	} else {
		log.Info().Msg("📦 No Argo CD repository credentials found in cluster")
	}

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
	if r, ok := rc.reposByURL[normalizeRepoURL(repoURL)]; ok {
		return r
	}
	// URL not found in the registry - return a bare stub.
	// This is correct for public repositories that don't need credentials.
	return &v1alpha1.Repository{Repo: repoURL}
}

// normalizeRepoURL returns a canonical form of a repository URL used for
// credential lookups. It lowercases the URL and strips a trailing ".git"
// suffix so that secrets stored without ".git" match app repoURLs that include
// it (and vice versa). For example:
//
//	https://github.com/StoryHouse-SubscriptionSystems/argo-apps
//	https://github.com/StoryHouse-SubscriptionSystems/argo-apps.git
//
// both normalise to the same key.
func normalizeRepoURL(u string) string {
	u = strings.ToLower(u)
	u = strings.TrimSuffix(u, ".git")
	return u
}

// repoURLContains reports whether the normalised form of repoURL contains the
// normalised form of substr. This is used to match source repoURLs against the
// --repo flag value which can be either a full URL
// ("https://github.com/org/repo.git") or a short slug ("org/repo").
//
// Using a substring match (like the patching code's containsIgnoreCase) keeps
// the comparison provider-agnostic — we don't assume GitHub, GitLab, etc.
func repoURLContains(repoURL, substr string) bool {
	return strings.Contains(normalizeRepoURL(repoURL), normalizeRepoURL(substr))
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
// registry - either via the "oci://" scheme (source.IsOCI()) or as a
// scheme-less Helm OCI registry URL (helm.IsHelmOciRepo). Mirrors the
// detection logic used in controller/state.go.
func sourceIsOCI(source *v1alpha1.ApplicationSource) bool {
	return source.IsOCI() || helmutil.IsHelmOciRepo(source.RepoURL)
}
