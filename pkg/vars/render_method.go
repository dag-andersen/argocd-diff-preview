package vars

// RenderMethod controls how Argo CD renders application manifests.
type RenderMethod string

const (
	RenderMethodCLI           RenderMethod = "cli"
	RenderMethodServerAPI     RenderMethod = "server-api"
	RenderMethodRepoServerAPI RenderMethod = "repo-server-api"
)
