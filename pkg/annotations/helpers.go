package annotations

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Common resource GVRs
var (
	// ApplicationGVR is the GroupVersionResource for ArgoCD applications
	ApplicationGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "applications",
	}
	OriginalApplicationNameKey = "argocd-diff-preview.io/original-application-name"
	SourcePathKey              = "argocd-diff-preview.io/source-path"
)
