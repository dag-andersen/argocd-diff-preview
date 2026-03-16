// Package reposerverextract - app-of-apps expansion.
//
// This file contains all logic for recursively discovering and rendering child
// Applications that appear in a parent application's rendered manifests
// (the "app-of-apps" pattern).  It is intentionally isolated so the feature
// can be removed cleanly in the future if it is no longer needed.
//
// The feature is only active when --traverse-app-of-apps is set.
package reposerverextract

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	argocdPkg "github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/reposerver"
)

// maxAppOfAppsDepth is the maximum recursion depth allowed when following
// child Applications discovered in rendered manifests (app-of-apps pattern).
// A depth of 0 means the seed apps themselves; depth 1 means their children,
// and so on. This prevents infinite loops in circular app-of-apps graphs.
const maxAppOfAppsDepth = 10

// workItem is a single unit of rendering work, carrying the app to render and
// how deep in the app-of-apps hierarchy it sits.
type workItem struct {
	app   argoapplication.ArgoResource
	depth int
}

// renderResult captures a single rendered application together with any child
// Application resources that were discovered in its manifests.
type renderResult struct {
	// extracted is the ExtractedApp for the rendered application. Its
	// Manifests slice already has Application resources stripped out.
	extracted extract.ExtractedApp

	// childApps are the ArgoResource values built from Application manifests
	// that were discovered inside the rendered output. They have been patched
	// and are ready to be enqueued for rendering.
	childApps []argoapplication.ArgoResource

	// depth is the depth of the app that produced this result, used to decide
	// whether to enqueue its children.
	depth int

	err error
}

// visitedKey returns a unique string key for an (appID, branch) pair, used to
// track which applications have already been rendered during app-of-apps
// expansion.
func visitedKey(id string, branch git.BranchType) string {
	return id + "|" + string(branch)
}

// RenderApplicationsFromBothBranchesWithAppOfApps is like
// RenderApplicationsFromBothBranches but additionally discovers and renders
// child Applications found in rendered manifests (the app-of-apps pattern).
//
// When a rendered app's manifests contain argoproj.io/Application resources,
// those children are patched and enqueued for rendering recursively — up to
// maxAppOfAppsDepth levels deep. Child Application YAML manifests are excluded
// from the parent's diff output; each child gets its own ExtractedApp entry.
//
// A visited set prevents re-rendering the same app twice, guarding against
// cycles (A→B→A) and diamond dependencies (A→C, B→C).
//
// Child apps are filtered by Selector, FilesChanged (via watch-pattern annotations),
// IgnoreInvalidWatchPattern, and WatchIfNoWatchPatternFound — the same as top-level
// apps. FileRegex is excluded because it filters by physical file path, and child
// apps have no file path (their FileName is a breadcrumb like "parent: <name>").
func RenderApplicationsFromBothBranchesWithAppOfApps(
	argocd *argocdPkg.ArgoCDInstallation,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	timeout uint64,
	maxConcurrency uint,
	baseApps []argoapplication.ArgoResource,
	targetApps []argoapplication.ArgoResource,
	prRepo string,
	appSelectionOptions argoapplication.ApplicationSelectionOptions,
	tempFolder string,
) ([]extract.ExtractedApp, []extract.ExtractedApp, time.Duration, error) {
	startTime := time.Now()

	branchFolderByType := map[git.BranchType]string{
		git.Base:   baseBranch.FolderName(),
		git.Target: targetBranch.FolderName(),
	}

	branchByType := map[git.BranchType]*git.Branch{
		git.Base:   baseBranch,
		git.Target: targetBranch,
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

	log.Info().Msgf("🤖 Rendering Applications via repo server with app-of-apps traversal (timeout in %d seconds)", timeout)

	remainingTime := func() int {
		return max(0, int(timeout)-int(time.Since(startTime).Seconds()))
	}

	// ── Single-pool expansion ────────────────────────────────────────────────
	// All apps (seed + discovered children) go through the same worker pool.
	// A pending counter tracks how many items are in-flight or queued; when it
	// reaches zero every goroutine has finished and all results are collected.
	// A visited set (mutex-protected) prevents re-rendering the same app twice.

	var (
		extractedBaseApps   []extract.ExtractedApp
		extractedTargetApps []extract.ExtractedApp
		renderedApps        atomic.Int32
		pending             atomic.Int32
		firstError          error
		visitedMu           sync.Mutex
	)

	visited := make(map[string]bool)

	semSize := int(maxConcurrency)
	if semSize == 0 {
		semSize = 1
	}
	sem := make(chan struct{}, semSize)

	// work is a buffered channel; workers send newly discovered children back
	// onto it. We size it generously so senders are never blocked.
	work := make(chan workItem, 1024)
	results := make(chan renderResult, 1024)

	// enqueue increments pending before sending so the counter is always >=
	// actual in-flight count.
	enqueue := func(app argoapplication.ArgoResource, depth int) {
		pending.Add(1)
		work <- workItem{app: app, depth: depth}
	}

	// Seed the queue with the initial base + target apps (depth 0).
	visitedMu.Lock()
	for _, app := range append(baseApps, targetApps...) {
		visited[visitedKey(app.Id, app.Branch)] = true
		enqueue(app, 0)
	}
	visitedMu.Unlock()

	progressDone := make(chan bool)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				log.Info().Msgf("🤖 Rendered %d applications via repo server so far (timeout in %d seconds)",
					renderedApps.Load(), remainingTime())
			case <-progressDone:
				return
			}
		}
	}()

	// Single collector goroutine: reads results, collects extracted apps, and
	// enqueues newly discovered children back onto the work channel.
	collectorDone := make(chan struct{})
	go func() {
		defer close(collectorDone)
		for r := range results {
			if r.err != nil {
				if firstError == nil {
					firstError = r.err
				}
				log.Error().Err(r.err).Msg("❌ Failed to render application via repo server:")
				pending.Add(-1)
				continue
			}

			switch r.extracted.Branch {
			case git.Base:
				extractedBaseApps = append(extractedBaseApps, r.extracted)
			case git.Target:
				extractedTargetApps = append(extractedTargetApps, r.extracted)
			default:
				if firstError == nil {
					firstError = fmt.Errorf("unknown branch type: '%s'", r.extracted.Branch)
				}
			}

			// Enqueue children that haven't been seen yet and pass the selection filter.
			// Child apps are filtered by Selector, FilesChanged (via watch-pattern annotations),
			// IgnoreInvalidWatchPattern, and WatchIfNoWatchPatternFound — exactly as top-level apps are.
			// FilesChanged works correctly here: the PR diff is the same regardless of whether an
			// app was discovered from a file or from a parent's rendered output; the watch pattern
			// on the child app is what determines whether it is affected.
			//
			// FileRegex is intentionally excluded because it filters by the physical filename of
			// the Application YAML file. Child apps don't come from a file; their FileName is
			// "parent: <name>" (a breadcrumb), which would give false matches against any regex.
			if r.depth < maxAppOfAppsDepth {
				childSelectionOptions := argoapplication.ApplicationSelectionOptions{
					Selector:                   appSelectionOptions.Selector,
					FilesChanged:               appSelectionOptions.FilesChanged,
					IgnoreInvalidWatchPattern:  appSelectionOptions.IgnoreInvalidWatchPattern,
					WatchIfNoWatchPatternFound: appSelectionOptions.WatchIfNoWatchPatternFound,
					// FileRegex intentionally omitted: child apps have no real file path
				}
				selection := argoapplication.ApplicationSelection(r.childApps, childSelectionOptions)
				for _, skipped := range selection.SkippedApps {
					log.Debug().Str("App", skipped.GetLongName()).Msg("Skipping child Application excluded by ApplicationSelectionOptions")
				}
				visitedMu.Lock()
				for _, child := range selection.SelectedApps {
					key := visitedKey(child.Id, child.Branch)
					if visited[key] {
						log.Debug().Str("App", child.GetLongName()).Msg("Skipping already-visited child Application")
						continue
					}
					visited[key] = true
					enqueue(child, r.depth+1)
				}
				visitedMu.Unlock()
			} else if len(r.childApps) > 0 {
				log.Warn().Msgf("⚠️ App-of-apps depth limit (%d) reached; not enqueuing %d child(ren) of %s",
					maxAppOfAppsDepth, len(r.childApps), r.extracted.Name)
			}

			pending.Add(-1)

			// When all pending work is done, close the work channel so workers exit.
			if pending.Load() == 0 {
				close(work)
			}
		}
	}()

	// Workers: pull from work channel, render, send result.
	var wg sync.WaitGroup
	for item := range work {
		sem <- struct{}{}
		wg.Add(1)
		go func(item workItem) {
			defer wg.Done()
			defer func() { <-sem }()

			if remainingTime() <= 0 {
				results <- renderResult{err: fmt.Errorf("timeout reached before starting to render application: %s", item.app.GetLongName())}
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(remainingTime())*time.Second)
			defer cancel()

			manifests, childApps, err := renderAppWithChildDiscovery(ctx, repoClient, argocd, item.app, branchFolderByType, branchByType, namespacedScopedResources, creds, prRepo, argocd.Namespace, tempFolder, item.depth)
			if err != nil {
				results <- renderResult{err: fmt.Errorf("failed to render app %s: %w", item.app.GetLongName(), err)}
				return
			}

			renderedApps.Add(1)
			results <- renderResult{
				extracted: extract.CreateExtractedApp(item.app.Id, item.app.Name, item.app.FileName, manifests, item.app.Branch),
				childApps: childApps,
				depth:     item.depth,
			}
		}(item)
	}

	// All work items have been dequeued. Wait for in-flight workers to finish
	// sending their results before closing the results channel.
	wg.Wait()
	close(results)
	<-collectorDone

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

// renderAppWithChildDiscovery renders a single application and separates the
// resulting manifests into two groups:
//
//  1. Regular Kubernetes manifests (returned as the first value) – these are
//     included in the parent application's diff output.
//
//  2. Child Application resources (returned as the second value) – these are
//     patched and queued for recursive rendering. They are excluded from the
//     parent's manifest list so that the diff only shows the actual cluster
//     resources they produce, not the Application objects themselves.
//
// ApplicationSet resources found in rendered manifests are expanded into their
// generated Applications via argocd.AppsetGenerateWithRetry, then treated the
// same as directly-discovered child Applications.
func renderAppWithChildDiscovery(
	ctx context.Context,
	repoClient *reposerver.Client,
	argocd *argocdPkg.ArgoCDInstallation,
	app argoapplication.ArgoResource,
	branchFolderByType map[git.BranchType]string,
	branchByType map[git.BranchType]*git.Branch,
	namespacedScopedResources map[schema.GroupKind]bool,
	creds *RepoCreds,
	prRepo string,
	argocdNamespace string,
	tempFolder string,
	depth int,
) ([]unstructured.Unstructured, []argoapplication.ArgoResource, error) {
	allManifests, err := renderApp(ctx, repoClient, app, branchFolderByType, namespacedScopedResources, creds, prRepo)
	if err != nil {
		return nil, nil, err
	}

	// ── Separate regular manifests from child Application/ApplicationSet resources ──────────
	var regularManifests []unstructured.Unstructured
	var childApps []argoapplication.ArgoResource

	for _, m := range allManifests {
		if !strings.HasPrefix(m.GetAPIVersion(), "argoproj.io/") {
			regularManifests = append(regularManifests, m)
			continue
		}

		switch m.GetKind() {
		case "Application":
			name := m.GetName()
			if name == "" {
				log.Warn().Str("parentApp", app.GetLongName()).Msg("⚠️ Discovered child Application has no name; skipping")
				continue
			}
			fileName := fmt.Sprintf("parent: %s", app.Name)
			resource := argoapplication.NewArgoResource(&m, argoapplication.Application, name, name, fileName, app.Branch)
			child, err := argoapplication.PatchApplication(argocdNamespace, *resource, branchByType[app.Branch], prRepo, nil)
			if err != nil {
				log.Warn().Err(err).
					Str("parentApp", app.GetLongName()).
					Str("childName", name).
					Msg("⚠️ Could not patch child Application; skipping")
				continue
			}
			childApps = append(childApps, *child)
			log.Debug().
				Str("parentApp", app.GetLongName()).
				Str("childApp", child.GetLongName()).
				Msg("Discovered child Application via app-of-apps pattern")

		case "ApplicationSet":
			// Expand the ApplicationSet into its generated Applications using the
			// ArgoCD API/CLI, then treat each generated app as a child to enqueue.
			appSetName := m.GetName()
			log.Info().
				Str("parentApp", app.GetLongName()).
				Str("appSet", appSetName).
				Msgf("🔍 Discovered child ApplicationSet in rendered manifests; expanding to Applications")

			appSetTempFolder := fmt.Sprintf("%s/appsets/depth-%d", tempFolder, depth)
			branch := branchByType[app.Branch]

			// Patch the ApplicationSet the same way top-level ApplicationSets are patched
			// before being sent to the API. This strips spec.template.metadata.namespace
			// (e.g. "argocd") which ArgoCD's /api/v1/applicationsets/generate endpoint rejects.
			appSetResource := argoapplication.NewArgoResource(&m, argoapplication.ApplicationSet, appSetName, appSetName, app.FileName, app.Branch)
			patchedAppSet, err := argoapplication.PatchApplication(argocdNamespace, *appSetResource, branch, prRepo, nil)
			if err != nil {
				log.Warn().Err(err).
					Str("parentApp", app.GetLongName()).
					Str("appSet", appSetName).
					Msg("⚠️ Failed to patch child ApplicationSet before expansion; skipping")
				continue
			}

			generatedManifests, err := argocd.AppsetGenerateWithRetry(patchedAppSet.Yaml, appSetTempFolder, 5)
			if err != nil {
				log.Warn().Err(err).
					Str("parentApp", app.GetLongName()).
					Str("appSet", appSetName).
					Msg("⚠️ Could not expand child ApplicationSet; skipping")
				continue
			}

			breadcrumb := fmt.Sprintf("parent: %s (appset: %s)", app.Name, appSetName)
			for _, genDoc := range generatedManifests {
				if genDoc.GetKind() != "Application" {
					log.Warn().
						Str("appSet", appSetName).
						Str("kind", genDoc.GetKind()).
						Msg("⚠️ ApplicationSet generated unexpected non-Application resource; skipping")
					continue
				}
				name := genDoc.GetName()
				if name == "" {
					log.Warn().Str("appSet", appSetName).Msg("⚠️ ApplicationSet-generated Application has no name; skipping")
					continue
				}
				resource := argoapplication.NewArgoResource(&genDoc, argoapplication.Application, name, name, breadcrumb, app.Branch)
				child, err := argoapplication.PatchApplication(argocdNamespace, *resource, branch, prRepo, nil)
				if err != nil {
					log.Warn().Err(err).
						Str("parentApp", app.GetLongName()).
						Str("appSet", appSetName).
						Msg("⚠️ Could not patch ApplicationSet-generated Application; skipping")
					continue
				}
				childApps = append(childApps, *child)
				log.Debug().
					Str("parentApp", app.GetLongName()).
					Str("appSet", appSetName).
					Str("childApp", child.GetLongName()).
					Msg("Discovered child Application via ApplicationSet expansion")
			}

		default:
			regularManifests = append(regularManifests, m)
		}
	}

	if len(childApps) > 0 {
		log.Info().
			Str("parentApp", app.GetLongName()).
			Msgf("🔍 Discovered %d child Application(s) in rendered manifests", len(childApps))
	}

	return regularManifests, childApps, nil
}
