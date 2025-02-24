package parsing

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/argocd"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	"gopkg.in/yaml.v3"
)

// GetApplicationsForBranches gets applications for both base and target branches
func GetApplicationsForBranches(
	argocdNamespace string,
	baseBranch *types.Branch,
	targetBranch *types.Branch,
	regex *string,
	selector []types.Selector,
	filesChanged []string,
	repo string,
	ignoreInvalidWatchPattern bool,
	redirectRevisions []string,
) ([]types.ArgoResource, []types.ArgoResource, error) {
	baseApps, err := GetApplications(
		argocdNamespace,
		baseBranch,
		regex,
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
		regex,
		selector,
		filesChanged,
		repo,
		ignoreInvalidWatchPattern,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	// Find and remove duplicates
	var duplicateYaml []*yaml.Node
	for _, baseApp := range baseApps {
		for _, targetApp := range targetApps {
			if baseApp.Name == targetApp.Name && yamlEqual(baseApp.Yaml, targetApp.Yaml) {
				log.Printf("Skipping application '%s' because it has not changed", baseApp.Name)
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

	log.Printf(
		"🤖 Skipped %d Application[Sets] for branch: '%s' because they have not changed after patching",
		baseAppsBefore-len(baseApps),
		baseBranch.Name,
	)

	log.Printf(
		"🤖 Skipped %d Application[Sets] for branch: '%s' because they have not changed after patching",
		targetAppsBefore-len(targetApps),
		targetBranch.Name,
	)

	log.Printf(
		"🤖 Using the remaining %d Application[Sets] for branch: '%s'",
		len(baseApps),
		baseBranch.Name,
	)

	log.Printf(
		"🤖 Using the remaining %d Application[Sets] for branch: '%s'",
		len(targetApps),
		targetBranch.Name,
	)

	return baseApps, targetApps, nil
}

// GetApplications gets applications for a single branch
func GetApplications(
	argocdNamespace string,
	branch *types.Branch,
	regex *string,
	selector []types.Selector,
	filesChanged []string,
	repo string,
	ignoreInvalidWatchPattern bool,
	redirectRevisions []string,
) ([]types.ArgoResource, error) {
	yamlFiles := GetYamlFiles(branch.FolderName(), regex)
	log.Printf("🤖 Found %d files", len(yamlFiles))

	k8sResources := ParseYaml(branch.FolderName(), yamlFiles)
	log.Printf("🤖 Found %d k8sResources", len(k8sResources))

	applications := FromResourceToApplication(
		k8sResources,
		selector,
		filesChanged,
		ignoreInvalidWatchPattern,
	)

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

	if len(applications) > 0 {
		log.Printf(
			"🤖 Processing %d Argo CD Application[Sets] for branch: %s",
			len(applications),
			branch.Name,
		)
	}

	return applications, nil
}

// Helper functions

func yamlEqual(a, b *yaml.Node) bool {
	aStr, err := yaml.Marshal(a)
	if err != nil {
		return false
	}
	bStr, err := yaml.Marshal(b)
	if err != nil {
		return false
	}
	return string(aStr) == string(bStr)
}

func filterDuplicates(apps []types.ArgoResource, duplicates []*yaml.Node) []types.ArgoResource {
	var filtered []types.ArgoResource
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
	k8sResources []types.K8sResource,
	selector []types.Selector,
	filesChanged []string,
	ignoreInvalidWatchPattern bool,
) []types.ArgoResource {
	var apps []types.ArgoResource

	// Convert K8sResources to ArgoResources
	for _, r := range k8sResources {
		if app := types.FromK8sResource(&r); app != nil {
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
		log.Printf(
			"🤖 Will only run on Applications that match '%s' and watch these files: '%s'",
			strings.Join(selectorStrs, ","),
			strings.Join(filesChanged, "', '"),
		)
	case len(selector) > 0:
		var selectorStrs []string
		for _, s := range selector {
			selectorStrs = append(selectorStrs, s.String())
		}
		log.Printf(
			"🤖 Will only run on Applications that match '%s'",
			strings.Join(selectorStrs, ","),
		)
	case len(filesChanged) > 0:
		log.Printf(
			"🤖 Will only run on Applications that watch these files: '%s'",
			strings.Join(filesChanged, "', '"),
		)
	}

	numberOfAppsBeforeFiltering := len(apps)

	// Filter applications
	var filteredApps []types.ArgoResource
	for _, app := range apps {
		if filtered := app.Filter(selector, filesChanged, ignoreInvalidWatchPattern); filtered != nil {
			filteredApps = append(filteredApps, *filtered)
		}
	}

	// Log filtering results
	if numberOfAppsBeforeFiltering != len(filteredApps) {
		log.Printf(
			"🤖 Found %d Application[Sets] before filtering",
			numberOfAppsBeforeFiltering,
		)
		log.Printf(
			"🤖 Found %d Application[Sets] after filtering",
			len(filteredApps),
		)
	} else {
		log.Printf(
			"🤖 Found %d Application[Sets]",
			numberOfAppsBeforeFiltering,
		)
	}

	return filteredApps
}

// PatchApplication patches a single ArgoResource
func PatchApplication(
	argocdNamespace string,
	application types.ArgoResource,
	branch *types.Branch,
	repo string,
	redirectRevisions []string,
) (*types.ArgoResource, error) {
	appName := application.Name

	// Chain the modifications
	app := &application
	app = app.SetNamespace(argocdNamespace)

	var err error
	appAfterRemoveSyncPolicy, err := app.RemoveSyncPolicy()
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to remove sync policy: %w", err)
	}

	// compare yaml of app and appAfterRemoveSyncPolicy
	if !yamlEqual(app.Yaml, appAfterRemoveSyncPolicy.Yaml) {
		log.Printf("❌ YAML of app and appAfterRemoveSyncPolicy are not equal")
	} else {
		log.Printf("✅ YAML of app and appAfterRemoveSyncPolicy are equal")
	}

	app, err = appAfterRemoveSyncPolicy.SetProjectToDefault()
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to set project to default: %w", err)
	}

	app, err = app.PointDestinationToInCluster()
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to point destination to in-cluster: %w", err)
	}

	appAfterRedirectSources, err := app.RedirectSources(repo, branch.Name, redirectRevisions)
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to redirect sources: %w", err)
	}

	// compare yaml of app and appAfterRedirectSources
	if !yamlEqual(app.Yaml, appAfterRedirectSources.Yaml) {
		log.Printf("❌ YAML of app and appAfterRedirectSources are not equal")
	} else {
		log.Printf("✅ YAML of app and appAfterRedirectSources are equal")
	}

	app, err = appAfterRedirectSources.RedirectGenerators(repo, branch.Name, redirectRevisions)
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to redirect generators: %w", err)
	}

	return app, nil
}

// PatchApplications patches a slice of ArgoResources
func PatchApplications(
	argocdNamespace string,
	applications []types.ArgoResource,
	branch *types.Branch,
	repo string,
	redirectRevisions []string,
) ([]types.ArgoResource, error) {
	var patchedApps []types.ArgoResource

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

// GenerateAppsFromAppSet generates Applications from ApplicationSets
func GenerateAppsFromAppSet(
	argocd *argocd.ArgoCDInstallation,
	appSets []types.ArgoResource,
	branch *types.Branch,
	repo string,
	tempFolder string,
	redirectRevisions []string,
) ([]types.ArgoResource, error) {
	var appsNew []types.ArgoResource
	appSetCounter := 0
	generatedAppsCounter := 0

	log.Printf("🤖 Generating Applications from ApplicationSets for branch: %s", branch.Name)

	for _, appSet := range appSets {
		// Skip non-ApplicationSets
		if appSet.Kind != types.ApplicationSet {
			appsNew = append(appsNew, appSet)
			continue
		}

		appSetCounter++

		// Generate random filename for the patched ApplicationSet
		randomFileName := fmt.Sprintf("%s/%s-%d.yaml",
			tempFolder,
			appSet.Name,
			time.Now().UnixNano(),
		)

		// Write patched ApplicationSet to file
		yamlStr, err := appSet.AsString()
		if err != nil {
			log.Printf("❌ Failed to convert ApplicationSet to YAML: %v", err)
			continue
		}

		if err := os.WriteFile(randomFileName, []byte(yamlStr), 0644); err != nil {
			log.Printf("❌ Failed to write ApplicationSet to file: %v", err)
			continue
		}
		defer os.Remove(randomFileName)

		// Generate applications using argocd appset generate
		output, err := argocd.AppsetGenerate(randomFileName)
		if err != nil {
			log.Printf("❌ Failed to generate applications from ApplicationSet %s: %v", appSet.Name, err)
			continue
		}

		// check if is list of applications
		isList := strings.HasPrefix(output, "-")

		var yamlData []yaml.Node
		if isList {
			var yamlOutput []yaml.Node
			if err := yaml.Unmarshal([]byte(output), &yamlOutput); err == nil {
				yamlData = yamlOutput
			}
		} else {
			var yamlOutput yaml.Node
			if err := yaml.Unmarshal([]byte(output), &yamlOutput); err == nil {
				yamlData = []yaml.Node{yamlOutput}
			}
		}

		if len(yamlData) == 0 {
			log.Printf("❌ No applications found in ApplicationSet %s", appSet.Name)
			continue
		}

		// Convert each document to ArgoResource
		for _, doc := range yamlData {
			kind := types.GetYamlValue(&doc, []string{"kind"})
			if kind == nil || kind.Value != "Application" {
				continue
			}

			name := types.GetYamlValue(&doc, []string{"metadata", "name"})
			if name == nil {
				continue
			}

			app := types.ArgoResource{
				Yaml:     &doc,
				Kind:     types.Application,
				Name:     name.Value,
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
				log.Printf("❌ Failed to patch application: %s", name.Value)
				continue
			}

			generatedAppsCounter++
			appsNew = append(appsNew, *patchedApp)
		}

		log.Printf(
			"Generated %d Applications from ApplicationSet in file: %s",
			generatedAppsCounter,
			appSet.FileName,
		)
	}

	if appSetCounter > 0 {
		log.Printf(
			"🤖 Generated %d applications from %d ApplicationSets for branch: %s",
			generatedAppsCounter, appSetCounter, branch.Name,
		)
	} else {
		log.Printf("🤖 No ApplicationSets found for branch: %s", branch.Name)
	}

	return appsNew, nil
}
