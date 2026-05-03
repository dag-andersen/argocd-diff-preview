package argoapplication

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
)

func ConvertAppSetsToAppsInBothBranches(
	argocd *argocd.ArgoCDInstallation,
	baseApps *ArgoSelection,
	targetApps *ArgoSelection,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	repo string,
	tempFolder string,
	redirectRevisions []string,
	debug bool,
	failOnDuplicateGeneratedApplications bool,
	appSelectionOptions ApplicationSelectionOptions,
) (*ArgoSelection, *ArgoSelection, time.Duration, error) {
	startTime := time.Now()

	log.Info().Msg("🤖 Converting ApplicationSets to Applications for both branches")

	baseTempFolder := fmt.Sprintf("%s/%s", tempFolder, git.Base)
	baseApps, err := processAppSets(
		argocd,
		baseApps,
		baseBranch,
		baseTempFolder,
		debug,
		failOnDuplicateGeneratedApplications,
		appSelectionOptions,
		repo,
		redirectRevisions,
	)

	if err != nil {
		log.Error().Str("branch", baseBranch.Name).Msg("❌ Failed to generate base apps")
		return nil, nil, time.Since(startTime), err
	}

	targetTempFolder := fmt.Sprintf("%s/%s", tempFolder, git.Target)
	targetApps, err = processAppSets(
		argocd,
		targetApps,
		targetBranch,
		targetTempFolder,
		debug,
		failOnDuplicateGeneratedApplications,
		appSelectionOptions,
		repo,
		redirectRevisions,
	)
	if err != nil {
		log.Error().Str("branch", targetBranch.Name).Msg("❌ Failed to generate target apps")
		return nil, nil, time.Since(startTime), err
	}

	log.Debug().Msgf("Converted ApplicationSets to Applications in %s", time.Since(startTime).Round(time.Second))

	return baseApps, targetApps, time.Since(startTime), nil
}

func processAppSets(
	argocd *argocd.ArgoCDInstallation,
	appSets *ArgoSelection,
	branch *git.Branch,
	tempFolder string,
	debug bool,
	failOnDuplicateGeneratedApplications bool,
	appSelectionOptions ApplicationSelectionOptions,
	repo string,
	redirectRevisions []string,
) (*ArgoSelection, error) {

	appSetConversionResult, err := convertAppSetsToApps(
		argocd,
		appSets.SelectedApps,
		branch,
		tempFolder,
		debug,
		failOnDuplicateGeneratedApplications,
	)
	if err != nil {
		log.Error().Str("branch", branch.Name).Msg("❌ Failed to generate apps")
		return nil, err
	}

	if appSetConversionResult.appSetsProcessedCount > 0 {
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Generated %d applications from %d ApplicationSets",
			appSetConversionResult.generatedApplicationsCount,
			appSetConversionResult.appSetsProcessedCount,
		)
	} else {
		log.Info().Str("branch", branch.Name).Msg("🤖 No ApplicationSets found for branch")
	}

	// if no newly generated applications were found skip the patching and filtering
	if appSetConversionResult.generatedApplicationsCount <= 0 {
		return &ArgoSelection{
			SelectedApps: appSetConversionResult.argoResource,
			SkippedApps:  appSets.SkippedApps,
		}, nil
	}

	// if there is no apps after conversion just return the apps that were skipped
	if len(appSetConversionResult.argoResource) == 0 {
		return &ArgoSelection{
			SelectedApps: appSetConversionResult.argoResource,
			SkippedApps:  appSets.SkippedApps,
		}, nil
	}

	selection := ApplicationSelection(appSetConversionResult.argoResource, appSelectionOptions)

	// real applications (not application sets)
	numberOfNewlySkippedApps := len(selection.SkippedApps)
	selectedAppsBeforeConversion := appSetConversionResult.originalApplicationsCount
	selectedAppsAfterConversion := len(selection.SelectedApps)
	numberOfNewlySelectedApplicationsCount := selectedAppsAfterConversion - selectedAppsBeforeConversion

	// Sanity check
	if numberOfNewlySkippedApps < 0 {
		log.Fatal().Str("branch", branch.Name).Msg("❌ This should never happen. Please report this as a bug. Number of newly skipped applications is negative.")
	}
	if numberOfNewlySelectedApplicationsCount < 0 {
		log.Fatal().Str("branch", branch.Name).Msg("❌ This should never happen. Please report this as a bug. Number of newly selected applications is negative.")
	}

	if numberOfNewlySelectedApplicationsCount > 0 && numberOfNewlySkippedApps <= 0 {
		log.Info().Str("branch", branch.Name).Msgf("🤖 Selected all %d Applications, Skipped none", numberOfNewlySelectedApplicationsCount)
	} else {
		log.Info().Str("branch", branch.Name).Msgf("🤖 Selected %d Applications, Skipped %d Applications of the newly generated applications", numberOfNewlySelectedApplicationsCount, numberOfNewlySkippedApps)
	}

	log.Info().Str("branch", branch.Name).Msgf("🤖 Patching %d Applications from ApplicationSets", numberOfNewlySelectedApplicationsCount)
	// We are actually patching all apps again. Not only the newly selected ones.
	patchedApps, err := patchApplications(
		argocd.Namespace,
		selection.SelectedApps,
		branch,
		repo,
		redirectRevisions,
	)
	if err != nil {
		log.Error().Str("branch", branch.Name).Msg("❌ Failed to patch new Applications from ApplicationSets")
		return nil, err
	}

	log.Debug().Str("branch", branch.Name).Msgf("Patched all %d applications", len(patchedApps))

	if debug {
		appTempFolder := fmt.Sprintf("%s/apps", tempFolder)
		if err := utils.CreateFolder(appTempFolder, true); err != nil {
			log.Error().Msgf("❌ Failed to create temp folder: %s", appTempFolder)
		}

		for _, app := range patchedApps {
			if _, err := app.WriteToFolder(appTempFolder); err != nil {
				log.Error().Err(err).Str("branch", branch.Name).Str(app.Kind.ShortName(), app.GetLongName()).Msgf("❌ Failed to write Application to file")
				break
			}
		}
	}

	return &ArgoSelection{
		SelectedApps: patchedApps,
		SkippedApps:  append(appSets.SkippedApps, selection.SkippedApps...),
	}, nil
}

type AppSetConversionResult struct {
	originalApplicationsCount  int // real applications (not application sets)
	generatedApplicationsCount int // real applications (not application sets)
	appSetsProcessedCount      int
	argoResource               []ArgoResource
}

// appSetGenerateResult holds the output of a single ApplicationSet generation call.
type appSetGenerateResult struct {
	index int            // original index in the onlyAppSets slice (for stable ordering)
	apps  []ArgoResource // generated Applications from this ApplicationSet
	err   error
}

// maxAppSetConcurrency is the maximum number of ApplicationSet generation
// calls that run in parallel. Each call hits the ArgoCD API/CLI, so we
// cap concurrency to avoid overwhelming the server.
const maxAppSetConcurrency = 5

func convertAppSetsToApps(
	argocd *argocd.ArgoCDInstallation,
	appSets []ArgoResource,
	branch *git.Branch,
	tempFolder string,
	debug bool,
	failOnDuplicateGeneratedApplications bool,
) (*AppSetConversionResult, error) {

	log.Debug().Str("branch", branch.Name).Msg("🤖 Generating Applications from ApplicationSets")

	if debug {
		if err := argocd.EnsureArgoCdIsReady(); err != nil {
			return nil, fmt.Errorf("failed to wait for deployments to be ready: %w", err)
		}
	}

	// Separate plain Applications from ApplicationSets so we only
	// parallelise the expensive generation calls.
	var plainApps []ArgoResource
	var onlyAppSets []ArgoResource
	for _, res := range appSets {
		if res.Kind != ApplicationSet {
			plainApps = append(plainApps, res)
		} else {
			onlyAppSets = append(onlyAppSets, res)
		}
	}

	// Nothing to generate – return early.
	if len(onlyAppSets) == 0 {
		return &AppSetConversionResult{
			appSetsProcessedCount:      0,
			originalApplicationsCount:  len(plainApps),
			generatedApplicationsCount: 0,
			argoResource:               plainApps,
		}, nil
	}

	// --- parallel generation ------------------------------------------------

	sem := make(chan struct{}, maxAppSetConcurrency)
	results := make(chan appSetGenerateResult, len(onlyAppSets))
	var wg sync.WaitGroup

	for i, appSet := range onlyAppSets {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore slot
		go func(i int, appSet ArgoResource) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore slot

			apps, err := generateAppsFromAppSet(argocd, appSet, branch, tempFolder, failOnDuplicateGeneratedApplications)
			results <- appSetGenerateResult{index: i, apps: apps, err: err}
		}(i, appSet)
	}

	// Close results channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	// --- collect results (preserve original ordering) -------------------------

	generatedApplicationsCount := 0
	// Collect results into an indexed slice so the final order matches
	// the original onlyAppSets slice, regardless of goroutine scheduling.
	orderedResults := make([][]ArgoResource, len(onlyAppSets))

	for res := range results {
		if res.err != nil {
			return nil, res.err
		}
		generatedApplicationsCount += len(res.apps)
		orderedResults[res.index] = res.apps
	}

	appsNew := make([]ArgoResource, 0, len(plainApps)+generatedApplicationsCount)
	appsNew = append(appsNew, plainApps...)
	for _, apps := range orderedResults {
		appsNew = append(appsNew, apps...)
	}

	return &AppSetConversionResult{
		appSetsProcessedCount:      len(onlyAppSets),
		originalApplicationsCount:  len(plainApps),
		generatedApplicationsCount: generatedApplicationsCount,
		argoResource:               appsNew,
	}, nil
}

// generateAppsFromAppSet runs a single ApplicationSet through argocd appset
// generate (with retries) and converts the output into ArgoResource values.
func generateAppsFromAppSet(
	argocd *argocd.ArgoCDInstallation,
	appSet ArgoResource,
	branch *git.Branch,
	tempFolder string,
	failOnDuplicateGeneratedApplications bool,
) ([]ArgoResource, error) {
	retryCount := 5
	generatedApps, err := argocd.AppsetGenerateWithRetry(appSet.Yaml, tempFolder, retryCount)
	if err != nil {
		log.Error().Err(err).Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msg("❌ Failed to generate applications from ApplicationSet")
		return nil, err
	}

	if len(generatedApps) == 0 {
		log.Warn().Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msgf("⚠️ ApplicationSet generated empty output")
		return nil, nil
	}

	var apps []ArgoResource
	for _, doc := range generatedApps {
		kind := doc.GetKind()
		if kind == "" {
			log.Error().
				Str(appSet.Kind.ShortName(), appSet.GetLongName()).
				Msg("❌ Output from ApplicationSet contains no kind")
			continue
		}
		if kind != "Application" {
			log.Error().
				Str(appSet.Kind.ShortName(), appSet.GetLongName()).
				Msg("❌ Output from ApplicationSet contains non-Application resources")
			continue
		}

		name := doc.GetName()
		if name == "" {
			log.Error().Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msg("❌ Generated Application missing name")
			continue
		}

		docCopy := doc.DeepCopy()
		app := NewArgoResource(docCopy, Application, name, name, appSet.FileName, branch.Type())
		apps = append(apps, *app)
	}

	if err := validateGeneratedApplicationNames(appSet, apps, branch, failOnDuplicateGeneratedApplications); err != nil {
		return nil, err
	}

	log.Debug().Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msgf(
		"Generated %d Applications from ApplicationSet",
		len(apps),
	)

	return apps, nil
}

func duplicateGeneratedApplicationNames(apps []ArgoResource) []string {
	counts := make(map[string]int)
	for _, app := range apps {
		counts[app.Name]++
	}

	var duplicates []string
	for name, count := range counts {
		if count > 1 {
			duplicates = append(duplicates, name)
		}
	}

	slices.Sort(duplicates)
	return duplicates
}

func validateGeneratedApplicationNames(
	appSet ArgoResource,
	apps []ArgoResource,
	branch *git.Branch,
	failOnDuplicateGeneratedApplications bool,
) error {
	duplicates := duplicateGeneratedApplicationNames(apps)
	if len(duplicates) == 0 {
		return nil
	}

	msg := fmt.Sprintf(
		"ApplicationSet %s generated applications with duplicate names: %s",
		appSet.GetLongName(),
		strings.Join(duplicates, ", "),
	)

	if failOnDuplicateGeneratedApplications {
		return fmt.Errorf("%s", msg)
	}

	log.Warn().
		Str("branch", branch.Name).
		Str(appSet.Kind.ShortName(), appSet.GetLongName()).
		Msgf("⚠️ %s. Continuing because --fail-on-duplicate-generated-applications is not enabled", msg)

	return nil
}
