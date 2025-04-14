package argoapplicaiton

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	k8s "github.com/dag-andersen/argocd-diff-preview/pkg/k8s"
	"github.com/dag-andersen/argocd-diff-preview/pkg/selector"
	"sigs.k8s.io/yaml"
)

// GetApplicationsForBranches gets applications for both base and target branches
func GetApplicationsForBranches(
	argocdNamespace string,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	fileRegex *string,
	selector []selector.Selector,
	filesChanged []string,
	repo string,
	ignoreInvalidWatchPattern bool,
	redirectRevisions []string,
) ([]ArgoResource, []ArgoResource, error) {
	baseApps, err := GetApplications(
		argocdNamespace,
		baseBranch,
		fileRegex,
		selector,
		filesChanged,
		repo,
		ignoreInvalidWatchPattern,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	targetApps, err := GetApplications(
		argocdNamespace,
		targetBranch,
		fileRegex,
		selector,
		filesChanged,
		repo,
		ignoreInvalidWatchPattern,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	// Find duplicates
	var duplicateYaml []*unstructured.Unstructured
	for _, baseApp := range baseApps {
		for _, targetApp := range targetApps {
			if baseApp.Name == targetApp.Name && yamlEqual(baseApp.Yaml, targetApp.Yaml) {
				log.Debug().Msgf("Skipping application '%s' because it has not changed", baseApp.Name)
				duplicateYaml = append(duplicateYaml, baseApp.Yaml)
				break
			}
		}
	}

	if len(duplicateYaml) == 0 {
		return baseApps, targetApps, nil
	}

	// Remove duplicates and log stats
	baseAppsBefore := len(baseApps)
	targetAppsBefore := len(targetApps)

	if len(duplicateYaml) == 0 {
		return baseApps, targetApps, nil
	}

	// Actually filter out the duplicates using the helper function
	baseApps = filterDuplicates(baseApps, duplicateYaml)
	targetApps = filterDuplicates(targetApps, duplicateYaml)

	log.Info().Str("branch", baseBranch.Name).Msgf(
		"🤖 Skipped %d Application[Sets] because they have not changed after patching",
		baseAppsBefore-len(baseApps),
	)

	log.Info().Str("branch", targetBranch.Name).Msgf(
		"🤖 Skipped %d Application[Sets] because they have not changed after patching",
		targetAppsBefore-len(targetApps),
	)

	log.Info().Str("branch", baseBranch.Name).Msgf(
		"🤖 Using the remaining %d Application[Sets]",
		len(baseApps),
	)

	log.Info().Str("branch", targetBranch.Name).Msgf(
		"🤖 Using the remaining %d Application[Sets]",
		len(targetApps),
	)

	return baseApps, targetApps, nil
}

// GetApplications gets applications for a single branch
func GetApplications(
	argocdNamespace string,
	branch *git.Branch,
	fileRegex *string,
	selector []selector.Selector,
	filesChanged []string,
	repo string,
	ignoreInvalidWatchPattern bool,
	redirectRevisions []string,
) ([]ArgoResource, error) {
	log.Info().Str("branch", branch.Name).Msgf("🤖 Fetching all files for branch %s", branch.Name)

	yamlFiles := k8s.GetYamlFiles(branch.FolderName(), fileRegex)
	log.Info().Str("branch", branch.Name).Msgf("🤖 Found %d files in dir %s", len(yamlFiles), branch.FolderName())

	k8sResources := k8s.ParseYaml(branch.FolderName(), yamlFiles)
	log.Info().Str("branch", branch.Name).Msgf("🤖 Which resulted in %d k8sResources", len(k8sResources))

	applications := FromResourceToApplication(
		k8sResources,
		selector,
		filesChanged,
		ignoreInvalidWatchPattern,
		branch,
	)

	if len(applications) == 0 {
		return []ArgoResource{}, nil
	}

	log.Info().Str("branch", branch.Name).Msgf("🤖 Patching %d Application[Sets]", len(applications))

	applications, err := PatchApplications(
		argocdNamespace,
		applications,
		branch,
		repo,
		redirectRevisions,
	)
	if err != nil {
		return nil, err
	}

	log.Debug().Str("branch", branch.Name).Msgf("Patched %d Application[Sets]", len(applications))

	return applications, nil
}

// Helper functions

func filterDuplicates(apps []ArgoResource, duplicates []*unstructured.Unstructured) []ArgoResource {
	var filtered []ArgoResource
	for _, app := range apps {
		isDuplicate := false
		for _, dup := range duplicates {
			if yamlEqual(app.Yaml, dup) {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			filtered = append(filtered, app)
		}
	}
	return filtered
}

// FromResourceToApplication converts K8sResources to ArgoResources with filtering
func FromResourceToApplication(
	k8sResources []k8s.Resource,
	selector []selector.Selector,
	filesChanged []string,
	ignoreInvalidWatchPattern bool,
	branch *git.Branch,
) []ArgoResource {
	var apps []ArgoResource

	// Convert K8sResources to ArgoResources
	for _, r := range k8sResources {
		if app := FromK8sResource(r); app != nil {
			apps = append(apps, *app)
		}
	}

	// Log selector and files changed info
	switch {
	case len(selector) > 0 && len(filesChanged) > 0:
		var selectorStrs []string
		for _, s := range selector {
			selectorStrs = append(selectorStrs, s.String())
		}
		log.Info().Msgf(
			"🤖 Will only run on Applications that match '%s' and watch these files: '%s'",
			strings.Join(selectorStrs, ","),
			strings.Join(filesChanged, "', '"),
		)
	case len(selector) > 0:
		var selectorStrs []string
		for _, s := range selector {
			selectorStrs = append(selectorStrs, s.String())
		}
		log.Info().Msgf(
			"🤖 Will only run on Applications that match '%s'",
			strings.Join(selectorStrs, ","),
		)
	case len(filesChanged) > 0:
		log.Info().Msgf(
			"🤖 Will only run on Applications that watch these files: '%s'",
			strings.Join(filesChanged, "', '"),
		)
	}

	numberOfAppsBeforeFiltering := len(apps)

	// Filter applications
	var filteredApps []ArgoResource
	for _, app := range apps {
		if filtered := app.Filter(selector, filesChanged, ignoreInvalidWatchPattern); filtered != nil {
			filteredApps = append(filteredApps, *filtered)
		}
	}

	// Log filtering results
	if numberOfAppsBeforeFiltering != len(filteredApps) {
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets] before filtering",
			numberOfAppsBeforeFiltering,
		)
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets] after filtering",
			len(filteredApps),
		)
	} else {
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Found %d Application[Sets]",
			numberOfAppsBeforeFiltering,
		)
	}

	return filteredApps
}

// PatchApplication patches a single ArgoResource
func PatchApplication(
	argocdNamespace string,
	application ArgoResource,
	branch *git.Branch,
	repo string,
	redirectRevisions []string,
) (*ArgoResource, error) {
	appName := application.Name

	// Chain the modifications
	app := &application
	err := app.SetNamespace(argocdNamespace)
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to set namespace: %w", err)
	}

	err = app.RemoveSyncPolicy()
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to remove sync policy: %w", err)
	}

	err = app.SetProjectToDefault()
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to set project to default: %w", err)
	}

	err = app.PointDestinationToInCluster()
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to point destination to in-cluster: %w", err)
	}

	err = app.RedirectSources(repo, branch.Name, redirectRevisions)
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to redirect sources: %w", err)
	}

	err = app.RedirectGenerators(repo, branch.Name, redirectRevisions)
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to redirect generators: %w", err)
	}

	return app, nil
}

// PatchApplications patches a slice of ArgoResources
func PatchApplications(
	argocdNamespace string,
	applications []ArgoResource,
	branch *git.Branch,
	repo string,
	redirectRevisions []string,
) ([]ArgoResource, error) {
	var patchedApps []ArgoResource

	for _, app := range applications {
		patchedApp, err := PatchApplication(
			argocdNamespace,
			app,
			branch,
			repo,
			redirectRevisions,
		)
		if err != nil {
			return nil, err
		}
		patchedApps = append(patchedApps, *patchedApp)
	}

	return patchedApps, nil
}

func ConvertAppSetsToAppsInBothBranches(
	argocd *argocd.ArgoCDInstallation,
	baseApps []ArgoResource,
	targetApps []ArgoResource,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	repo string,
	tempFolder string,
	redirectRevisions []string,
	debug bool,
) ([]ArgoResource, []ArgoResource, error) {

	log.Info().Msg("🤖 Converting ApplicationSets to Applications in both branches")

	baseApps = UniqueNames(baseApps, baseBranch)
	targetApps = UniqueNames(targetApps, targetBranch)

	baseApps, err := ConvertAppSetsToApps(
		argocd,
		baseApps,
		baseBranch,
		repo,
		tempFolder,
		redirectRevisions,
		debug,
	)
	if err != nil {
		log.Error().Msgf("❌ Failed to generate base apps: %v", err)
	}

	targetApps, err = ConvertAppSetsToApps(
		argocd,
		targetApps,
		targetBranch,
		repo,
		tempFolder,
		redirectRevisions,
		debug,
	)
	if err != nil {
		log.Error().Msgf("❌ Failed to generate target apps: %v", err)
	}

	baseApps = UniqueNames(baseApps, baseBranch)
	targetApps = UniqueNames(targetApps, targetBranch)

	return baseApps, targetApps, nil
}

func ConvertAppSetsToApps(
	argocd *argocd.ArgoCDInstallation,
	appSets []ArgoResource,
	branch *git.Branch,
	repo string,
	tempFolder string,
	redirectRevisions []string,
	debug bool,
) ([]ArgoResource, error) {
	var appsNew []ArgoResource
	appSetCounter := 0
	generatedAppsCounter := 0

	log.Debug().Str("branch", branch.Name).Msg("🤖 Generating Applications from ApplicationSets")

	for _, appSet := range appSets {
		// Skip non-ApplicationSets
		if appSet.Kind != ApplicationSet {
			appsNew = append(appsNew, appSet)
			continue
		}

		appSetCounter++
		localGeneratedAppsCounter := 0

		// Generate random filename for the patched ApplicationSet
		randomFileName := fmt.Sprintf("%s/%s-%d.yaml",
			tempFolder,
			appSet.Name,
			time.Now().UnixNano(),
		)

		// Write patched ApplicationSet to file
		yamlStr, err := appSet.AsString()
		if err != nil {
			log.Error().Err(err).Str("branch", branch.Name).Msgf("❌ Failed to convert ApplicationSet to YAML")
			continue
		}

		if err := os.WriteFile(randomFileName, []byte(yamlStr), 0644); err != nil {
			log.Error().Err(err).Str("branch", branch.Name).Msgf("❌ Failed to write ApplicationSet to file")
			continue
		}
		if !debug {
			defer func() {
				if err := os.Remove(randomFileName); err != nil {
					log.Warn().Err(err).Str("branch", branch.Name).Msg("⚠️ Failed to remove temporary file")
				}
			}()
		}

		// Generate applications using argocd appset generate
		output, err := argocd.AppsetGenerate(randomFileName)
		if err != nil {
			log.Error().Err(err).Str("branch", branch.Name).Msgf("❌ Failed to generate applications from ApplicationSet %s", appSet.Name)
			continue
		}

		// check if output is empty / null
		if strings.TrimSpace(output) == "" || strings.TrimSpace(output) == "null" {
			log.Warn().Str("branch", branch.Name).Str("file", appSet.FileName).Msgf("⚠️ ApplicationSet %s generated empty output", appSet.Name)
			continue
		}

		// check if output is list of applications
		isList := strings.HasPrefix(output, "-")

		var yamlData []unstructured.Unstructured
		if isList {
			var yamlOutput []unstructured.Unstructured
			if err := yaml.Unmarshal([]byte(output), &yamlOutput); err != nil {
				log.Error().Err(err).Str("branch", branch.Name).Msgf("❌ Failed to read output from ApplicationSet %s", appSet.Name)
				continue
			}
			yamlData = yamlOutput
		} else {
			var yamlOutput unstructured.Unstructured
			if err := yaml.Unmarshal([]byte(output), &yamlOutput); err != nil {
				log.Error().Err(err).Str("branch", branch.Name).Msgf("❌ Failed to read output from ApplicationSet %s", appSet.Name)
				continue
			}
			yamlData = []unstructured.Unstructured{yamlOutput}
		}

		if len(yamlData) == 0 {
			log.Error().Str("branch", branch.Name).Msgf("❌ No applications found in ApplicationSet %s", appSet.Name)
			continue
		}

		// Convert each document to ArgoResource
		for _, doc := range yamlData {
			kind := doc.GetKind()
			if kind == "" {
				log.Error().
					Str("file", appSet.FileName).
					Msg("❌ Output from ApplicationSet contains no kind")
				continue
			}
			if kind != "Application" {
				log.Error().
					Str("file", appSet.FileName).
					Msg("❌ Output from ApplicationSet contains non-Application resources")
				continue
			}

			name := doc.GetName()
			if name == "" {
				log.Error().Str("file", appSet.FileName).Msg("❌ Generated Application missing name")
				continue
			}

			// Create a deep copy of the YAML node to avoid reference issues
			docCopy := doc.DeepCopy()

			app := ArgoResource{
				Yaml:     docCopy,
				Kind:     Application,
				Name:     name,
				FileName: appSet.FileName,
			}

			patchedApp, err := PatchApplication(
				argocd.Namespace,
				app,
				branch,
				repo,
				redirectRevisions,
			)
			if err != nil {
				log.Error().Err(err).Str("branch", branch.Name).Msgf("❌ Failed to patch application: %s", name)
				continue
			}

			localGeneratedAppsCounter++
			generatedAppsCounter++
			appsNew = append(appsNew, *patchedApp)
		}

		log.Debug().Str("branch", branch.Name).Str("file", appSet.FileName).Str("appSet", appSet.Name).Msgf(
			"Generated %d Applications from ApplicationSet",
			localGeneratedAppsCounter,
		)
	}

	// After all apps are processed, ensure unique names
	appsNew = UniqueNames(appsNew, branch)

	if appSetCounter > 0 {
		log.Info().Str("branch", branch.Name).Msgf(
			"🤖 Generated %d applications from %d ApplicationSets for branch: %s",
			generatedAppsCounter, appSetCounter, branch.Name,
		)
	} else {
		log.Info().Msgf("🤖 No ApplicationSets found for branch: %s", branch.Name)
	}

	return appsNew, nil
}
