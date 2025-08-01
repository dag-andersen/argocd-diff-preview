package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/vars"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type K8sClient struct {
	clientset *dynamic.DynamicClient
	InCluster bool
}

func NewK8sClient() (*K8sClient, error) {

	inCluster := false
	// First try to use the in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		inCluster = true
		log.Info().Msgf("✅ Using service account to connect to cluster")
	} else {
		log.Debug().Err(err).Msgf("Failed to get in-cluster config")

		// fallback to kubeconfig
		kubeconfig := GetKubeConfigPath()
		if kubeconfig == "" {
			return nil, fmt.Errorf("no kubeconfig found, and no in-cluster config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from flags: %w", err)
		}

		log.Debug().Msgf("using kubeconfig: %s", kubeconfig)

	}

	// Increase QPS and Burst to mitigate client-side throttling on the CI
	config.QPS = 20   // Default is 5
	config.Burst = 40 // Default is 10

	clientset, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &K8sClient{clientset: clientset, InCluster: inCluster}, nil
}

func (c *K8sClient) CheckIfResourceExists(gvr schema.GroupVersionResource, namespace string, name string) (bool, error) {
	_, err := c.clientset.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *K8sClient) GetArgoCDApplications(namespace string) (string, error) {
	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	result, err := c.clientset.Resource(applicationRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	// convert result to string
	resultString, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(resultString), nil
}

// GetArgoCDApplication gets a single ArgoCD application by name
func (c *K8sClient) GetArgoCDApplication(namespace string, name string) (string, error) {
	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	result, err := c.clientset.Resource(applicationRes).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// convert result to string
	resultString, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(resultString), nil
}

// DeleteArgoCDApplication deletes a single ArgoCD application by name
func (c *K8sClient) DeleteArgoCDApplication(namespace string, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("no application name provided")
	}
	if strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("no namespace provided")
	}

	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	return c.clientset.Resource(applicationRes).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// DeleteAllApplicationsOlderThan deletes all ArgoCD applications older than a given number of minutes
// and matching the given label key
func (c *K8sClient) DeleteAllApplicationsOlderThan(namespace string, minutes int) error {

	log.Info().Msgf("🧼 Deleting applications older than %d minutes", minutes)

	deletedCount := 0

	listOptions := metav1.ListOptions{
		LabelSelector: vars.ArgoCDApplicationLabelKey,
	}

	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	apps, err := c.clientset.Resource(applicationRes).Namespace(namespace).List(context.Background(), listOptions)
	if err != nil {
		return err
	}

	for _, app := range apps.Items {
		creationTimestamp := app.GetCreationTimestamp()
		timeDiff := time.Since(creationTimestamp.Time)
		if timeDiff.Minutes() > float64(minutes) {
			err := c.clientset.Resource(applicationRes).Namespace(namespace).Delete(context.Background(), app.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Info().Msgf("🧼 Deleted %d applications", deletedCount)
	} else {
		log.Info().Msgf("🧼 No applications with the label '%s' were found older than %d minutes", vars.ArgoCDApplicationLabelKey, minutes)
	}

	return nil
}

func (c *K8sClient) DeleteArgoCDApplications(namespace string) error {

	log.Info().Msg("🧼 Deleting applications")

	// Remove obstructive finalizers
	if err := c.RemoveObstructiveFinalizers(namespace); err != nil {
		return fmt.Errorf("failed to remove obstructive finalizers: %w", err)
	}

	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	apps, err := c.clientset.Resource(applicationRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, app := range apps.Items {
		err := c.clientset.Resource(applicationRes).Namespace(namespace).Delete(context.Background(), app.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Error().Err(err).Msgf("❌ Failed to delete application %s", app.GetName())
		}
	}

	// ensure all applications are deleted
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for applications to be deleted")
		default:
			apps, err := c.clientset.Resource(applicationRes).Namespace(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			if len(apps.Items) == 0 {
				log.Info().Msg("🧼 Deleted applications")
				return nil
			}

			log.Debug().Msgf("Waiting for applications to be deleted: %d", len(apps.Items))

			time.Sleep(1 * time.Second)
		}
	}
}

// RemoveObstructiveFinalizers removes finalizers from applications that would prevent deletion
func (c *K8sClient) RemoveObstructiveFinalizers(namespace string) error {

	// List of obstructiveFinalizers that prevent deletion of applications
	obstructiveFinalizers := []string{
		"post-delete-finalizer.argocd.argoproj.io",
		"post-delete-finalizer.argoproj.io/cleanup",
	}

	log.Debug().Msg("Removing obstructive finalizers from applications")

	// Get ArgoCD applications
	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	apps, err := c.clientset.Resource(applicationRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list applications: %w", err)
	}

	for _, app := range apps.Items {
		appName := app.GetName()
		currentFinalizers := app.GetFinalizers()

		if len(currentFinalizers) == 0 {
			continue
		}

		// Create a map for faster lookup of obstructive finalizers
		obstructiveMap := make(map[string]bool)
		for _, f := range obstructiveFinalizers {
			obstructiveMap[f] = true
		}

		// Check if any current finalizers are in our obstructive list
		foundObstructive := false
		for _, fin := range currentFinalizers {
			if obstructiveMap[fin] {
				foundObstructive = true
				break
			}
		}

		if !foundObstructive {
			continue
		}

		log.Info().Msgf("🧹 Removing obstructive finalizers from application %s", appName)

		app.SetFinalizers(nil)
		_, err := c.clientset.Resource(applicationRes).Namespace(namespace).Update(
			context.Background(),
			&app,
			metav1.UpdateOptions{},
		)

		if err != nil {
			log.Error().Err(err).Msgf("❌ Failed to update finalizers for application %s", appName)
		} else {
			log.Info().Msgf("✅ Removed finalizers from application %s", appName)
		}
	}

	log.Debug().Msg("Finished removing finalizers")
	return nil
}

// Helper function to apply a single manifest from an unstructured object
func (c *K8sClient) ApplyManifest(obj *unstructured.Unstructured, source string, fallbackNamespace string) error {
	// Skip if the document doesn't have a kind or apiVersion
	if obj.GetKind() == "" || obj.GetAPIVersion() == "" {
		log.Debug().Msg("Skipping document with no kind or apiVersion")
		return nil
	}

	// Get resource GVR based on apiVersion and kind
	gv, err := schema.ParseGroupVersion(obj.GetAPIVersion())
	if err != nil {
		return fmt.Errorf("invalid apiVersion: %w", err)
	}

	gvr := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: strings.ToLower(obj.GetKind()) + "s", // Basic pluralization
	}

	// Apply the manifest
	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = fallbackNamespace
	}

	log.Debug().
		Str("name", obj.GetName()).
		Str("namespace", namespace).
		Str("kind", obj.GetKind()).
		Str("source", source).
		Msg("Applying manifest")

	_, err = c.clientset.Resource(gvr).Namespace(namespace).Apply(
		context.Background(),
		obj.GetName(),
		obj,
		metav1.ApplyOptions{FieldManager: "argocd-diff-preview"},
	)
	if err != nil {
		return fmt.Errorf("failed to apply manifest: %w", err)
	}

	return nil
}

// ApplyManifestFromFile applies a Kubernetes manifest from a file
func (c *K8sClient) ApplyManifestFromFile(path string, fallbackNamespace string) (int, error) {
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

func (c *K8sClient) ApplyManifestFromString(manifest string, fallbackNamespace string) (int, error) {
	// Check if manifest is empty
	if strings.TrimSpace(manifest) == "" {
		log.Debug().Msg("Skipping empty manifest string")
		return 0, nil
	}

	// Split manifest into multiple documents (if any)
	documents := strings.Split(manifest, "---")

	count := 0

	for _, doc := range documents {
		// Skip empty documents
		trimmedDoc := strings.TrimSpace(doc)
		if trimmedDoc == "" {
			continue
		}

		// Parse YAML into unstructured object
		obj := &unstructured.Unstructured{}
		if err := yaml.Unmarshal([]byte(trimmedDoc), &obj.Object); err != nil {
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
func (c *K8sClient) CreateNamespace(namespace string) (bool, error) {
	namespaceRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}

	// First, check if the namespace already exists
	_, err := c.clientset.Resource(namespaceRes).Get(context.Background(), namespace, metav1.GetOptions{})
	if err == nil {
		// Namespace already exists, no need to create
		return false, nil
	}

	// If the error is not "not found", return the error
	if !strings.Contains(err.Error(), "not found") {
		return false, fmt.Errorf("failed to check if namespace exists: %w", err)
	}

	// Namespace doesn't exist, create it
	_, err = c.clientset.Resource(namespaceRes).Create(context.Background(), &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": namespace,
			},
		},
	}, metav1.CreateOptions{})
	return true, err
}

func (c *K8sClient) GetConfigMaps(namespace string, names ...string) (string, error) {
	configMapRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	// If no specific names are provided, get all ConfigMaps in the namespace
	if len(names) == 0 {
		result, err := c.clientset.Resource(configMapRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
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
		obj, err := c.clientset.Resource(configMapRes).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
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
func (c *K8sClient) GetSecretValue(namespace string, name string, key string) (string, error) {
	secretRes := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	result, err := c.clientset.Resource(secretRes).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// get value from path
	value, ok := result.Object["data"].(map[string]interface{})[key]
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

// WaitForDeploymentReady waits for a deployment to be available
// Looks for deployments with label app.kubernetes.io/name={name}
func (c *K8sClient) WaitForDeploymentReady(namespace, name string, timeoutSeconds int) error {
	log.Debug().Msgf("Waiting for deployment with label app.kubernetes.io/name=%s in namespace %s to be ready", name, namespace)

	// Define the Deployment resource
	deploymentRes := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Create label selector
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=%s", name)

	// Poll until ready or timeout
	pollInterval := 1 * time.Second
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment with label app.kubernetes.io/name=%s to be ready", name)
		default:
			// List deployments with the label selector
			deploymentList, err := c.clientset.Resource(deploymentRes).Namespace(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					log.Debug().Msgf("Deployment %s not found, waiting...", name)
					time.Sleep(pollInterval)
					continue
				}
				return fmt.Errorf("failed to list deployments with label %s: %w", labelSelector, err)
			}

			// Check if any deployments were found
			if len(deploymentList.Items) == 0 {
				log.Debug().Msgf("No deployments found with label %s, waiting...", labelSelector)
				time.Sleep(pollInterval)
				continue
			}

			// Use the first deployment found (there should typically be only one)
			deployment := &deploymentList.Items[0]
			deploymentName := deployment.GetName()

			// Check if status field exists
			_, found, err := unstructured.NestedMap(deployment.Object, "status")
			if err != nil || !found {
				log.Debug().Msgf("Status field not found in deployment %s, waiting...", deploymentName)
				time.Sleep(pollInterval)
				continue
			}

			// Check if deployment is available
			readyReplicas, found, err := unstructured.NestedInt64(deployment.Object, "status", "readyReplicas")
			if err != nil || !found {
				log.Debug().Msgf("readyReplicas field not found in deployment %s status, waiting...", deploymentName)
				time.Sleep(pollInterval)
				continue
			}

			desiredReplicas, found, err := unstructured.NestedInt64(deployment.Object, "spec", "replicas")
			if err != nil || !found {
				desiredReplicas = 1 // Default to 1 if not specified
				log.Debug().Msgf("replicas field not found in deployment %s spec, assuming default of 1", deploymentName)
			}

			// Get available replicas
			availableReplicas, found, err := unstructured.NestedInt64(deployment.Object, "status", "availableReplicas")
			if err != nil || !found {
				availableReplicas = 0
				log.Debug().Msgf("availableReplicas field not found in deployment %s status, assuming 0", deploymentName)
			}

			// Get updated replicas
			updatedReplicas, found, err := unstructured.NestedInt64(deployment.Object, "status", "updatedReplicas")
			if err != nil || !found {
				updatedReplicas = 0
				log.Debug().Msgf("updatedReplicas field not found in deployment %s status, assuming 0", deploymentName)
			}

			// Log current status
			log.Debug().Msgf("Deployment %s status: %d/%d replicas ready, %d available, %d updated",
				deploymentName, readyReplicas, desiredReplicas, availableReplicas, updatedReplicas)

			// Check if deployment is ready
			if readyReplicas == desiredReplicas && availableReplicas == desiredReplicas {
				conditions, found, err := unstructured.NestedSlice(deployment.Object, "status", "conditions")
				if err != nil || !found {
					log.Debug().Msgf("No conditions found in deployment %s status, continuing to wait...", deploymentName)
					time.Sleep(pollInterval)
					continue
				}

				// Check for Available condition
				isAvailable := false
				for _, conditionUnstructured := range conditions {
					condition, ok := conditionUnstructured.(map[string]interface{})
					if !ok {
						continue
					}

					conditionType, ok := condition["type"].(string)
					if !ok {
						continue
					}

					if conditionType == "Available" {
						status, ok := condition["status"].(string)
						if ok && status == "True" {
							isAvailable = true
							break
						}
					}
				}

				if isAvailable {
					log.Debug().Msgf("Deployment %s is ready and available", deploymentName)
					return nil
				}
			}

			// Sleep before next poll
			time.Sleep(pollInterval)
		}
	}
}

// GetResourceAnnotation retrieves a specific annotation from any Kubernetes resource
func (c *K8sClient) GetResourceAnnotation(gvr schema.GroupVersionResource, namespace string, name string, annotationKey string) (string, error) {
	// Get the resource
	result, err := c.clientset.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get %s '%s' in namespace '%s': %w", gvr.Resource, name, namespace, err)
	}

	// Extract metadata
	metadata, ok := result.Object["metadata"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("failed to extract metadata from %s '%s'", gvr.Resource, name)
	}

	// Extract annotations
	annotations, ok := metadata["annotations"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("no annotations found in %s '%s'", gvr.Resource, name)
	}

	// Get the specific annotation
	value, ok := annotations[annotationKey]
	if !ok {
		return "", fmt.Errorf("annotation '%s' not found in %s '%s'", annotationKey, gvr.Resource, name)
	}

	// Convert to string
	valueStr, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("annotation value is not a string")
	}

	return valueStr, nil
}
