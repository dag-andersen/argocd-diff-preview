package diff

import (
	"github.com/dag-andersen/argocd-diff-preview/pkg/arrayorder"
	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// sortArraysInApps returns deep copies of the apps where every array matched by
// one of the rules is sorted into a canonical order.
// The input is left untouched so manifest output files keep the original order.
func sortArraysInApps(apps []extract.ExtractedApp, rules []arrayorder.Rule) []extract.ExtractedApp {
	result := make([]extract.ExtractedApp, len(apps))
	for i, app := range apps {
		manifests := make([]unstructured.Unstructured, len(app.Manifests))
		for j := range app.Manifests {
			copied := app.Manifests[j].DeepCopy()
			arrayorder.Apply(copied.Object, rules)
			manifests[j] = *copied
		}
		app.Manifests = manifests
		result[i] = app
	}
	return result
}
