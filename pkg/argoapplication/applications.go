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

// GetApplicationsForBranches gets applications for both base and target branches
func GetApplicationsForBranches(
	argocdNamespace string,
	baseBranch *git.Branch,
	targetBranch *git.Branch,
	filterOptions FilterOptions,
	repo string,
	redirectRevisions []string,
) ([]ArgoResource, []ArgoResource, error) {

	// GET APPLICATIONS FOR BOTH BRANCHES ------------------------------------------------------
	baseApps, err := getApplications(baseBranch, filterOptions.FileRegex)
	if err != nil {
		return nil, nil, err
	}

	targetApps, err := getApplications(targetBranch, filterOptions.FileRegex)
	if err != nil {
		return nil, nil, err
	}

	// FILTER APPLICATIONS ------------------------------------------------------
	baseAppsSelected, targetAppsSelected := FilterApps(baseApps, targetApps, filterOptions, baseBranch, targetBranch)

	// PATCH APPLICATIONS ------------------------------------------------------
	baseAppsPatched, err := patchApplications(
		argocdNamespace,
		baseAppsSelected,
		baseBranch,
		repo,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	targetAppsPatched, err := patchApplications(
		argocdNamespace,
		targetAppsSelected,
		targetBranch,
		repo,
		redirectRevisions,
	)
	if err != nil {
		return nil, nil, err
	}

	return baseAppsPatched, targetAppsPatched, nil
}

// getApplications gets applications for a single branch
func getApplications(
	branch *git.Branch,
	fileRegex *string,
) ([]ArgoResource, error) {
	log.Info().Str("branch", branch.Name).Msg("🤖 Fetching all files for branch")

	yamlFiles := fileparsing.GetYamlFiles(branch.FolderName(), fileRegex)
	log.Info().Str("branch", branch.Name).Msgf("🤖 Found %d files in dir %s", len(yamlFiles), branch.FolderName())

	k8sResources := fileparsing.ParseYaml(branch.FolderName(), yamlFiles, branch.Type())
	log.Info().Str("branch", branch.Name).Msgf("🤖 Which resulted in %d Kubernetes Resources", len(k8sResources))

	applications := FromResourceToApplication(k8sResources)
	log.Info().Str("branch", branch.Name).Msgf("🤖 Which resulted in %d Argo CD Applications or ApplicationSets", len(applications))
	return applications, nil
}
