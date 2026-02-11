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

const (
	// HiddenResourceSuffix is appended to resource headers when they match ignore rules
	HiddenResourceSuffix = ": Hidden"
)

// contains a app name, source path, and extracted manifest
type ExtractedApp struct {
	Id         string
	Name       string
	SourcePath string
	Manifests  []unstructured.Unstructured
	Branch     git.BranchType
}

// CreateExtractedApp creates an ExtractedApp from an ArgoResource
func CreateExtractedApp(id string, name string, sourcePath string, manifest []unstructured.Unstructured, branch git.BranchType) ExtractedApp {
	return ExtractedApp{
		Id:         id,
		Name:       name,
		SourcePath: sourcePath,
		Manifests:  manifest,
		Branch:     branch,
	}
}

func (e *ExtractedApp) FlattenToString(ignoreResourceRules []resource_filter.IgnoreResourceRule) (string, error) {
	e.sortManifests()
	var manifestStrings []string
	for _, manifest := range e.Manifests {

		// If there are ignore resource rules, check if the manifest should be ignored
		if len(ignoreResourceRules) > 0 {
			if resource_filter.MatchesAnyIgnoreRule(&manifest, ignoreResourceRules) {
				kind, name, ns := manifest.GetKind(), manifest.GetName(), manifest.GetNamespace()
				log.Debug().Msgf("Skipping ignored resource: %s/%s", kind, name)
				if ns != "" {
					manifestStrings = append(manifestStrings, fmt.Sprintf("%s/%s (%s)%s\n", kind, name, ns, HiddenResourceSuffix))
				} else {
					manifestStrings = append(manifestStrings, fmt.Sprintf("%s/%s%s\n", kind, name, HiddenResourceSuffix))
				}
				continue
			}
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
	// Sort by API version, then by kind, then by namespace, then by name, with CRDs always at the end

	sort.SliceStable(e.Manifests, func(i, j int) bool {
		apiI := e.Manifests[i].GetAPIVersion()
		apiJ := e.Manifests[j].GetAPIVersion()
		kindI := e.Manifests[i].GetKind()
		kindJ := e.Manifests[j].GetKind()
		nsI := e.Manifests[i].GetNamespace()
		nsJ := e.Manifests[j].GetNamespace()
		nameI := e.Manifests[i].GetName()
		nameJ := e.Manifests[j].GetName()

		// CRDs should always be at the end
		isCRD_I := kindI == "CustomResourceDefinition"
		isCRD_J := kindJ == "CustomResourceDefinition"

		if isCRD_I != isCRD_J {
			// If only one is a CRD, the non-CRD comes first
			return !isCRD_I
		}

		// Sort by apiVersion first, then by kind, then by namespace, then by name
		if apiI != apiJ {
			return apiI < apiJ
		}
		if kindI != kindJ {
			return kindI < kindJ
		}
		if nsI != nsJ {
			return nsI < nsJ
		}
		return nameI < nameJ
	})
}
