# Demo: Helm + Kustomize Post-Render Naming Mismatch (Issue #355)

This example reproduces the bug reported in
[issue #355](https://github.com/dag-andersen/argocd-diff-preview/issues/355).

## What the bug is

When `argocd-diff-preview` renders an Application it renames it to something
like `b-target-my-app` to keep base- and target-branch names unique.

If the Helm release name is tied to the ArgoCD Application name (as it is
when a CMP runs `helm template --name-template $ARGOCD_APP_NAME`), then the
Deployment rendered by Helm will also carry the mangled name.  Any kustomize
patch that targets the Deployment **by the original name** (`my-app`) will
then fail to find a match, causing `argocd-diff-preview` to error out or
produce an empty/incorrect diff.

## File layout

```
examples/helm-kustomize-postrender/
├── application.yaml          # ArgoCD Application (points at kustomize/)
├── chart/                    # Minimal Helm chart (the "upstream" chart)
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       └── deployment.yaml   # Deployment name = .Release.Name = app name
└── kustomize/                # Kustomize overlay (the "post-renderer")
    ├── kustomization.yaml    # Inflates the helm chart, then patches
    └── patches/
        └── add-labels.yaml   # Strategic-merge patch: targets name "my-app"
```

## How to reproduce

Place `application.yaml` somewhere that `argocd-diff-preview` will scan it
(e.g. the `base-branch/` or `target-branch/` directory).  The tool will
rename the Application internally from `my-app` to something like
`b-target-my-app`, which causes kustomize to fail because the patch in
`patches/add-labels.yaml` still targets the Deployment named `my-app`.

```bash
# Quick local run (adjust branches as needed)
make run-with-go target_branch=main
```

You should see kustomize complain that it cannot find a resource matching
the patch target `my-app` — because the rendered Deployment is now named
`b-target-my-app`.

## Root cause

`pkg/extract/prefix.go` — `addApplicationPrefix()` rewrites
`metadata.name` on the ArgoCD Application object.  For Helm-type
applications this also changes the effective release name, which in turn
changes every resource name that Helm derives from `.Release.Name`.
Kustomize patches that reference those resource names by their original
value then fail to match.
