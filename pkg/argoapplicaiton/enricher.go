package argoapplicaiton

import (
	"github.com/dag-andersen/argocd-diff-preview/pkg/annotations"
	"github.com/rs/zerolog/log"
)

func (a *ArgoResource) SetSourcePath(sourcePath string) {
	annotationMap := a.Yaml.GetAnnotations()
	if annotationMap == nil {
		annotationMap = make(map[string]string)
	}
	annotationMap[annotations.SourcePathKey] = sourcePath
	a.Yaml.SetAnnotations(annotationMap)
}

func (a *ArgoResource) SetOriginalApplicationName(originalApplicationName string) {
	annotationMap := a.Yaml.GetAnnotations()
	if annotationMap == nil {
		annotationMap = make(map[string]string)
	}
	annotationMap[annotations.OriginalApplicationNameKey] = originalApplicationName
	a.Yaml.SetAnnotations(annotationMap)
}

func (a *ArgoResource) EnrichApplication() error {
	a.SetSourcePath(a.FileName)
	a.SetOriginalApplicationName(a.Name)

	return nil
}

func EnrichApplications(applications []ArgoResource) error {
	log.Debug().Msgf("Adding source path and original application name to applications: %d", len(applications))
	for _, application := range applications {
		application.EnrichApplication()
	}
	return nil
}
