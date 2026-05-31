package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/dag-andersen/argocd-diff-preview/pkg/vars"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var argoCDApplicationResource = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

var argoCDApplicationFinalizers = map[string]bool{
	"resources-finalizer.argocd.argoproj.io":            true,
	"resources-finalizer.argocd.argoproj.io/background": true,
	"resources-finalizer.argocd.argoproj.io/foreground": true,
	"pre-delete-finalizer.argocd.argoproj.io":           true,
	"pre-delete-finalizer.argocd.argoproj.io/cleanup":   true,
	"post-delete-finalizer.argocd.argoproj.io":          true,
	"post-delete-finalizer.argocd.argoproj.io/cleanup":  true,
}

// GetArgoCDApplication gets a single ArgoCD application by name
func (c *Client) GetArgoCDApplication(namespace string, name string) (*v1alpha1.Application, error) {
	result, err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Convert unstructured to typed Application
	var app v1alpha1.Application
	resultJSON, err := json.Marshal(result.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal unstructured object: %w", err)
	}

	if err := json.Unmarshal(resultJSON, &app); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to Application: %w", err)
	}

	return &app, nil
}

// SetArgoCDAppRefreshAnnotation sets the refresh annotation on an ArgoCD application to trigger a refresh
func (c *Client) SetArgoCDAppRefreshAnnotation(namespace string, name string) error {
	// Get the current application
	app, err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get application %s: %w", name, err)
	}

	// Get current annotations or create new map
	annotations := app.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Set the refresh annotation
	annotations[v1alpha1.AnnotationKeyRefresh] = "normal"
	app.SetAnnotations(annotations)

	// Update the application
	_, err = c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).Update(context.Background(), app, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update application %s with refresh annotation: %w", name, err)
	}

	return nil
}

// DeleteArgoCDApplication deletes a single ArgoCD application by name
func (c *Client) DeleteArgoCDApplication(namespace string, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("no application name provided")
	}
	if strings.TrimSpace(namespace) == "" {
		return fmt.Errorf("no namespace provided")
	}

	if err := c.removeArgoCDApplicationFinalizers(namespace, name); err != nil {
		return err
	}

	return c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

func (c *Client) removeArgoCDApplicationFinalizers(namespace string, name string) error {
	for attempt := 1; attempt <= 5; attempt++ {
		app, err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to get application %s before deletion: %w", name, err)
		}

		removed, err := c.removeArgoCDApplicationFinalizersFromApp(namespace, app)
		if err == nil || !errors.IsConflict(err) {
			return err
		}
		if !removed {
			return nil
		}

		time.Sleep(time.Duration(attempt) * 100 * time.Millisecond)
	}

	return fmt.Errorf("failed to remove finalizers from application %s before deletion: too many update conflicts", name)
}

func (c *Client) removeArgoCDApplicationFinalizersFromApp(namespace string, app *unstructured.Unstructured) (bool, error) {
	appName := app.GetName()
	currentFinalizers := app.GetFinalizers()
	if len(currentFinalizers) == 0 {
		return false, nil
	}

	log.Info().Msgf("Removing finalizers from application %s", appName)

	patch := []byte(`{"metadata":{"finalizers":null}}`)
	if _, err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).Patch(context.Background(), appName, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		if errors.IsConflict(err) {
			return true, err
		}

		return true, fmt.Errorf("failed to remove finalizers from application %s before deletion: %w", appName, err)
	}

	log.Info().Msgf("Removed finalizers from application %s", appName)
	return true, nil
}

func hasArgoCDApplicationFinalizer(app *unstructured.Unstructured) bool {
	for _, finalizer := range app.GetFinalizers() {
		if argoCDApplicationFinalizers[finalizer] {
			return true
		}
	}

	return false
}

// DeleteAllApplicationsOlderThan deletes all ArgoCD applications older than a given number of minutes
// and matching the given label key
func (c *Client) DeleteAllApplicationsOlderThan(namespace string, minutes int) error {

	log.Info().Msgf("⏳ Deleting applications older than %d minutes", minutes)

	deletedCount := 0

	listOptions := metav1.ListOptions{
		LabelSelector: vars.ArgoCDApplicationLabelKey,
	}

	apps, err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).List(context.Background(), listOptions)
	if err != nil {
		return err
	}

	for _, app := range apps.Items {
		creationTimestamp := app.GetCreationTimestamp()
		timeDiff := time.Since(creationTimestamp.Time)
		if timeDiff.Minutes() > float64(minutes) {
			if _, err := c.removeArgoCDApplicationFinalizersFromApp(namespace, &app); err != nil {
				return err
			}

			if err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).Delete(context.Background(), app.GetName(), metav1.DeleteOptions{}); err != nil {
				return err
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Info().Msgf("🗑️ Deleted %d applications", deletedCount)
	} else {
		log.Info().Msgf("🤖 No applications with the label '%s' were found older than %d minutes", vars.ArgoCDApplicationLabelKey, minutes)
	}

	return nil
}

func (c *Client) DeleteArgoCDApplications(namespace string) error {

	log.Info().Msg("Deleting applications")

	apps, err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, app := range apps.Items {
		if _, err := c.removeArgoCDApplicationFinalizersFromApp(namespace, &app); err != nil {
			log.Error().Err(err).Msgf("Failed to remove finalizers from application %s", app.GetName())
			continue
		}

		if err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).Delete(context.Background(), app.GetName(), metav1.DeleteOptions{}); err != nil {
			log.Error().Err(err).Msgf("Failed to delete application %s", app.GetName())
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
			apps, err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}

			if len(apps.Items) == 0 {
				log.Info().Msg("Deleted applications")
				return nil
			}

			log.Debug().Msgf("Waiting for applications to be deleted: %d", len(apps.Items))

			time.Sleep(1 * time.Second)
		}
	}
}

// RemoveObstructiveFinalizers removes finalizers from applications that would prevent deletion
func (c *Client) RemoveObstructiveFinalizers(namespace string) error {
	log.Debug().Msg("Removing obstructive finalizers from applications")

	// Get ArgoCD applications
	apps, err := c.clientSet.Resource(argoCDApplicationResource).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list applications: %w", err)
	}

	for _, app := range apps.Items {
		if !hasArgoCDApplicationFinalizer(&app) {
			continue
		}

		if _, err := c.removeArgoCDApplicationFinalizersFromApp(namespace, &app); err != nil {
			log.Error().Err(err).Msgf("Failed to remove finalizers from application %s", app.GetName())
		}
	}

	log.Debug().Msg("Finished removing finalizers")
	return nil
}
