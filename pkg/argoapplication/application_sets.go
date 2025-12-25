package argoapplication

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"sigs.k8s.io/yaml"
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
		appSelectionOptions,
		repo,
		redirectRevisions,
	)
	if err != nil {
		log.Error().Str("branch", targetBranch.Name).Msg("❌ Failed to generate target apps")
		return nil, nil, time.Since(startTime), err
	}

	return baseApps, targetApps, time.Since(startTime), nil
}

func processAppSets(
	argocd *argocd.ArgoCDInstallation,
	appSets *ArgoSelection,
	branch *git.Branch,
	tempFolder string,
	debug bool,
	appSelectionOptions ApplicationSelectionOptions,
	repo string,
	redirectRevisions []string,
) (*ArgoSelection, error) {

	appSetTempFolder := fmt.Sprintf("%s/app-sets", tempFolder)
	if err := utils.CreateFolder(appSetTempFolder, true); err != nil {
		log.Error().Msgf("❌ Failed to create temp folder: %s", appSetTempFolder)
		return nil, err
	}

	appSetConversionResult, err := convertAppSetsToApps(
		argocd,
		appSets.SelectedApps,
		branch,
		appSetTempFolder,
		debug,
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
	selectedAppsBeforeConversion := appSetConversionResult.origialApplicationsCount
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
	origialApplicationsCount   int // real applications (not application sets)
	generatedApplicationsCount int // real applications (not application sets)
	appSetsProcessedCount      int
	argoResource               []ArgoResource
}

func convertAppSetsToApps(
	argocd *argocd.ArgoCDInstallation,
	appSets []ArgoResource,
	branch *git.Branch,
	tempFolder string,
	debug bool,
) (*AppSetConversionResult, error) {
	var appsNew []ArgoResource
	appSetsProcessedCount := 0
	generatedApplicationsCounter := 0
	origialApplicationsCounter := 0

	log.Debug().Str("branch", branch.Name).Msg("🤖 Generating Applications from ApplicationSets")

	if debug {
		if err := argocd.EnsureArgoCdIsReady(); err != nil {
			return nil, fmt.Errorf("failed to wait for deployments to be ready: %w", err)
		}
	}

	for _, appSet := range appSets {
		// Skip non-ApplicationSets
		if appSet.Kind != ApplicationSet {
			appsNew = append(appsNew, appSet)
			origialApplicationsCounter++
			continue
		}

		appSetsProcessedCount++

		randomFileName, err := appSet.WriteToFolder(tempFolder)
		if err != nil {
			log.Error().Err(err).Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msgf("❌ Failed to write ApplicationSet to file")
			continue
		}

		// Generate applications using argocd appset generate
		retryCount := 5
		output, err := argocd.AppsetGenerateWithRetry(randomFileName, retryCount)
		if err != nil {
			log.Error().Err(err).Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msg("❌ Failed to generate applications from ApplicationSet")
			return nil, err
		}

		// check if output is empty / null
		if strings.TrimSpace(output) == "" || strings.TrimSpace(output) == "null" {
			log.Warn().Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msgf("⚠️ ApplicationSet generated empty output")
			continue
		}

		// check if output is list of applications
		isList := strings.HasPrefix(output, "-")

		var yamlData []unstructured.Unstructured
		if isList {
			var yamlOutput []unstructured.Unstructured
			if err := yaml.Unmarshal([]byte(output), &yamlOutput); err != nil {
				log.Error().Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msg("❌ Failed to read output from ApplicationSet")
				log.Error().Err(err)
				continue
			}
			yamlData = yamlOutput
		} else {
			var yamlOutput unstructured.Unstructured
			if err := yaml.Unmarshal([]byte(output), &yamlOutput); err != nil {
				log.Error().Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msg("❌ Failed to read output from ApplicationSet")
				log.Error().Err(err)
				continue
			}
			yamlData = []unstructured.Unstructured{yamlOutput}
		}

		if len(yamlData) == 0 {
			log.Error().Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msg("❌ No applications found in ApplicationSet")
			continue
		}

		localGeneratedAppsCounter := 0

		// Convert each document to ArgoResource
		for _, doc := range yamlData {
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

			// Create a deep copy of the YAML node to avoid reference issues
			docCopy := doc.DeepCopy()

			app := NewArgoResource(docCopy, Application, name, name, appSet.FileName, branch.Type())

			localGeneratedAppsCounter++
			generatedApplicationsCounter++
			appsNew = append(appsNew, *app)
		}

		log.Debug().Str("branch", branch.Name).Str(appSet.Kind.ShortName(), appSet.GetLongName()).Msgf(
			"Generated %d Applications from ApplicationSet",
			localGeneratedAppsCounter,
		)
	}

	return &AppSetConversionResult{
		appSetsProcessedCount:      appSetsProcessedCount,
		origialApplicationsCount:   origialApplicationsCounter,
		generatedApplicationsCount: generatedApplicationsCounter,
		argoResource:               appsNew,
	}, nil
}
