package extract

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/resource_filter"
	"github.com/rs/zerolog/log"
)

// contains a app name, source path, and extracted manifest
type ExtractedApp struct {
	Id         string
	Name       string
	SourcePath string
	Manifest   []unstructured.Unstructured
	Branch     git.BranchType
}

// CreateExtractedApp creates an ExtractedApp from an ArgoResource
func CreateExtractedApp(id string, name string, sourcePath string, manifest []unstructured.Unstructured, branch git.BranchType) ExtractedApp {
	return ExtractedApp{
		Id:         id,
		Name:       name,
		SourcePath: sourcePath,
		Manifest:   manifest,
		Branch:     branch,
	}
}

func (e *ExtractedApp) FlattenToString(skipResourceRules []resource_filter.SkipResourceRule) (string, error) {
	e.sortManifests()
	var manifestStrings []string
	for _, manifest := range e.Manifest {
		if resource_filter.MatchesAnySkipRule(&manifest, skipResourceRules) {
			msg := fmt.Sprintf("Skipping manifest %s/%s/%s", manifest.GetAPIVersion(), manifest.GetKind(), manifest.GetName())
			log.Debug().Msg(msg)
			manifestStrings = append(manifestStrings, msg)
			continue
		}
		manifestString, err := yaml.Marshal(manifest.Object)
		if err != nil {
			log.Error().Msgf("❌ Failed to convert extracted app to yaml string: %s", err)
			return "", fmt.Errorf("failed to marshal unstructured object: %w", err)
		}
		manifestStrings = append(manifestStrings, string(manifestString))
	}
	return strings.Join(manifestStrings, "---\n"), nil
}

func (e *ExtractedApp) sortManifests() {
	// Sort by API version, then by kind, then by name, with CRDs always at the end

	sort.SliceStable(e.Manifest, func(i, j int) bool {
		apiI := e.Manifest[i].GetAPIVersion()
		apiJ := e.Manifest[j].GetAPIVersion()
		kindI := e.Manifest[i].GetKind()
		kindJ := e.Manifest[j].GetKind()
		nameI := e.Manifest[i].GetName()
		nameJ := e.Manifest[j].GetName()

		// CRDs should always be at the end
		isCRD_I := kindI == "CustomResourceDefinition"
		isCRD_J := kindJ == "CustomResourceDefinition"

		if isCRD_I != isCRD_J {
			// If only one is a CRD, the non-CRD comes first
			return !isCRD_I
		}

		// Sort by apiVersion first, then by kind, then by name
		if apiI != apiJ {
			return apiI < apiJ
		}
		if kindI != kindJ {
			return kindI < kindJ
		}
		return nameI < nameJ
	})
}
