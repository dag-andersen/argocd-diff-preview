# Rendering Methods

Argo CD Diff Preview supports three different ways to render application manifests. You can choose the rendering method using the `--render-method` flag.

## 1. CLI (`cli`)

The `cli` method is the most traditional way to generate manifests.

- **How it works:** It executes the `argocd app manifests` command via the Argo CD CLI binary against the ephemeral cluster for every application. 
- **Characteristics:** Highly reliable, because it authenticates and manages connections automatically. However, it is more restrictive, as it will throw the exact same errors users are accustomed to seeing in the Argo CD UI. It is also slower because it must wait for all applications to reach the "OutOfSync" state before it can render the manifests.
- **Note:** The Argo CD CLI binary is **not** included in the Docker image. If you want to use this render method, you must install the `argocd` binary yourself (either by extending the Docker image or by running the `argocd-diff-preview` binary directly in your pipeline).

## 2. Server API (`server-api`) - Default

The `server-api` method is the default rendering method. It improves performance by communicating directly with Argo CD's APIs.

- **How it works:** It communicates directly with the Argo CD API server over persistent gRPC connections to request manifest rendering.
- **Characteristics:** Faster than the CLI method because it does not wait for the Argo CD Application controller reconciliation loop. It also provides more detailed error messages when rendering fails, since it surfaces errors directly from the API server. However, it is slightly more fragile because the tool must manage connections and authentication manually.
- **Lockdown mode:** Compatible with [lockdown mode](reusing-clusters/lockdown-mode.md) (namespace-scoped Argo CD).

## 3. Repo Server API (`repo-server-api`) - 🧪 Experimental

The `repo-server-api` method is an experimental fast-path that bypasses the cluster's reconciliation loop entirely.

- **How it works:** It connects directly to the Argo CD `repo-server` component via gRPC, asking it to generate the manifests synchronously. 
- **Characteristics:** The fastest method available. No cluster-side Application objects are created, and no polling of the reconciliation loop is needed. 
- **Lockdown mode:** Compatible with [lockdown mode](reusing-clusters/lockdown-mode.md) (namespace-scoped Argo CD).

### Limit streamed Kustomize files

For local Kustomize sources, `repo-server-api` uses the Application's
`argocd.argoproj.io/manifest-generate-paths` annotation to limit the streamed
archive. The source directory is always included. Separate additional paths with
semicolons; paths starting with `/` are relative to the repository root, and
other paths are relative to the source directory. Include all external bases,
components, patches, and value files required by the build.

For example, a source at `apps/example` can use `.;/base/shared;/config`.
The archive keeps the repository layout, so relative references still work.
Applications without this annotation, and sources with retained symlinks, use
the full checkout. Full-checkout archives are shared across applications.
Explicit Helm, plugin, and directory sources are not narrowed. Sources with
applicable `.argocd-source*.yaml` override files also keep the full checkout,
because these files can change the renderer.
This does not change application selection or remote rendering.
