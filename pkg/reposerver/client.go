package reposerver

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/dag-andersen/argocd-diff-preview/pkg/k8s"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	// repoServerRemotePort is the port the Argo CD repo server pod listens on.
	repoServerRemotePort = 8081
	// repoServerLocalPort is the local port used when port-forwarding to the repo server.
	repoServerLocalPort = 8083
	// chunkSize is the number of bytes sent per gRPC stream message when streaming a tarball.
	chunkSize = 1024
	// maxGRPCMessageSize is the maximum message size for gRPC calls (100 MB).
	maxGRPCMessageSize = 100 * 1024 * 1024

	// maxGenerateRetries is the maximum number of attempts for GenerateManifests before giving up.
	// Retries are triggered by transient transport errors (gRPC Unavailable, or EOF on the port-forward tunnel under high concurrency).
	maxGenerateRetries = 5
	// generateRetryBaseDelay is the initial backoff delay before the first retry.
	generateRetryBaseDelay = 500 * time.Millisecond

	repoServerLabelSelector   = "app.kubernetes.io/part-of=argocd,app.kubernetes.io/component=repo-server"
	repoServerServicePortName = "server"
)

// isRetryableRenderError reports whether an error is a transient transport
// failure worth retrying: gRPC Unavailable, or a bare io.EOF from Send on an aborted stream whose status could not be recovered.
func isRetryableRenderError(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.Unavailable
}

// Client is a gRPC client for the Argo CD repo server.
// It manages an optional port-forward to the repo server so that the caller
// does not need to worry about cluster networking.
type Client struct {
	k8sClient *k8s.Client
	namespace string

	// address is the host:port of the repo server (may be a port-forwarded local address).
	address string
	// disableTLS controls whether to connect without TLS (plain-text).
	disableTLS bool
	// insecureSkipVerify allows self-signed certificates.
	insecureSkipVerify bool

	portForwardMutex    sync.Mutex
	portForwardActive   bool
	portForwardStopChan chan struct{}

	tgzCache *tgzCache
}

// NewClient creates a new repo server Client that port-forwards to the Argo CD
// repo server running inside the cluster.
//
// namespace is the Kubernetes namespace where Argo CD is installed.
// k8sClient is used to establish the port-forward.
func NewClient(k8sClient *k8s.Client, namespace string) *Client {
	return &Client{
		k8sClient:          k8sClient,
		namespace:          namespace,
		address:            fmt.Sprintf("localhost:%d", repoServerLocalPort),
		disableTLS:         false,
		insecureSkipVerify: true, // self-signed cert inside the cluster
		tgzCache:           newTgzCache(),
	}
}

// NewClientWithAddress creates a Client that connects directly to the given address
// without setting up a port-forward. Useful when the repo server is reachable
// from the current host (e.g. inside the cluster, or an already-established tunnel).
func NewClientWithAddress(address string, disableTLS bool, insecureSkipVerify bool) *Client {
	return &Client{
		address:            address,
		disableTLS:         disableTLS,
		insecureSkipVerify: insecureSkipVerify,
		tgzCache:           newTgzCache(),
	}
}

// EnsureConnection makes the repo server reachable. In-cluster runs use the
// repo-server Service DNS name directly so Kubernetes can load-balance between
// pods. External runs keep the existing port-forward behavior.
func (c *Client) EnsureConnection() error {
	if c.k8sClient != nil && c.k8sClient.IsInCluster() {
		address, err := c.k8sClient.GetServiceAddressByLabel(c.namespace, repoServerLabelSelector, repoServerServicePortName, repoServerRemotePort)
		if err != nil {
			return fmt.Errorf("failed to find Argo CD repo server service address (label: %s): %w", repoServerLabelSelector, err)
		}

		c.address = address
		log.Debug().Str("address", address).Msg("Using Argo CD repo server service address")
		return nil
	}

	return c.EnsurePortForward()
}

// EnsurePortForward starts a port-forward to the Argo CD repo server if one is
// not already running. It is idempotent and safe to call concurrently.
func (c *Client) EnsurePortForward() error {
	if c.k8sClient == nil {
		return fmt.Errorf("no k8s client configured - cannot port-forward")
	}

	c.portForwardMutex.Lock()

	if c.portForwardActive {
		c.portForwardMutex.Unlock()
		log.Debug().Msg("Port forward to Argo CD repo server is already active, reusing existing connection")
		return nil
	}

	log.Debug().Msg("🔌 Setting up port forward to Argo CD repo server...")

	readyChan := make(chan struct{}, 1)
	stopChan := make(chan struct{}, 1)

	serviceName, err := c.k8sClient.GetServiceNameByLabel(c.namespace, repoServerLabelSelector)
	if err != nil {
		c.portForwardMutex.Unlock()
		return fmt.Errorf("failed to find Argo CD repo server service (label: %s): %w", repoServerLabelSelector, err)
	}

	log.Debug().Msgf("Starting port forward from localhost:%d to %s:%d in namespace %s",
		repoServerLocalPort, serviceName, repoServerRemotePort, c.namespace)

	if err := c.k8sClient.PortForwardToService(c.namespace, serviceName, repoServerLocalPort, repoServerRemotePort, readyChan, stopChan); err != nil {
		c.portForwardMutex.Unlock()
		return fmt.Errorf("failed to set up port forward to repo server: %w", err)
	}

	c.portForwardActive = true
	c.portForwardStopChan = stopChan
	c.portForwardMutex.Unlock()

	log.Debug().Msg("Waiting for repo server port forward to be ready...")
	select {
	case <-readyChan:
		log.Debug().Msgf("🔌 Repo server port forward ready: localhost:%d -> %s:%d",
			repoServerLocalPort, serviceName, repoServerRemotePort)
		return nil
	case <-time.After(30 * time.Second):
		c.portForwardMutex.Lock()
		c.portForwardActive = false
		if c.portForwardStopChan != nil {
			close(c.portForwardStopChan)
			c.portForwardStopChan = nil
		}
		c.portForwardMutex.Unlock()
		return fmt.Errorf("timeout waiting for repo server port forward to be ready")
	}
}

// Cleanup stops the port-forward if one was started by this client.
func (c *Client) Cleanup() {
	if c.tgzCache != nil {
		c.tgzCache.Cleanup()
	}

	c.portForwardMutex.Lock()
	defer c.portForwardMutex.Unlock()

	if c.portForwardActive && c.portForwardStopChan != nil {
		log.Debug().Msg("Stopping port forward to Argo CD repo server...")
		close(c.portForwardStopChan)
		c.portForwardActive = false
		c.portForwardStopChan = nil
	}
}

// newGRPCConn opens a new gRPC connection to the repo server.
// Creating a new connection per request forces the Kubernetes load-balancer to
// pick a potentially different repo-server pod, spreading the load evenly.
func (c *Client) newGRPCConn() (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(maxGRPCMessageSize),
			grpc.MaxCallSendMsgSize(maxGRPCMessageSize),
		),
	}

	if c.disableTLS {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsCfg := &tls.Config{InsecureSkipVerify: c.insecureSkipVerify} //nolint:gosec
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}

	conn, err := grpc.NewClient(c.address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to repo server at %s: %w", c.address, err)
	}
	return conn, nil
}

// GenerateManifests renders Kubernetes manifests for the given application source
// by streaming the source files to the Argo CD repo server via the
// GenerateManifestWithFiles gRPC endpoint.
//
// appDir is the local directory that contains the rendered/checked-out source
// files to stream. It must exist and be readable. The directory is compressed
// into a .tar.gz archive and streamed chunk-by-chunk to the repo server.
//
// request is a fully-populated ManifestRequest that describes the application,
// its source, cluster API versions, Helm settings, etc.
//
// Returns the list of rendered manifest strings (one per Kubernetes resource,
// serialised as JSON by the repo server).
//
// Transient gRPC Unavailable errors (e.g. EOF on the kubectl port-forward
// tunnel under high concurrency) are retried with exponential back-off up to
// maxGenerateRetries times before the error is propagated to the caller.
func (c *Client) GenerateManifests(ctx context.Context, appDir string, request *repoapiclient.ManifestRequest) ([]string, error) {
	log.Debug().
		Str("app", request.AppName).
		Str("dir", appDir).
		Msg("Opening application directory archive for repo server")

	// Cached per directory: Applications routinely share a source path, so
	// compressing per Application repeats identical work. Closed, not deleted -
	// the archive is reused by the other Applications on this path.
	archive, err := c.tgzCache.openCachedTgz(appDir)
	if err != nil {
		return nil, fmt.Errorf("failed to compress app directory %q: %w", appDir, err)
	}
	defer func() {
		if err := archive.file.Close(); err != nil {
			log.Debug().Err(err).Str("app", request.AppName).Msg("Failed to close archive handle")
		}
	}()

	log.Debug().
		Str("app", request.AppName).
		Str("sourcePath", request.ApplicationSource.Path).
		Str("streamDir", appDir).
		Int("sourceFiles", archive.files).
		Int64("archiveBytes", archive.bytes).
		Bool("archiveCacheHit", archive.cacheHit).
		Dur("compressionDuration", archive.compressionDuration).
		Dur("archiveOpenDuration", archive.cacheWaitDuration).
		Str("checksum", archive.checksum).
		Msg("📦 Repo-server stream staging metrics")

	var lastErr error
	for attempt := 1; attempt <= maxGenerateRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled before attempt %d: %w", attempt, err)
		}

		if attempt > 1 {
			delay := generateRetryBaseDelay * time.Duration(1<<uint(attempt-2)) // 500ms, 1s, 2s, 4s …
			log.Debug().
				Str("app", request.AppName).
				Int("attempt", attempt).
				Dur("delay", delay).
				Err(lastErr).
				Msg("Retrying GenerateManifests after transient error")

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled while waiting to retry: %w", ctx.Err())
			}

			// Seek the tgz file back to the beginning for the next attempt.
			if _, seekErr := archive.file.Seek(0, io.SeekStart); seekErr != nil {
				return nil, fmt.Errorf("failed to seek tarball for retry: %w", seekErr)
			}
		}

		manifests, err := c.generateManifestsOnce(ctx, archive.file, archive.checksum, request)
		if err == nil {
			return manifests, nil
		}

		// Only retry transient transport errors.
		if isRetryableRenderError(err) {
			lastErr = err
			log.Warn().
				Str("app", request.AppName).
				Int("attempt", attempt).
				Err(err).
				Msg("⚠️ Transient transport error from repo server; will retry")
			continue
		}

		// Non-retryable error - return immediately.
		return nil, err
	}

	return nil, fmt.Errorf("repo server unavailable after %d attempts: %w", maxGenerateRetries, lastErr)
}

// abortReason resolves a Send error to the stream's real failure:
// Send on an aborted stream returns a bare io.EOF, and the actual status (which decides retryability) is only surfaced by CloseAndRecv.
func abortReason(stream repoapiclient.RepoServerService_GenerateManifestWithFilesClient, sendErr error) error {
	if !errors.Is(sendErr, io.EOF) {
		return sendErr
	}
	_, recvErr := stream.CloseAndRecv()
	if recvErr == nil || errors.Is(recvErr, io.EOF) {
		return sendErr // no better information; io.EOF stays retryable
	}
	return fmt.Errorf("stream aborted: %w (send: %v)", recvErr, sendErr)
}

// generateManifestsOnce performs a single GenerateManifestWithFiles call.
// tgzFile must be seeked to its start before each call.
func (c *Client) generateManifestsOnce(ctx context.Context, tgzFile *os.File, checksum string, request *repoapiclient.ManifestRequest) ([]string, error) {
	conn, err := c.newGRPCConn()
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close gRPC connection to repo server")
		}
	}()

	repoClient := repoapiclient.NewRepoServerServiceClient(conn)

	log.Debug().Str("app", request.AppName).Msg("Opening GenerateManifestWithFiles stream")
	stream, err := repoClient.GenerateManifestWithFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open GenerateManifestWithFiles stream: %w", err)
	}
	// Note: do NOT defer stream.CloseSend() here. CloseAndRecv() (step 4 below)
	// calls CloseSend() internally as its first action, so a deferred call would
	// fire on an already-closed stream and emit a spurious warn log on every
	// successful render.

	// 1. Send the manifest request metadata.
	log.Debug().Str("app", request.AppName).Msg("Sending manifest request to repo server")
	if err := stream.Send(&repoapiclient.ManifestRequestWithFiles{
		Part: &repoapiclient.ManifestRequestWithFiles_Request{
			Request: request,
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to send manifest request: %w", abortReason(stream, err))
	}

	// 2. Send the tarball checksum.
	log.Debug().Str("app", request.AppName).Msg("Sending file metadata to repo server")
	if err := stream.Send(&repoapiclient.ManifestRequestWithFiles{
		Part: &repoapiclient.ManifestRequestWithFiles_Metadata{
			Metadata: &repoapiclient.ManifestFileMetadata{
				Checksum: checksum,
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to send file metadata: %w", abortReason(stream, err))
	}

	// 3. Stream the tarball contents in 1 KiB chunks.
	log.Debug().Str("app", request.AppName).Msg("Streaming tarball to repo server")
	if err := sendFileChunks(ctx, stream, tgzFile); err != nil {
		return nil, fmt.Errorf("failed to stream tarball: %w", abortReason(stream, err))
	}

	// 4. Signal that we're done and receive the rendered manifests.
	log.Debug().Str("app", request.AppName).Msg("Waiting for manifest response from repo server")
	response, err := stream.CloseAndRecv()
	if err != nil {
		return nil, fmt.Errorf("failed to receive manifest response: %w", err)
	}

	log.Debug().
		Str("app", request.AppName).
		Int("manifests", len(response.Manifests)).
		Msg("Received manifests from repo server")

	return response.Manifests, nil
}

// GenerateManifestsRemote renders manifests for an application whose source
// lives on a remote server (e.g. an external Helm chart registry). Unlike
// GenerateManifests, no local files are streamed; the repo server fetches
// the chart directly from the registry using the RepoURL / TargetRevision /
// Chart fields in the ManifestRequest.
//
// This method uses the unary GenerateManifest RPC (not the streaming
// GenerateManifestWithFiles RPC).
//
// Transient gRPC Unavailable errors are retried with the same exponential
// back-off as GenerateManifests.
func (c *Client) GenerateManifestsRemote(ctx context.Context, request *repoapiclient.ManifestRequest) ([]string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxGenerateRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled before attempt %d: %w", attempt, err)
		}

		if attempt > 1 {
			delay := generateRetryBaseDelay * time.Duration(1<<uint(attempt-2)) // 500ms, 1s, 2s, 4s …
			log.Debug().
				Str("app", request.AppName).
				Int("attempt", attempt).
				Dur("delay", delay).
				Err(lastErr).
				Msg("Retrying GenerateManifestsRemote after transient error")
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled while waiting to retry: %w", ctx.Err())
			}
		}

		conn, err := c.newGRPCConn()
		if err != nil {
			return nil, err
		}

		repoClient := repoapiclient.NewRepoServerServiceClient(conn)
		response, err := repoClient.GenerateManifest(ctx, request)
		if closeErr := conn.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("Failed to close gRPC connection to repo server")
		}

		if err == nil {
			log.Debug().
				Str("app", request.AppName).
				Int("manifests", len(response.Manifests)).
				Msg("Received remote manifests from repo server")
			return response.Manifests, nil
		}

		if isRetryableRenderError(err) {
			lastErr = err
			log.Warn().
				Str("app", request.AppName).
				Int("attempt", attempt).
				Err(err).
				Msg("⚠️ Transient transport error from repo server; will retry")
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("repo server unavailable after %d attempts: %w", maxGenerateRetries, lastErr)
}

// GenerateManifestsForApp is a higher-level helper that compresses the given
// appDir and sends it to the repo server. It constructs a minimal
// ManifestRequest from the provided Application object.
//
// This is intentionally kept simple - callers that need fine-grained control
// over Helm repos, API versions, project settings, etc. should build the
// ManifestRequest themselves and call GenerateManifests directly.
func (c *Client) GenerateManifestsForApp(
	ctx context.Context,
	appDir string,
	app *v1alpha1.Application,
	appLabelKey string,
	kubeVersion string,
	apiVersions []string,
) ([]string, error) {
	source := app.Spec.GetSource()

	request := &repoapiclient.ManifestRequest{
		Repo:               &v1alpha1.Repository{Repo: source.RepoURL},
		Revision:           source.TargetRevision,
		AppLabelKey:        appLabelKey,
		AppName:            app.Name,
		Namespace:          app.Spec.Destination.Namespace,
		ApplicationSource:  &source,
		KubeVersion:        kubeVersion,
		ApiVersions:        apiVersions,
		HasMultipleSources: app.Spec.HasMultipleSources(),
	}

	return c.GenerateManifests(ctx, appDir, request)
}

// sender is the subset of the gRPC client stream interface we need for sending chunks.
type sender interface {
	Send(*repoapiclient.ManifestRequestWithFiles) error
}

// sendFileChunks reads the given file and sends its contents to the gRPC stream
// in chunkSize-byte pieces.
func sendFileChunks(ctx context.Context, s sender, file *os.File) error {
	reader := bufio.NewReader(file)
	buf := make([]byte, chunkSize)

	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled while streaming: %w", err)
		}

		n, readErr := reader.Read(buf)
		if n > 0 {
			msg := &repoapiclient.ManifestRequestWithFiles{
				Part: &repoapiclient.ManifestRequestWithFiles_Chunk{
					Chunk: &repoapiclient.ManifestFileChunk{
						Chunk: buf[:n],
					},
				},
			}
			if sendErr := s.Send(msg); sendErr != nil {
				return fmt.Errorf("failed to send chunk: %w", sendErr)
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading tarball: %w", readErr)
		}
	}
}
