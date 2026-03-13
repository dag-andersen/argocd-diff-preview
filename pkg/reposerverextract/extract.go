// Package reposerverextract provides an alternative to pkg/extract that renders
// Argo CD Application manifests by streaming local source files directly to the
// Argo CD repo server via gRPC, instead of deploying Applications to the cluster
// and polling until they are reconciled.
//
// This approach is faster and simpler: no cluster-side Application objects are
// created, there is no reconciliation loop to wait for, and manifests are
// returned synchronously.
//
// The entry point is RenderApplicationsFromBothBranches, which has the same
// return type as extract.RenderApplicationsFromBothBranches so callers can
// switch between the two with minimal code changes.
package reposerverextract

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	argocdPkg "github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/reposerver"
)

// resourceInfoProvider implements kubeutil.ResourceInfoProvider to supply
// namespace-scope information for Kubernetes resources.
type resourceInfoProvider struct {
	namespacedByGk map[schema.GroupKind]bool
}

func (p *resourceInfoProvider) IsNamespaced(gk schema.GroupKind) (bool, error) {
	return p.namespacedByGk[gk], nil
}

// RenderApplicationsFromBothBranches renders manifests for all supplied base
// and target Applications by streaming their local source directories to the
// Argo CD repo server via gRPC.
//
// baseBranch / targetBranch identify the local folders that hold each branch's
// checked-out source files (e.g. "base-branch/" or "target-branch/").
//
// prRepo is the URL of the pull-request repository (e.g.
// "https://github.com/org/repo.git"). Sources whose repoURL does not match
// this URL point at a different repository whose files are not checked out
// locally; those sources are rendered via the remote GenerateManifest RPC so
// the repo server fetches them itself.
//
// The return type is identical to extract.RenderApplicationsFromBothBranches
// so that callers can swap implementations with minimal changes.
func RenderApplicationsFromBothBranches(
	argocd *argocdPkg.ArgoCDInstallation,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	timeout uint64,
	maxConcurrency uint,
	baseApps []argoapplication.ArgoResource,
	targetApps []argoapplication.ArgoResource,
	prRepo string,
) ([]extract.ExtractedApp, []extract.ExtractedApp, time.Duration, error) {
	startTime := time.Now()

	branchFolderByType := map[git.BranchType]string{
		git.Base:   baseBranch.FolderName(),
		git.Target: targetBranch.FolderName(),
	}

	log.Info().Msgf("📌 Final number of Applications planned to be rendered via repo server: [Base: %d], [Target: %d]",
		len(baseApps), len(targetApps))

	if err := extract.VerifyNoApplicationSets(baseApps); err != nil {
		return nil, nil, time.Since(startTime), err
	}

	if err := extract.VerifyNoApplicationSets(targetApps); err != nil {
		return nil, nil, time.Since(startTime), err
	}

	namespacedScopedResources, err := argocd.K8sClient.GetListOfNamespacedScopedResources()
	if err != nil {
		return nil, nil, time.Since(startTime), fmt.Errorf("failed to get list of namespaced scoped resources: %w", err)
	}

	// Collect all unique repository URLs referenced by the Applications so that
	// FetchRepoCreds can enrich them with credentials from repo-creds templates.
	appRepoURLs := collectRepoURLs(baseApps, targetApps)

	// Fetch all repository credentials from the cluster once, upfront.
	// The repo server has no access to Kubernetes secrets - credentials must be
	// provided by the caller in every ManifestRequest. We mirror what the
	// ArgoCD app controller does in controller/state.go before calling the repo server.
	creds, err := FetchRepoCreds(context.Background(), argocd.K8sClient, argocd.Namespace, appRepoURLs)
	if err != nil {
		return nil, nil, time.Since(startTime), fmt.Errorf("failed to fetch repository credentials: %w", err)
	}

	// Create a single repo server client shared across all goroutines.
	// EnsurePortForward is idempotent and mutex-protected inside the client.
	repoClient := reposerver.NewClient(argocd.K8sClient, argocd.Namespace)
	defer repoClient.Cleanup()

	if err := repoClient.EnsurePortForward(); err != nil {
		return nil, nil, time.Since(startTime), fmt.Errorf("failed to set up port forward to repo server: %w", err)
	}

	allApps := append(baseApps, targetApps...)

	log.Info().Msgf("🤖 Rendering Applications via repo server (timeout in %d seconds)", timeout)

	// ── Worker pool ──────────────────────────────────────────────────────────

	type result struct {
		app extract.ExtractedApp
		err error
	}

	results := make(chan result, len(allApps))

	semSize := int(maxConcurrency)
	if semSize == 0 {
		semSize = len(allApps)
	}
	if semSize == 0 {
		semSize = 1
	}
	sem := make(chan struct{}, semSize)

	totalApps := len(allApps)
	var renderedApps atomic.Int32

	progressDone := make(chan bool)
	remainingTime := func() int {
		return max(0, int(timeout)-int(time.Since(startTime).Seconds()))
	}

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Info().Msgf("🤖 Rendered %d out of %d applications via repo server (timeout in %d seconds)",
					renderedApps.Load(), totalApps, remainingTime())
			case <-progressDone:
				return
			}
		}
	}()

	for _, app := range allApps {
		sem <- struct{}{}
		go func(app argoapplication.ArgoResource) {
			defer func() { <-sem }()

			if remainingTime() <= 0 {
				results <- result{err: fmt.Errorf("timeout reached before starting to render application: %s", app.GetLongName())}
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(remainingTime())*time.Second)
			defer cancel()

			manifests, err := renderApp(ctx, repoClient, app, branchFolderByType, namespacedScopedResources, creds, prRepo)
			if err != nil {
				results <- result{err: fmt.Errorf("failed to render app %s: %w", app.GetLongName(), err)}
				return
			}

			renderedApps.Add(1)
			results <- result{app: extract.CreateExtractedApp(app.Id, app.Name, app.FileName, manifests, app.Branch)}
		}(app)
	}

	// ── Collect results ──────────────────────────────────────────────────────

	extractedBaseApps := make([]extract.ExtractedApp, 0, len(baseApps))
	extractedTargetApps := make([]extract.ExtractedApp, 0, len(targetApps))
	var firstError error

	for range len(allApps) {
		r := <-results
		if r.err != nil {
			if firstError == nil {
				firstError = r.err
			}
			log.Error().Err(r.err).Msg("❌ Failed to render application via repo server:")
			continue
		}
		switch r.app.Branch {
		case git.Base:
			extractedBaseApps = append(extractedBaseApps, r.app)
		case git.Target:
			extractedTargetApps = append(extractedTargetApps, r.app)
		default:
			if firstError == nil {
				firstError = fmt.Errorf("unknown branch type: '%s'", r.app.Branch)
			}
		}
	}

	close(progressDone)

	if firstError != nil {
		return nil, nil, time.Since(startTime), firstError
	}

	duration := time.Since(startTime)
	log.Info().Msgf("🎉 Rendered all %d applications via repo server in %s",
		renderedApps.Load(), duration.Round(time.Second))
	log.Info().Msgf("🤖 Got %d resources from %s-branch and %d from %s-branch via repo server",
		len(extractedBaseApps), git.Base, len(extractedTargetApps), git.Target)

	return extractedBaseApps, extractedTargetApps, time.Since(startTime), nil
}

// renderApp packages a single application's source directory and streams it to
// the repo server, returning the post-processed manifests.
//
// For multi-content-source applications the repo server is called once per
// content source and the resulting manifests are merged before post-processing.
// Ref-only sources (used for $ref value-file paths) are always forwarded to
// every content-source call so that cross-source references keep working.
//
// ApplicationSet resources are handled by reading sources from
// spec.template.spec rather than spec.
//
// prRepo is the URL of the pull-request repository. Sources whose repoURL does
// not match prRepo are rendered via the remote GenerateManifest RPC instead of
// streaming local files (which would not exist for a foreign repository).
func renderApp(
	ctx context.Context,
	repoClient *reposerver.Client,
	app argoapplication.ArgoResource,
	branchFolderByType map[git.BranchType]string,
	namespacedScopedResources map[schema.GroupKind]bool,
	creds *RepoCreds,
	prRepo string,
) ([]unstructured.Unstructured, error) {
	branchFolder, ok := branchFolderByType[app.Branch]
	if !ok {
		return nil, fmt.Errorf("unknown branch type: %s", app.Branch)
	}

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	if err != nil {
		return nil, fmt.Errorf("failed to split sources: %w", err)
	}

	var allManifestStrings []string

	for i, contentSource := range contentSources {
		request, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSource, refSources, hasMultipleSources, branchFolder, creds, prRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to build manifest request for content source %d: %w", i, err)
		}

		// streamDir == "" signals that the primary source is a remote chart (e.g. an
		// external Helm registry). In that case we use the regular (non-file-streaming)
		// GenerateManifest RPC so that the repo server fetches the chart itself.
		useRemote := streamDir == ""

		log.Debug().
			Str("App", app.GetLongName()).
			Int("sourceIndex", i).
			Str("streamDir", streamDir).
			Bool("multiSource", request.HasMultipleSources).
			Bool("remoteChart", useRemote).
			Msg("Rendering application source via repo server")

		var manifestStrings []string
		if useRemote {
			manifestStrings, err = repoClient.GenerateManifestsRemote(ctx, request)
		} else {
			manifestStrings, err = repoClient.GenerateManifests(ctx, streamDir, request)
		}
		// Clean up the temp dir immediately after the RPC completes - don't defer
		// inside a loop, which would keep all temp dirs alive until renderApp returns.
		if cleanup != nil {
			cleanup()
		}
		if err != nil {
			return nil, fmt.Errorf("repo server returned error for content source %d: %w", i, err)
		}

		allManifestStrings = append(allManifestStrings, manifestStrings...)
	}

	if len(allManifestStrings) == 0 {
		log.Warn().Str("App", app.GetLongName()).Msg("⚠️ Repo server returned no manifests")
		return []unstructured.Unstructured{}, nil
	}

	// Parse JSON manifest strings into unstructured objects.
	manifests := make([]unstructured.Unstructured, 0, len(allManifestStrings))
	for i, raw := range allManifestStrings {
		var obj map[string]any
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest %d for %s: %w", i, app.GetLongName(), err)
		}
		if len(obj) == 0 {
			continue
		}
		apiVersion, _, _ := unstructured.NestedString(obj, "apiVersion")
		kind, _, _ := unstructured.NestedString(obj, "kind")
		name, _, _ := unstructured.NestedString(obj, "metadata", "name")
		if apiVersion == "" || kind == "" || name == "" {
			log.Debug().Str("App", app.GetLongName()).Msg("Skipping manifest with missing apiVersion, kind, or name")
			continue
		}
		manifests = append(manifests, unstructured.Unstructured{Object: obj})
	}

	// ── Post-processing (mirrors extract.getManifestsFromApp) ────────────────

	// Remove null metadata fields that some repo server responses include.
	// For example, the nginx-ingress chart returns resources with
	// "metadata": {"annotations": null} which yaml.Marshal would render as
	// "annotations: null" - not present in the old cluster-reconciliation path.
	removeNullMetadataFields(manifests)

	replaceAppIDInManifests(manifests, app.Id, app.Name)

	// Apply ignoreDifferences rules using the shared implementation in pkg/extract.
	extract.ApplyIgnoreDifferences(manifests, app)

	if err := removeArgoCDTrackingID(manifests); err != nil {
		return nil, fmt.Errorf("failed to remove Argo CD tracking ID: %w", err)
	}

	destNamespace, _, _ := unstructured.NestedString(app.Yaml.Object, "spec", "destination", "namespace")
	manifests, err = normalizeNamespaces(manifests, destNamespace, namespacedScopedResources, app.GetLongName())
	if err != nil {
		return nil, err
	}

	// Filter out Helm hook resources (reuse the exported helper from extract).
	filtered := make([]unstructured.Unstructured, 0, len(manifests))
	for _, m := range manifests {
		if extract.HelmHookFilter(m) {
			filtered = append(filtered, m)
		}
	}

	return filtered, nil
}

// collectRepoURLs extracts all unique repository URLs referenced by the given
// Application resources (from both base and target branches). It inspects
// spec.source.repoURL and spec.sources[*].repoURL. The result is deduplicated
// by the normalised URL form.
//
// NOTE: by the time apps reach the repo-server-extract path, ApplicationSets
// have already been converted to Applications, so we only need to look under
// spec (not spec.template.spec).
func collectRepoURLs(appLists ...[]argoapplication.ArgoResource) []string {
	seen := make(map[string]bool)
	var urls []string

	for _, apps := range appLists {
		for _, app := range apps {
			obj := app.Yaml.Object

			// spec.source.repoURL (single-source apps)
			if repoURL, found, _ := unstructured.NestedString(obj, "spec", "source", "repoURL"); found && repoURL != "" {
				key := normalizeRepoURL(repoURL)
				if !seen[key] {
					seen[key] = true
					urls = append(urls, repoURL)
				}
			}

			// spec.sources[*].repoURL (multi-source apps)
			if sourcesRaw, found, _ := unstructured.NestedSlice(obj, "spec", "sources"); found {
				for _, srcRaw := range sourcesRaw {
					srcMap, ok := srcRaw.(map[string]any)
					if !ok {
						continue
					}
					repoURL, _ := srcMap["repoURL"].(string)
					if repoURL == "" {
						continue
					}
					key := normalizeRepoURL(repoURL)
					if !seen[key] {
						seen[key] = true
						urls = append(urls, repoURL)
					}
				}
			}
		}
	}

	return urls
}

// splitSources parses the application's spec.sources / spec.source and splits
// them into content sources (sources that produce manifests) and ref-only
// sources (sources whose sole purpose is to provide files referenced by $ref/…
// value-file paths in another source).
//
// The returned hasMultipleSources flag reflects whether the original
// Application YAML used spec.sources (true) or spec.source (false), which is
// forwarded verbatim in every ManifestRequest so the repo server knows the
// application's topology.
func splitSources(app argoapplication.ArgoResource) (
	contentSources []v1alpha1.ApplicationSource,
	refSources []v1alpha1.ApplicationSource,
	hasMultipleSources bool,
	err error,
) {
	obj := app.Yaml.Object

	var specPath []string
	switch app.Kind {
	case argoapplication.ApplicationSet:
		specPath = []string{"spec", "template", "spec"}
	default:
		specPath = []string{"spec"}
	}

	var appSources v1alpha1.ApplicationSources

	if sourcesRaw, found, _ := unstructured.NestedSlice(obj, append(specPath, "sources")...); found && len(sourcesRaw) > 0 {
		hasMultipleSources = true
		sourcesBytes, marshalErr := json.Marshal(sourcesRaw)
		if marshalErr != nil {
			return nil, nil, false, fmt.Errorf("failed to marshal spec.sources: %w", marshalErr)
		}
		if unmarshalErr := json.Unmarshal(sourcesBytes, &appSources); unmarshalErr != nil {
			return nil, nil, false, fmt.Errorf("failed to unmarshal ApplicationSources: %w", unmarshalErr)
		}
	} else if sourceRaw, found, _ := unstructured.NestedMap(obj, append(specPath, "source")...); found {
		sourceBytes, marshalErr := json.Marshal(sourceRaw)
		if marshalErr != nil {
			return nil, nil, false, fmt.Errorf("failed to marshal spec.source: %w", marshalErr)
		}
		var singleSource v1alpha1.ApplicationSource
		if unmarshalErr := json.Unmarshal(sourceBytes, &singleSource); unmarshalErr != nil {
			return nil, nil, false, fmt.Errorf("failed to unmarshal ApplicationSource: %w", unmarshalErr)
		}
		appSources = v1alpha1.ApplicationSources{singleSource}
	} else {
		return nil, nil, false, fmt.Errorf("application %s has neither spec.source nor spec.sources", app.GetLongName())
	}

	for _, s := range appSources {
		if s.Ref != "" && s.Path == "" {
			refSources = append(refSources, s)
		} else {
			contentSources = append(contentSources, s)
		}
	}
	if len(contentSources) == 0 {
		return nil, nil, false, fmt.Errorf("application %s has no content source (all sources are ref-only)", app.GetLongName())
	}

	return contentSources, refSources, hasMultipleSources, nil
}

// buildManifestRequestForSource constructs the ManifestRequest and the
// directory to stream to the repo server for a single content source.
//
// refSources contains all ref-only sources from the same application; they are
// forwarded so the repo server (or local path rewriting) can resolve $ref/…
// value-file paths. hasMultipleSources reflects the original application's
// spec topology and is forwarded verbatim in the request.
//
// For multi-source apps with $ref value files the ref source directories are
// placed under <tempDir>/.refs/<refName>/ and the corresponding $ref/… paths
// in the ManifestRequest are rewritten to relative paths, following the same
// approach used by other tools that integrate with the repo server.
//
// prRepo is the URL of the pull-request repository. When the primary source's
// repoURL does not match prRepo the source files are not present locally;
// in that case the function returns streamDir="" so the caller uses the remote
// GenerateManifest RPC and the repo server fetches the content itself.
//
// cleanup must be called by the caller when the stream directory is no longer
// needed.
func buildManifestRequestForSource(
	app argoapplication.ArgoResource,
	primarySource v1alpha1.ApplicationSource,
	refSources []v1alpha1.ApplicationSource,
	hasMultipleSources bool,
	branchFolder string,
	creds *RepoCreds,
	prRepo string,
) (request *repoapiclient.ManifestRequest, streamDir string, cleanup func(), err error) {
	obj := app.Yaml.Object

	var specPath []string
	switch app.Kind {
	case argoapplication.ApplicationSet:
		specPath = []string{"spec", "template", "spec"}
	default:
		specPath = []string{"spec"}
	}

	namespace, _, _ := unstructured.NestedString(obj, append(specPath, "destination", "namespace")...)

	// ── Fast path: no ref sources → stream the whole branch folder ────────────
	// The repo server resolves ApplicationSource.Path relative to the stream
	// root (workDir), so streaming the entire branch folder and setting Path
	// correctly is sufficient. This also handles kustomize overlays that
	// reference sibling directories (e.g. ../../base) which would be missing
	// if we only copied the leaf path into a temp dir.
	//
	// Special case: if the primary source has a Chart field (external Helm
	// registry chart) there are no local files to stream. We signal this by
	// returning an empty streamDir; the caller will use the regular
	// (non-file-streaming) GenerateManifest RPC instead.
	if len(refSources) == 0 {
		if primarySource.Chart != "" {
			// Remote Helm chart – no local files to stream.
			request = &repoapiclient.ManifestRequest{
				Repo:               creds.GetRepo(primarySource.RepoURL),
				Repos:              creds.HelmRepos(&primarySource),
				HelmRepoCreds:      creds.HelmRepoCreds(&primarySource),
				Revision:           primarySource.TargetRevision,
				AppName:            app.Id,
				Namespace:          namespace,
				ApplicationSource:  &primarySource,
				HasMultipleSources: hasMultipleSources,
			}
			return request, "", nil, nil
		}
		// Cross-repo source: the source's repoURL points at a different
		// repository than the PR repo. Those files are not checked out
		// locally, so we cannot stream them. Fall back to the remote
		// GenerateManifest RPC and let the repo server fetch them itself.
		if prRepo != "" && !repoURLContains(primarySource.RepoURL, prRepo) {
			log.Debug().
				Str("App", app.GetLongName()).
				Str("sourceRepoURL", primarySource.RepoURL).
				Str("prRepo", prRepo).
				Msg("Source repoURL does not match PR repo - using remote RPC")
			request = &repoapiclient.ManifestRequest{
				Repo:               creds.GetRepo(primarySource.RepoURL),
				Repos:              creds.HelmRepos(&primarySource),
				HelmRepoCreds:      creds.HelmRepoCreds(&primarySource),
				Revision:           primarySource.TargetRevision,
				AppName:            app.Id,
				Namespace:          namespace,
				ApplicationSource:  &primarySource,
				HasMultipleSources: hasMultipleSources,
			}
			return request, "", nil, nil
		}
		request = &repoapiclient.ManifestRequest{
			Repo:               creds.GetRepo(primarySource.RepoURL),
			Repos:              creds.HelmRepos(&primarySource),
			HelmRepoCreds:      creds.HelmRepoCreds(&primarySource),
			Revision:           primarySource.TargetRevision,
			AppName:            app.Id,
			Namespace:          namespace,
			ApplicationSource:  &primarySource,
			HasMultipleSources: hasMultipleSources,
		}
		return request, branchFolder, nil, nil
	}
	// ── Slow path: ref sources present ───────────────────────────────────────
	//
	// Special case: external Helm chart primary source WITH ref sources.
	// Example pattern (cluster-common-charts ApplicationSet):
	//   sources:
	//     - repoURL: https://github.com/…  ref: local          ← ref source
	//     - chart: cert-manager  repoURL: https://charts.jetstack.io  ← primary
	//       helm.valueFiles: [$local/path/to/values.yaml]
	//
	// We cannot stream local files for a chart: source via GenerateManifestWithFiles
	// because the repo server tries to read Chart.yaml from the tarball root and
	// fails (the chart lives in an external registry, not in the tarball).
	// Instead, use the unary GenerateManifest RPC and populate RefSources so the
	// repo server fetches the ref content from its own git cache. The $ref/…
	// value file paths are left unchanged (no rewriting needed).
	if primarySource.Chart != "" {
		refSourcesMap := make(map[string]*v1alpha1.RefTarget, len(refSources))
		for _, ref := range refSources {
			refSourcesMap["$"+ref.Ref] = &v1alpha1.RefTarget{
				Repo:           *creds.GetRepo(ref.RepoURL),
				TargetRevision: ref.TargetRevision,
			}
		}
		request = &repoapiclient.ManifestRequest{
			Repo:               creds.GetRepo(primarySource.RepoURL),
			Repos:              creds.HelmRepos(&primarySource),
			HelmRepoCreds:      creds.HelmRepoCreds(&primarySource),
			Revision:           primarySource.TargetRevision,
			AppName:            app.Id,
			Namespace:          namespace,
			ApplicationSource:  &primarySource,
			HasMultipleSources: hasMultipleSources,
			RefSources:         refSourcesMap,
		}
		return request, "", nil, nil
	}

	// ── Slow path: ref sources present - cross-repo primary source ────────────
	// When the primary source lives in a different repository from the PR, we
	// cannot stream its files locally. Use the remote RPC and let the repo
	// server fetch both the primary content and the ref sources from their
	// respective git caches. Value-file $ref/… paths are left unrewritten.
	if prRepo != "" && !repoURLContains(primarySource.RepoURL, prRepo) {
		log.Debug().
			Str("App", app.GetLongName()).
			Str("sourceRepoURL", primarySource.RepoURL).
			Str("prRepo", prRepo).
			Msg("Source repoURL does not match PR repo (slow path) - using remote RPC")
		refSourcesMap := make(map[string]*v1alpha1.RefTarget, len(refSources))
		for _, ref := range refSources {
			refSourcesMap["$"+ref.Ref] = &v1alpha1.RefTarget{
				Repo:           *creds.GetRepo(ref.RepoURL),
				TargetRevision: ref.TargetRevision,
			}
		}
		request = &repoapiclient.ManifestRequest{
			Repo:               creds.GetRepo(primarySource.RepoURL),
			Repos:              creds.HelmRepos(&primarySource),
			HelmRepoCreds:      creds.HelmRepoCreds(&primarySource),
			Revision:           primarySource.TargetRevision,
			AppName:            app.Id,
			Namespace:          namespace,
			ApplicationSource:  &primarySource,
			HasMultipleSources: hasMultipleSources,
			RefSources:         refSourcesMap,
		}
		return request, "", nil, nil
	}

	//   <tempDir>/<primarySource.Path>/  ← content source files
	//   <tempDir>/.refs/<refName>/       ← files for each ref source
	tempDir, err := os.MkdirTemp("", "argocd-diff-preview-*")
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	cleanup = func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			log.Warn().Err(removeErr).Str("dir", tempDir).Msg("Failed to remove temp dir")
		}
	}

	// Copy the content source directory into the temp tree.
	srcContentDir := filepath.Join(branchFolder, primarySource.Path)
	dstContentDir := filepath.Join(tempDir, primarySource.Path)
	if err := copyDir(srcContentDir, dstContentDir); err != nil {
		cleanup()
		return nil, "", nil, fmt.Errorf("failed to copy content source dir %q: %w", srcContentDir, err)
	}

	// Copy each ref source and build a ref name → local-path mapping.
	refDirs := make(map[string]string) // ref name → absolute path inside tempDir
	for _, ref := range refSources {
		refDir := filepath.Join(tempDir, ".refs", ref.Ref)
		srcRefDir := filepath.Join(branchFolder, ref.Path)
		if ref.Path == "" {
			// Ref-only source with no path points at the repo root.
			srcRefDir = branchFolder
		}
		if err := copyDir(srcRefDir, refDir); err != nil {
			cleanup()
			return nil, "", nil, fmt.Errorf("failed to copy ref source %q: %w", ref.Ref, err)
		}
		refDirs[ref.Ref] = refDir
	}

	// ── Rewrite $ref/… paths in Helm ValueFiles to relative paths ─────────────
	rewrittenSource := primarySource
	if rewrittenSource.Helm != nil {
		rewritten := make([]string, len(rewrittenSource.Helm.ValueFiles))
		copy(rewritten, rewrittenSource.Helm.ValueFiles)
		appDirAbs := filepath.Join(tempDir, primarySource.Path)
		for i, vf := range rewritten {
			if !strings.HasPrefix(vf, "$") {
				continue
			}
			refName, refPath, ok := splitRefPath(vf)
			if !ok {
				continue
			}
			refLocalDir, known := refDirs[refName]
			if !known {
				cleanup()
				return nil, "", nil, fmt.Errorf("value file %q references unknown ref %q in app %s", vf, refName, app.GetLongName())
			}
			absTarget := filepath.Join(refLocalDir, refPath)
			relPath, err := filepath.Rel(appDirAbs, absTarget)
			if err != nil {
				cleanup()
				return nil, "", nil, fmt.Errorf("failed to compute relative path for ref value file: %w", err)
			}
			rewritten[i] = relPath
		}
		helmCopy := *rewrittenSource.Helm
		helmCopy.ValueFiles = rewritten
		rewrittenSource.Helm = &helmCopy
	}

	request = &repoapiclient.ManifestRequest{
		Repo:               creds.GetRepo(rewrittenSource.RepoURL),
		Repos:              creds.HelmRepos(&rewrittenSource),
		HelmRepoCreds:      creds.HelmRepoCreds(&rewrittenSource),
		Revision:           rewrittenSource.TargetRevision,
		AppName:            app.Id,
		Namespace:          namespace,
		ApplicationSource:  &rewrittenSource,
		HasMultipleSources: hasMultipleSources,
	}
	return request, tempDir, cleanup, nil
}

// splitRefPath splits a $refName/path/to/file value-file string into
// (refName, path/to/file, true), or returns ("", "", false) if the string
// doesn't match the expected pattern. A ref with no sub-path (e.g. "$cfg"
// with no trailing "/") is considered invalid and returns false.
func splitRefPath(valueFile string) (refName, path string, ok bool) {
	rest, hasPrefix := strings.CutPrefix(valueFile, "$")
	if !hasPrefix {
		return
	}
	refName, path, ok = strings.Cut(rest, "/")
	if !ok {
		// No sub-path after the ref name - this is a malformed value file
		// (e.g. "$cfg" instead of "$cfg/values.yaml"). A path pointing at a
		// ref directory root is not a valid file reference, so treat it as
		// unrecognised.
		return "", "", false
	}
	return refName, path, true
}

// copyDir recursively copies src into dst, creating dst if needed.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(srcPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, srcPath)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}
		return copyFile(srcPath, dstPath)
	})
}

// copyFile copies a single file from src to dst, creating parent directories.
func copyFile(src, dst string) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := r.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := w.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(w, r)
	return err
}

// replaceAppIDInManifests replaces the app ID with the app name in all
// annotation values, mirroring the behaviour in pkg/extract.
func replaceAppIDInManifests(manifests []unstructured.Unstructured, appID, appName string) {
	for i := range manifests {
		annotations := manifests[i].GetAnnotations()
		if annotations == nil {
			continue
		}
		for k, v := range annotations {
			if v == appID {
				annotations[k] = appName
			}
		}
		manifests[i].SetAnnotations(annotations)
	}
}

// removeNullMetadataFields removes metadata sub-fields (annotations, labels,
// finalizers, managedFields, ownerReferences) that the repo server serialises
// as JSON null. yaml.Marshal preserves null values as "field: null" which
// doesn't match the output from the old cluster-reconciliation path.
func removeNullMetadataFields(manifests []unstructured.Unstructured) {
	nullableFields := []string{"annotations", "labels", "finalizers", "managedFields", "ownerReferences"}
	for i := range manifests {
		meta, ok, _ := unstructured.NestedMap(manifests[i].Object, "metadata")
		if !ok {
			continue
		}
		for _, field := range nullableFields {
			if v, exists := meta[field]; exists && v == nil {
				unstructured.RemoveNestedField(manifests[i].Object, "metadata", field)
			}
		}
	}
}

// removeArgoCDTrackingID removes the "argocd.argoproj.io/tracking-id" annotation
// from all manifests.
func removeArgoCDTrackingID(manifests []unstructured.Unstructured) error {
	for i := range manifests {
		annotations := manifests[i].GetAnnotations()
		if annotations == nil {
			continue
		}
		for key := range annotations {
			if key == common.AnnotationKeyAppInstance {
				delete(annotations, key)
			}
		}
		if len(annotations) == 0 {
			// Remove the key entirely so we don't emit "annotations: null" in the YAML/JSON output.
			unstructured.RemoveNestedField(manifests[i].Object, "metadata", "annotations")
		} else {
			manifests[i].SetAnnotations(annotations)
		}
	}
	return nil
}

// normalizeNamespaces uses Argo CD's DeduplicateTargetObjects to normalise
// namespaces on manifests, mirroring the same function in pkg/extract.
func normalizeNamespaces(
	manifests []unstructured.Unstructured,
	destNamespace string,
	namespacedResources map[schema.GroupKind]bool,
	appName string,
) ([]unstructured.Unstructured, error) {
	if destNamespace == "" {
		return manifests, nil
	}

	ptrManifests := make([]*unstructured.Unstructured, len(manifests))
	for i := range manifests {
		ptrManifests[i] = &manifests[i]
	}

	provider := &resourceInfoProvider{namespacedByGk: namespacedResources}
	deduped, conditions, err := controller.DeduplicateTargetObjects(destNamespace, ptrManifests, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to normalise namespaces: %w", err)
	}

	for _, cond := range conditions {
		log.Warn().Str("App", appName).Msgf("Duplicate resource warning: %s", cond.Message)
	}

	result := make([]unstructured.Unstructured, len(deduped))
	for i, ptr := range deduped {
		result[i] = *ptr
	}
	return result, nil
}
