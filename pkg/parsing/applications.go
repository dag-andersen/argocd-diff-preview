package parsing

import (
	"fmt"
	"log"
	"strings"

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

	baseApps = filterDuplicates(baseApps, duplicateYaml)
	targetApps = filterDuplicates(targetApps, duplicateYaml)

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

	// print number of files found
	log.Printf("🤖 Found %d files", len(yamlFiles))

	k8sResources := ParseYaml(branch.FolderName(), yamlFiles)

	// print number of k8sResources found
	log.Printf("🤖 Found %d k8sResources", len(k8sResources))

	applications := FromResourceToApplication(
		k8sResources,
		selector,
		filesChanged,
		ignoreInvalidWatchPattern,
	)

	if len(applications) > 0 {
		log.Printf("🤖 Patching Application[Sets] for branch: %s", branch.Name)
		apps, err := PatchApplications(
			argocdNamespace,
			applications,
			branch,
			repo,
			redirectRevisions,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to patch applications: %w", err)
		}

		log.Printf(
			"🤖 Patching %d Argo CD Application[Sets] for branch: %s",
			len(apps),
			branch.Name,
		)
		return apps, nil
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
	app, err = app.RemoveSyncPolicy()
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to remove sync policy: %w", err)
	}

	app, err = app.SetProjectToDefault()
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to set project to default: %w", err)
	}

	app, err = app.PointDestinationToInCluster()
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to point destination to in-cluster: %w", err)
	}

	app, err = app.RedirectSources(repo, branch.Name, redirectRevisions)
	if err != nil {
		log.Printf("❌ Failed to patch application: %s", appName)
		return nil, fmt.Errorf("failed to redirect sources: %w", err)
	}

	app, err = app.RedirectGenerators(repo, branch.Name, redirectRevisions)
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
