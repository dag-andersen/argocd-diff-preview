package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

func (c *Client) CheckIfResourceExists(gvr schema.GroupVersionResource, namespace string, name string) (bool, error) {
	_, err := c.clientSet.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

// Helper function to apply a single manifest from an unstructured object
func (c *Client) ApplyManifest(obj *unstructured.Unstructured, source string, fallbackNamespace string) error {
	// Skip if the document doesn't have a kind or apiVersion
	if obj.GetKind() == "" || obj.GetAPIVersion() == "" {
		log.Debug().Msg("Skipping document with no kind or apiVersion")
		return nil
	}

	// Get resource GVR using proper discovery (same as kubectl api-resources)
	gv, err := schema.ParseGroupVersion(obj.GetAPIVersion())
	if err != nil {
		return fmt.Errorf("invalid apiVersion: %w", err)
	}

	gvk := schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    obj.GetKind(),
	}

	// Use REST mapper to get the correct GVR (handles proper pluralization)
	// Retry once with cache invalidation if the first attempt fails
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		// If we get a "no matches" error, the cache might be stale
		// Invalidate the cache and retry
		if strings.Contains(err.Error(), "no matches for kind") {
			log.Debug().Msgf("REST mapping failed for %s, invalidating cache and retrying", gvk.String())
			c.cachedDiscoveryClient.Invalidate()
			c.mapper.Reset()

			// Retry after cache invalidation
			mapping, err = c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err != nil {
				return fmt.Errorf("failed to get REST mapping for %s (after cache invalidation): %w", gvk.String(), err)
			}
		} else {
			return fmt.Errorf("failed to get REST mapping for %s: %w", gvk.String(), err)
		}
	}

	gvr := mapping.Resource

	isNamespaced := mapping.Scope.Name() == "namespace"

	// Determine namespace (only for namespaced resources)
	var namespace string
	if isNamespaced {
		namespace = obj.GetNamespace()
		if namespace == "" {
			namespace = fallbackNamespace
		}
	}

	// Check if resource is cluster-scoped or namespaced
	var resourceInterface dynamic.ResourceInterface

	if isNamespaced {
		resourceInterface = c.clientSet.Resource(gvr).Namespace(namespace)
	} else {
		// Cluster-scoped resource (e.g., CRD, ClusterRole, Namespace)
		resourceInterface = c.clientSet.Resource(gvr)
	}

	logEvent := log.Debug().
		Str("name", obj.GetName()).
		Str("kind", obj.GetKind()).
		Str("resource", gvr.Resource).
		Str("apiVersion", obj.GetAPIVersion()).
		Str("source", source)

	if isNamespaced {
		logEvent.Str("namespace", namespace).Msg("Applying namespaced manifest with server-side apply")
	} else {
		logEvent.Msg("Applying cluster-scoped manifest with server-side apply")
	}

	_, err = resourceInterface.Apply(
		context.Background(),
		obj.GetName(),
		obj,
		metav1.ApplyOptions{FieldManager: "argocd-diff-preview"},
	)
	if err != nil {
		return fmt.Errorf("failed to apply %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}

	return nil
}

// ApplyManifestFromFile applies a Kubernetes manifest from a file
func (c *Client) ApplyManifestFromFile(path string, fallbackNamespace string) (int, error) {
	// Read manifest file
	manifestBytes, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("failed to read manifest file: %w", err)
	}

	// Check if file is empty
	if len(manifestBytes) == 0 {
		log.Debug().Str("path", path).Msg("Skipping empty manifest file")
		return 0, nil
	}

	return c.ApplyManifestFromString(string(manifestBytes), fallbackNamespace)
}

func (c *Client) ApplyManifestFromString(manifest string, fallbackNamespace string) (int, error) {
	// Check if manifest is empty
	if strings.TrimSpace(manifest) == "" {
		log.Debug().Msg("Skipping empty manifest string")
		return 0, nil
	}

	// Split manifest into multiple documents (if any)
	documents := utils.SplitYAMLDocuments(manifest)

	count := 0

	for _, doc := range documents {

		// Parse YAML into unstructured object
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(doc), &obj.Object); err != nil {
			return count, fmt.Errorf("failed to parse manifest YAML: %w", err)
		}

		if err := c.ApplyManifest(obj, "string", fallbackNamespace); err != nil {
			return count, err
		}

		count++
	}

	return count, nil
}

// create namespace. Returns true if the namespace was created, false if it already existed.
func (c *Client) CreateNamespace(namespace string) (bool, error) {
	namespaceRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}

	// First, check if the namespace already exists
	_, err := c.clientSet.Resource(namespaceRes).Get(context.Background(), namespace, metav1.GetOptions{})
	if err == nil {
		// Namespace already exists, no need to create
		return false, nil
	}

	// If the error is not "not found", return the error
	if !strings.Contains(err.Error(), "not found") {
		return false, fmt.Errorf("failed to check if namespace exists: %w", err)
	}

	// Namespace doesn't exist, create it
	_, err = c.clientSet.Resource(namespaceRes).Create(context.Background(), &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": namespace,
			},
		},
	}, metav1.CreateOptions{})
	return true, err
}

func (c *Client) GetConfigMaps(namespace string, names ...string) (string, error) {
	configMapRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	// If no specific names are provided, get all ConfigMaps in the namespace
	if len(names) == 0 {
		result, err := c.clientSet.Resource(configMapRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return "", err
		}

		resultString, err := yaml.Marshal(result)
		if err != nil {
			return "", err
		}
		return string(resultString), nil
	}

	// For multiple ConfigMaps, fetch them individually and combine results
	var items []unstructured.Unstructured

	for _, name := range names {
		obj, err := c.clientSet.Resource(configMapRes).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to get ConfigMap %s: %w", name, err)
		}
		items = append(items, *obj)
	}

	// Create a combined result
	combinedResult := &unstructured.UnstructuredList{
		Items: items,
	}

	resultString, err := yaml.Marshal(combinedResult)
	if err != nil {
		return "", err
	}
	return string(resultString), nil
}

// get secret value from key. e.g. key: "password"
func (c *Client) GetSecretValue(namespace string, name string, key string) (string, error) {
	secretRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	result, err := c.clientSet.Resource(secretRes).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// get value from path
	value, ok := result.Object["data"].(map[string]any)[key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret %s", key, name)
	}

	// convert value to string
	valueString, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value is not a string")
	}

	// decode
	decoded, err := base64.StdEncoding.DecodeString(valueString)
	if err != nil {
		return "", fmt.Errorf("failed to decode value: %w", err)
	}

	return string(decoded), nil
}
