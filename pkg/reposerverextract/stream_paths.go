package reposerverextract

import (
	"path/filepath"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	argocdpath "github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Keep the checkout layout, but omit files outside a Kustomize application's
// declared dependencies. Already-staged sources and unannotated applications
// retain their existing archive contents.
func localStreamPaths(app argoapplication.ArgoResource, source v1alpha1.ApplicationSource, branchFolder, streamDir string) []string {
	if streamDir != branchFolder || app.Yaml == nil || !isKustomizeSource(filepath.Join(branchFolder, source.Path)) {
		return nil
	}
	paths := argocdpath.GetSourceRefreshPaths(&v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Annotations: app.Yaml.GetAnnotations()},
	}, source)
	if len(paths) == 0 {
		return nil
	}
	return append(paths, source.Path)
}
