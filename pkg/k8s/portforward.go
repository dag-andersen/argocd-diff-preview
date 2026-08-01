package k8s

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PortForwardToPod sets up a port forward to a pod in the specified namespace
// Returns a channel that will be closed when the port forward is ready, a stop channel to terminate the forward,
// and an error if the setup fails
func (c *Client) PortForwardToPod(namespace, podName string, localPort, remotePort int, readyChan chan struct{}, stopChan chan struct{}) error {
	// Create a Kubernetes clientset for pod operations
	clientset, err := kubernetes.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Build the URL for port forwarding
	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	// Create both WebSocket and SPDY dialers for compatibility
	// GKE Connect Gateway requires WebSocket with SPDY protocol (Sec-WebSocket-Protocol: SPDY/3.1)
	// Traditional clusters accept direct SPDY upgrade without WebSocket wrapping

	// Create SPDY dialer (fallback for clusters)
	transport, upgrader, err := spdy.RoundTripperFor(c.config)
	if err != nil {
		return fmt.Errorf("failed to create SPDY round tripper: %w", err)
	}
	spdyDialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	// Create WebSocket tunneling dialer (required for GKE Connect Gateway)
	websocketDialer, err := portforward.NewSPDYOverWebsocketDialer(req.URL(), c.config)
	if err != nil {
		return fmt.Errorf("failed to create WebSocket dialer: %w", err)
	}

	// Use fallback pattern: try WebSocket first, fallback to SPDY on upgrade failures
	dialer := portforward.NewFallbackDialer(
		websocketDialer,
		spdyDialer,
		func(err error) bool {
			shouldFallback := httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
			if shouldFallback {
				log.Debug().Err(err).Msg("WebSocket upgrade failed, falling back to SPDY")
			}
			return shouldFallback
		},
	)

	// Set up port forwarding
	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	// Use io.Discard instead of os.Stdout/os.Stderr to avoid cluttering logs
	// Create port forwarder
	fw, err := portforward.New(dialer, ports, stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %w", err)
	}

	// Create error channel to capture ForwardPorts errors
	errChan := make(chan error, 1)

	// Start port forwarding in a goroutine
	go func() {
		if err := fw.ForwardPorts(); err != nil {
			log.Error().Err(err).Msg("Port forward failed")
			errChan <- err
		}
	}()

	return nil
}

// GetServiceNameByLabel finds a service in the namespace with the specified label selector
// Returns the name of the first matching service, or an error if no service is found
func (c *Client) GetServiceNameByLabel(namespace, labelSelector string) (string, error) {
	service, err := c.GetServiceByLabel(namespace, labelSelector)
	if err != nil {
		return "", err
	}

	serviceName := service.Name
	log.Debug().Msgf("Found service %s with label %s in namespace %s", serviceName, labelSelector, namespace)
	return serviceName, nil
}

// GetServiceByLabel finds a service in the namespace with the specified label selector.
// If multiple services match, it returns the lexicographically first service name so
// address discovery is deterministic.
func (c *Client) GetServiceByLabel(namespace, labelSelector string) (*corev1.Service, error) {
	clientset, err := kubernetes.NewForConfig(c.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	services, err := clientset.CoreV1().Services(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list services with label %s: %w", labelSelector, err)
	}

	if len(services.Items) == 0 {
		return nil, fmt.Errorf("no services found with label %s in namespace %s", labelSelector, namespace)
	}

	sort.Slice(services.Items, func(i, j int) bool {
		return services.Items[i].Name < services.Items[j].Name
	})

	return &services.Items[0], nil
}

// GetServiceAddressByLabel builds an in-cluster DNS address for a service
// selected by label, using the preferred named port when available, then the
// preferred numeric port, then the first service port.
func (c *Client) GetServiceAddressByLabel(namespace, labelSelector, preferredPortName string, preferredPort int32) (string, error) {
	service, err := c.GetServiceByLabel(namespace, labelSelector)
	if err != nil {
		return "", err
	}

	port, err := chooseServicePort(service, preferredPortName, preferredPort)
	if err != nil {
		return "", err
	}

	address := serviceDNSAddress(service.Name, namespace, port)
	log.Debug().Msgf("Found service address %s with label %s in namespace %s", address, labelSelector, namespace)
	return address, nil
}

func chooseServicePort(service *corev1.Service, preferredPortName string, preferredPort int32) (int32, error) {
	if service == nil {
		return 0, fmt.Errorf("service is nil")
	}

	if len(service.Spec.Ports) == 0 {
		return 0, fmt.Errorf("service %s has no ports", service.Name)
	}

	if preferredPortName != "" {
		for _, port := range service.Spec.Ports {
			if port.Name == preferredPortName {
				return port.Port, nil
			}
		}
	}

	if preferredPort > 0 {
		for _, port := range service.Spec.Ports {
			if port.Port == preferredPort {
				return port.Port, nil
			}
		}
	}

	return service.Spec.Ports[0].Port, nil
}

func serviceDNSAddress(serviceName, namespace string, port int32) string {
	return fmt.Sprintf("%s.%s.svc:%d", serviceName, namespace, port)
}

// PortForwardToService sets up a port forward to a service by finding a pod for that service
// Returns a channel that will be closed when the port forward is ready, a stop channel to terminate the forward,
// and an error if the setup fails
func (c *Client) PortForwardToService(namespace, serviceName string, localPort, remotePort int, readyChan chan struct{}, stopChan chan struct{}) error {
	// Create a Kubernetes clientset
	clientset, err := kubernetes.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Get the service to find its selector
	service, err := clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service %s: %w", serviceName, err)
	}

	// Build label selector from service selector
	selector := metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: service.Spec.Selector})

	// Find pods matching the service selector
	pods, err := clientset.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods for service %s: %w", serviceName, err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found for service %s with selector %s", serviceName, selector)
	}

	// Find a running pod
	var targetPod *corev1.Pod
	for i := range pods.Items {
		if pods.Items[i].Status.Phase == corev1.PodRunning {
			targetPod = &pods.Items[i]
			break
		}
	}

	if targetPod == nil {
		return fmt.Errorf("no running pods found for service %s", serviceName)
	}

	log.Debug().Msgf("Using pod %s for port forwarding to service %s", targetPod.Name, serviceName)

	// Forward to the selected pod
	return c.PortForwardToPod(namespace, targetPod.Name, localPort, remotePort, readyChan, stopChan)
}
