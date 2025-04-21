package argoapplicaiton

import (
	"github.com/dag-andersen/argocd-diff-preview/pkg/annotations"
	"github.com/rs/zerolog/log"
)

func (a *ArgoResource) setSourcePath(sourcePath string) {
	annotationMap := a.Yaml.GetAnnotations()
	if annotationMap == nil {
		annotationMap = make(map[string]string)
	}
	annotationMap[annotations.SourcePathKey] = sourcePath
	a.Yaml.SetAnnotations(annotationMap)
}

func (a *ArgoResource) setOriginalApplicationName(originalApplicationName string) {
	annotationMap := a.Yaml.GetAnnotations()
	if annotationMap == nil {
		annotationMap = make(map[string]string)
	}
	annotationMap[annotations.OriginalApplicationNameKey] = originalApplicationName
	a.Yaml.SetAnnotations(annotationMap)
}

func (a *ArgoResource) enrichApplication() {
	a.setSourcePath(a.FileName)
	a.setOriginalApplicationName(a.Name)
}

func enrichApplications(applications []ArgoResource) {
	log.Debug().Msgf("Adding source path and original application name to applications: %d", len(applications))
	for _, application := range applications {
		application.enrichApplication()
	}
}
