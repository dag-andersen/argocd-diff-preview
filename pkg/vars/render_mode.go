package vars

// RenderMode controls how Argo CD renders application manifests.
type RenderMode string

const (
	RenderModeCLI           RenderMode = "cli"
	RenderModeServerAPI     RenderMode = "server-api"
	RenderModeRepoServerAPI RenderMode = "repo-server-api"
)

// IsAPI reports whether the render mode uses the Argo CD API (server-api or repo-server-api)
// rather than the CLI.
func (r RenderMode) IsAPI() bool {
	return r == RenderModeServerAPI || r == RenderModeRepoServerAPI
}
