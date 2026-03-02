package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// WaitForDeploymentReady waits for a deployment to be available
// Uses the provided label selector to find the deployment (e.g., "app.kubernetes.io/component=server")
func (c *Client) WaitForDeploymentReady(namespace, labelSelector string, timeoutSeconds int) error {
	log.Debug().Msgf("Waiting for deployment with labels '%s' in namespace %s to be ready", labelSelector, namespace)

	// Define the Deployment resource
	deploymentRes := schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	// Poll until ready or timeout
	pollInterval := 1 * time.Second
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment with labels '%s' to be ready", labelSelector)
		default:
			// List deployments with the label selector
			deploymentList, err := c.clientSet.Resource(deploymentRes).Namespace(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector,
			})
			if err != nil {
				if strings.Contains(err.Error(), "not found") {
					log.Debug().Msgf("Deployment with labels '%s' not found, waiting...", labelSelector)
					time.Sleep(pollInterval)
					continue
				}
				return fmt.Errorf("failed to list deployments with labels '%s': %w", labelSelector, err)
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
					condition, ok := conditionUnstructured.(map[string]any)
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
