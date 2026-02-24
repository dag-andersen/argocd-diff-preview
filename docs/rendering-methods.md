# Rendering Methods

Argo CD Diff Preview supports three different ways to render application manifests. You can choose the rendering method using the `--render-mode` flag.

## 1. CLI (`cli`) - Default

The `cli` method is the default and most traditional way to generate manifests.

- **How it works:** It executes the `argocd app manifests` command via the Argo CD CLI binary against the ephemeral cluster for every application. 
- **Characteristics:** Highly reliable and guarantees exact parity with what a user sees when running Argo CD locally. However, it is the slowest method due to the overhead of starting a new CLI binary process for each application and waiting for the cluster's reconciliation loop to create and sync the Application objects.

## 2. Server API (`server-api`)

The `server-api` method improves performance by communicating directly with Argo CD's APIs.

- **How it works:** It communicates directly with the Argo CD API server over persistent gRPC connections to request manifest rendering.
- **Characteristics:** Faster than the CLI method because it avoids the overhead of executing a binary for every app. It still relies on the cluster having the application resources applied and waits for the Argo CD application controller to reconcile them.

## 3. Repo Server API (`repo-server-api`) - 🧪 Experimental

The `repo-server-api` method is an experimental fast-path that bypasses the cluster's reconciliation loop entirely.

- **How it works:** It packages your local source files and streams them directly to the Argo CD `repo-server` component via gRPC, asking it to generate the manifests synchronously. 
- **Characteristics:** The fastest method available. No cluster-side Application objects are created, and no polling of the reconciliation loop is needed. 
- **Limitations:** Currently supports only a **single content source** per Application. If your Application uses `spec.sources` with multiple sources that *each produce manifests*, this method will fail and prompt you to switch to `cli` or `server-api`. *(Note: Using multiple `ref` sources used purely for values files alongside a single content source is fully supported).*