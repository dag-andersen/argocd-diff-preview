package argoapplication

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	k8s "github.com/dag-andersen/argocd-diff-preview/pkg/k8s"
	"sigs.k8s.io/yaml"
)

// ArgoResource represents an Argo CD Application or ApplicationSet
type ArgoResource struct {
	Yaml     *unstructured.Unstructured
	Kind     ApplicationKind
	Id       string // The ID is the name of the k8s resource
	Name     string // The name is the original name of the Application
	FileName string
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

// GetApplicationsForBranches gets applications for both base and target branches
func GetApplicationsForBranches(
	argocdNamespace string,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	fileRegex *string,
	filterOptions FilterOptions,
	repo string,
	redirectRevisions []string,
) ([]ArgoResource, []ArgoResource, error) {
	baseApps, err := getApplications(
		argocdNamespace,
		baseBranch,
		fileRegex,
		filterOptions,
		repo,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	targetApps, err := getApplications(
		argocdNamespace,
		targetBranch,
		fileRegex,
		filterOptions,
		repo,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	return baseApps, targetApps, nil
}

// getApplications gets applications for a single branch
func getApplications(
	argocdNamespace string,
	branch *git.Branch,
	fileRegex *string,
	filterOptions FilterOptions,
	repo string,
	redirectRevisions []string,
) ([]ArgoResource, error) {
	log.Info().Str("branch", branch.Name).Msg("🤖 Fetching all files for branch")

	yamlFiles := k8s.GetYamlFiles(branch.FolderName(), fileRegex)
	log.Info().Str("branch", branch.Name).Msgf("🤖 Found %d files in dir %s", len(yamlFiles), branch.FolderName())

	k8sResources := k8s.ParseYaml(branch.FolderName(), yamlFiles)
	log.Info().Str("branch", branch.Name).Msgf("🤖 Which resulted in %d k8sResources", len(k8sResources))

	applications := FromResourceToApplication(k8sResources)

	// filter applications
	applications = FilterAllWithLogging(applications, filterOptions, branch)

	if len(applications) == 0 {
		return []ArgoResource{}, nil
	}

	log.Info().Str("branch", branch.Name).Msgf("🤖 Patching %d Application[Sets]", len(applications))

	applications, err := patchApplications(
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
