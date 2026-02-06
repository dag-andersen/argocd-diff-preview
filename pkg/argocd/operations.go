package argocd

import (
	"fmt"
	"sync"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	// remotePort is the port that the ArgoCD server pod listens on
	remotePort = 8080
	localPort  = 8081
)

// Operations defines the interface for ArgoCD operations.
// There are two implementations: CLIOperations and APIOperations.
// The implementation is chosen at construction time based on the useAPI flag.
type Operations interface {
	// Login authenticates with ArgoCD and caches credentials for future calls.
	// If a token was provided during construction, this method will skip authentication
	// and use the provided token instead.
	Login() error

	// AppsetGenerate generates applications from an ApplicationSet file
	AppsetGenerate(appSetPath string) (string, error)

	// GetManifests returns the manifests for an application.
	// Returns: (manifests, appExists, error)
	GetManifests(appName string) ([]unstructured.Unstructured, bool, error)

	// CheckVersionCompatibility checks if the client/library version is compatible
	// with the ArgoCD server version
	CheckVersionCompatibility() error

	// AddSourceNamespaceToDefaultAppProject adds "*" to the sourceNamespaces
	// of the default AppProject to allow applications from any namespace
	AddSourceNamespaceToDefaultAppProject() error

	// Cleanup performs any necessary cleanup (e.g., stopping port forwards)
	Cleanup()

	// IsExpectedError checks if an error message is expected for this mode.
	// In API mode, certain errors are expected when running with 'createClusterRoles: false'.
	// In CLI mode, this always returns false.
	// Returns: (isExpected, reason)
	IsExpectedError(errorMessage string) (bool, string)
}

// apiConnection holds the state for API mode connections
type apiConnection struct {
	apiServerURL         string // Constructed API server URL (e.g., "http://localhost:8081")
	authToken            string // Cached authentication token
	portForwardActive    bool
	portForwardMutex     sync.Mutex
	portForwardStopChan  chan struct{}
	portForwardLocalPort int // Local port for port forwarding (e.g., 8081)
}

// NewOperations creates the appropriate Operations implementation based on the useAPI flag.
// If useAPI is true, returns an APIOperations instance.
// If useAPI is false, returns a CLIOperations instance.
// The authToken parameter is optional - if provided, it will be used instead of
// authenticating with the ArgoCD server during Login().
func NewOperations(useAPI bool, k8sClient *utils.K8sClient, namespace string, loginOptions string, authToken string) Operations {
	if useAPI {
		return &APIOperations{
			k8sClient: k8sClient,
			namespace: namespace,
			connection: &apiConnection{
				portForwardLocalPort: localPort,
				apiServerURL:         fmt.Sprintf("http://localhost:%d", localPort),
				authToken:            authToken,
			},
		}
	}
	return &CLIOperations{
		k8sClient:    k8sClient,
		namespace:    namespace,
		loginOptions: loginOptions,
		authToken:    authToken,
	}
}
