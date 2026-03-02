package k8s

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/client-go/util/homedir"
)

type Client struct {
	clientSet             *dynamic.DynamicClient
	discoveryClient       discovery.DiscoveryInterface
	cachedDiscoveryClient *disk.CachedDiscoveryClient
	mapper                *restmapper.DeferredDiscoveryRESTMapper
	config                *rest.Config
}

func NewClient(disableClientThrottling bool) (*Client, error) {

	var config *rest.Config

	// try to use kubeconfig
	kubeConfigPath, exists := GetKubeConfigPath()
	if exists {
		log.Debug().Msgf("Using kubeconfig: %s", kubeConfigPath)
		c, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
		if err != nil {
			log.Debug().Err(err).Msg("Failed to create k8s client from kubeconfig")
		} else {
			config = c
			log.Debug().Msg("Using kubeconfig to connect to cluster")
		}
	} else {
		log.Debug().Msgf("No kubeconfig file found at path '%s'", kubeConfigPath)
	}

	// fall back to service account
	if config == nil {
		c, err := rest.InClusterConfig()
		if err != nil {
			log.Debug().Err(err).Msg("Failed to create k8s client from service account")
		} else {
			config = c
			log.Info().Msg("Using service account and environment variables to connect to cluster")
		}
	}

	if config == nil {
		return nil, fmt.Errorf("failed to connect to cluster. No kubeconfig file found at '%s' and no service account credentials detected", kubeConfigPath)
	}

	// Configure rate limiting
	if disableClientThrottling {
		// Disable client-side throttling entirely, relying on API Priority and Fairness (APF)
		config.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()
		log.Debug().Msg("Client-side throttling disabled, relying on API Priority and Fairness")
	} else {
		// Increase QPS and Burst to mitigate client-side throttling on the CI
		config.QPS = 20   // Default is 5
		config.Burst = 40 // Default is 10
	}

	clientSet, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	// Create discovery client for mapping GVK to GVR (same as kubectl api-resources)
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %w", err)
	}

	// Wrap in a cached discovery client for better performance
	httpCacheDir := homedir.HomeDir() + "/.kube/cache"
	cachedDiscoveryClient, err := disk.NewCachedDiscoveryClientForConfig(config, httpCacheDir, "", 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to create cached discovery client: %w", err)
	}

	// Create REST mapper for resource discovery
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscoveryClient)

	return &Client{
		clientSet:             clientSet,
		discoveryClient:       discoveryClient,
		cachedDiscoveryClient: cachedDiscoveryClient,
		mapper:                mapper,
		config:                config,
	}, nil
}

// GetConfig returns the rest.Config used by the client
func (c *Client) GetConfig() *rest.Config {
	return c.config
}

// GetKubeConfigPath returns the path to the kubeconfig file and a boolean indicating if the file exists
func GetKubeConfigPath() (string, bool) {
	// Check KUBECONFIG environment variable first
	if kubeconfigPath := os.Getenv("KUBECONFIG"); kubeconfigPath != "" {
		_, err := os.Stat(kubeconfigPath)
		return kubeconfigPath, err == nil
	}

	// Fall back to default kubeconfig location
	path := clientcmd.RecommendedHomeFile
	_, err := os.Stat(path)
	return path, err == nil
}
