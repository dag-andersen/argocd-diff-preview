package extract

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// replaceAppIdInManifests replaces all occurrences of oldId with newName throughout
// all manifests. This is necessary because the app ID appears in many places like
// labels, names, annotations, and various spec fields.
func replaceAppIdInManifests(manifests []unstructured.Unstructured, oldId, newName string) {
	if oldId == newName {
		return
	}
	for i := range manifests {
		manifests[i].Object = replaceStringInObject(manifests[i].Object, oldId, newName).(map[string]any)
	}
}

// replaceStringInObject recursively traverses an object and replaces all string
// occurrences of oldStr with newStr
func replaceStringInObject(obj any, oldStr, newStr string) any {
	switch v := obj.(type) {
	case string:
		return strings.ReplaceAll(v, oldStr, newStr)
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, value := range v {
			result[key] = replaceStringInObject(value, oldStr, newStr)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, value := range v {
			result[i] = replaceStringInObject(value, oldStr, newStr)
		}
		return result
	default:
		// For other types (int, bool, etc.), return as-is
		return obj
	}
}
