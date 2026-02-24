package vars

// RenderMode controls how Argo CD renders application manifests.
type RenderMode string

const (
	RenderModeCLI           RenderMode = "cli"
	RenderModeServerAPI     RenderMode = "server-api"
	RenderModeRepoServerAPI RenderMode = "repo-server-api"
)
