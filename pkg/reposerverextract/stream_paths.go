package reposerverextract

import (
	"os"
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
	sourceType, err := source.ExplicitType()
	if err != nil || (sourceType != nil && *sourceType != v1alpha1.ApplicationSourceTypeKustomize) {
		return nil
	}
	// Repo-server applies these overrides before selecting the renderer. Keep
	// the checkout when a file could override the source type checked above.
	overrides := []string{".argocd-source.yaml"}
	if app.Id != "" {
		overrides = append(overrides, ".argocd-source-"+app.Id+".yaml")
	}
	for _, name := range overrides {
		if _, err := os.Stat(filepath.Join(branchFolder, source.Path, name)); !os.IsNotExist(err) {
			return nil
		}
	}
	paths := argocdpath.GetSourceRefreshPaths(&v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Annotations: app.Yaml.GetAnnotations()},
	}, source)
	if len(paths) == 0 {
		return nil
	}
	return append(paths, source.Path)
}
