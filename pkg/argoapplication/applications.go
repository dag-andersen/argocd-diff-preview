package argoapplication

import (
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/fileparsing"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"sigs.k8s.io/yaml"
)

// ArgoResource represents an Argo CD Application or ApplicationSet
type ArgoResource struct {
	Yaml     *unstructured.Unstructured
	Kind     ApplicationKind
	Id       string // The ID is the name of the k8s resource
	Name     string // The name is the original name of the Application
	FileName string
	Branch   git.BranchType
}

// NewArgoResource creates a new ArgoResource
func NewArgoResource(yaml *unstructured.Unstructured, kind ApplicationKind, id string, name string, fileName string, branch git.BranchType) *ArgoResource {
	return &ArgoResource{
		Yaml:     yaml,
		Kind:     kind,
		Id:       id,
		Name:     name,
		FileName: fileName,
		Branch:   branch,
	}
}

func (a *ArgoResource) GetLongName() string {
	return fmt.Sprintf("%s [%s]", a.Name, a.FileName)
}

// AsString returns the YAML representation of the resource
func (a *ArgoResource) AsString() (string, error) {
	bytes, err := yaml.Marshal(a.Yaml)
	if err != nil {
		return "", fmt.Errorf("failed to marshal yaml: %w", err)
	}
	return string(bytes), nil
}

// Write to file and return filename
func (a *ArgoResource) WriteToFolder(folder string) (string, error) {
	randomFileName := fmt.Sprintf("%s/%s-%s.yaml",
		folder,
		a.Id,
		utils.UniqueId(),
	)

	yamlStr, err := a.AsString()
	if err != nil {
		return "", fmt.Errorf("failed to convert to yaml: %w", err)
	}

	if err := os.WriteFile(randomFileName, []byte(yamlStr), 0644); err != nil {
		return "", fmt.Errorf("failed to write to file: %w", err)
	}

	return randomFileName, nil
}

// GetApplicationsForBranches gets applications for both base and target branches.
// Apps that are ignored via the argocd-diff-preview/ignore annotation on the target branch
// will also be filtered out from the base branch to avoid showing them as "deleted".
func GetApplicationsForBranches(
	argocdNamespace string,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	filterOptions FilterOptions,
	repo string,
	redirectRevisions []string,
) ([]ArgoResource, []ArgoResource, error) {
	baseApps, _, err := getApplications(
		argocdNamespace,
		baseBranch,
		filterOptions,
		repo,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	targetApps, targetIgnoredApps, err := getApplications(
		argocdNamespace,
		targetBranch,
		filterOptions,
		repo,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	// Filter out apps from base branch that are ignored on target branch.
	// This prevents apps with argocd-diff-preview/ignore annotation from showing as "deleted".
	baseApps = RemoveIgnoredApps(baseApps, targetIgnoredApps, baseBranch.Name)

	return baseApps, targetApps, nil
}

// getApplications gets applications for a single branch.
// Returns (apps, ignoredApps, error) where ignoredApps contains the apps
// that were filtered out due to the argocd-diff-preview/ignore annotation.
func getApplications(
	argocdNamespace string,
	branch *git.Branch,
	filterOptions FilterOptions,
	repo string,
	redirectRevisions []string,
) ([]ArgoResource, []IgnoredApp, error) {
	log.Info().Str("branch", branch.Name).Msg("🤖 Fetching all files for branch")

	yamlFiles := fileparsing.GetYamlFiles(branch.FolderName(), filterOptions.FileRegex)
	log.Info().Str("branch", branch.Name).Msgf("🤖 Found %d files in dir %s", len(yamlFiles), branch.FolderName())

	k8sResources := fileparsing.ParseYaml(branch.FolderName(), yamlFiles, branch.Type())
	log.Info().Str("branch", branch.Name).Msgf("🤖 Which resulted in %d k8sResources", len(k8sResources))

	applications := FromResourceToApplication(k8sResources)

	if len(applications) == 0 {
		return []ArgoResource{}, nil, nil
	}

	// filter applications
	log.Info().Str("branch", branch.Name).Msgf("🤖 Filtering %d Application[Sets]", len(applications))
	filterResult := FilterAllWithLogging(applications, filterOptions, branch)

	if len(filterResult.Apps) == 0 {
		return []ArgoResource{}, filterResult.IgnoredApps, nil
	}

	log.Info().Str("branch", branch.Name).Msgf("🤖 Patching %d Application[Sets]", len(filterResult.Apps))

	patchedApps, err := patchApplications(
		argocdNamespace,
		filterResult.Apps,
		branch,
		repo,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	log.Debug().Str("branch", branch.Name).Msgf("Patched %d Application[Sets]", len(patchedApps))

	return patchedApps, filterResult.IgnoredApps, nil
}
