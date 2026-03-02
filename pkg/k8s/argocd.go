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
	"k8s.io/apimachinery/pkg/runtime/schema"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetArgoCDApplication gets a single ArgoCD application by name
func (c *Client) GetArgoCDApplication(namespace string, name string) (*v1alpha1.Application, error) {
	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	result, err := c.clientSet.Resource(applicationRes).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
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
	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	// Get the current application
	app, err := c.clientSet.Resource(applicationRes).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
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
	_, err = c.clientSet.Resource(applicationRes).Namespace(namespace).Update(context.Background(), app, metav1.UpdateOptions{})
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

	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	return c.clientSet.Resource(applicationRes).Namespace(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
}

// DeleteAllApplicationsOlderThan deletes all ArgoCD applications older than a given number of minutes
// and matching the given label key
func (c *Client) DeleteAllApplicationsOlderThan(namespace string, minutes int) error {

	log.Info().Msgf("Deleting applications older than %d minutes", minutes)

	deletedCount := 0

	listOptions := metav1.ListOptions{
		LabelSelector: vars.ArgoCDApplicationLabelKey,
	}

	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	apps, err := c.clientSet.Resource(applicationRes).Namespace(namespace).List(context.Background(), listOptions)
	if err != nil {
		return err
	}

	for _, app := range apps.Items {
		creationTimestamp := app.GetCreationTimestamp()
		timeDiff := time.Since(creationTimestamp.Time)
		if timeDiff.Minutes() > float64(minutes) {
			err := c.clientSet.Resource(applicationRes).Namespace(namespace).Delete(context.Background(), app.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		log.Info().Msgf("Deleted %d applications", deletedCount)
	} else {
		log.Info().Msgf("No applications with the label '%s' were found older than %d minutes", vars.ArgoCDApplicationLabelKey, minutes)
	}

	return nil
}

func (c *Client) DeleteArgoCDApplications(namespace string) error {

	log.Info().Msg("Deleting applications")

	// Remove obstructive finalizers
	if err := c.RemoveObstructiveFinalizers(namespace); err != nil {
		return fmt.Errorf("failed to remove obstructive finalizers: %w", err)
	}

	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}

	apps, err := c.clientSet.Resource(applicationRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, app := range apps.Items {
		err := c.clientSet.Resource(applicationRes).Namespace(namespace).Delete(context.Background(), app.GetName(), metav1.DeleteOptions{})
		if err != nil {
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
			apps, err := c.clientSet.Resource(applicationRes).Namespace(namespace).List(ctx, metav1.ListOptions{})
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

	// List of obstructiveFinalizers that prevent deletion of applications
	obstructiveFinalizers := []string{
		"post-delete-finalizer.argocd.argoproj.io",
		"post-delete-finalizer.argoproj.io/cleanup",
	}

	log.Debug().Msg("Removing obstructive finalizers from applications")

	// Get ArgoCD applications
	applicationRes := schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	apps, err := c.clientSet.Resource(applicationRes).Namespace(namespace).List(context.Background(), metav1.ListOptions{})
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

		log.Info().Msgf("Removing obstructive finalizers from application %s", appName)

		app.SetFinalizers(nil)
		_, err := c.clientSet.Resource(applicationRes).Namespace(namespace).Update(
			context.Background(),
			&app,
			metav1.UpdateOptions{},
		)

		if err != nil {
			log.Error().Err(err).Msgf("Failed to update finalizers for application %s", appName)
		} else {
			log.Info().Msgf("Removed finalizers from application %s", appName)
		}
	}

	log.Debug().Msg("Finished removing finalizers")
	return nil
}
