# Frequently Asked Questions (FAQ)

---

### Does the tool work with `ApplicationSets`?

**Short answer:** Yes.

**Longer Answer:** Yes, but _how well it works_ depends on the complexity of your generators.

- **List generator**:
  Yes, no issues reported

- **Cluster generator**:
  Yes, but you have to add the ClusterSecrets to the `secrets` folder. Read more [here](./cluster-generator.md)

- **Git generator**:
  Yes, no issues reported

- **Matrix generator**:
  Yes, no issues reported

- **Merge generator**:
  Yes, no issues reported

- **Plugin generator**: 
  Should work, but not tested. Read more on how to install a plugin [here](./getting-started/custom-argo-cd-installation.md#argo-cd-config-management-plugins-cmp). Related Issue: [#40](https://github.com/dag-andersen/argocd-diff-preview/issues/40)

- **Pull Request generator**:
   Should work, but not tested.

- **SCM Provider generator**:
  Not tested

- **Cluster Decision Resource generator**:
  Not tested

---

### Does it work with Config Management Plugins (CMP)

Yes. More info [here](./getting-started/custom-argo-cd-installation.md#argo-cd-config-management-plugins-cmp)

---

### Does it work with Git providers other than GitHub and GitLab?

**Short answer:** Yes.

**Longer answer:** In theory, yes, but not all providers have been tested. If you are using a different Git provider and encounter issues, please open an issue in the repository. Additionally, if you have successfully used the tool with a different provider, consider contributing to the documentation so others can benefit from a working example ❤️

Relevant issue: [#94](https://github.com/dag-andersen/argocd-diff-preview/issues/94)

---

### Why not compare the PR branch to the live cluster state?

Comparing to live cluster state introduces non-determinism and ambiguity. The cluster state can change between runs, making the output unreliable and hard to trust during code review. Read more about this in the [Design Philosophy](./design-philosophy.md).

---

### Does it work with the App of Apps Pattern?

**Short answer:** Yes.

**Longer answer:** The tool scans your repository for all resources with `kind: Application` or `kind: ApplicationSet` and renders them. If your child Applications exist as plain YAML manifests in the repository, the tool works out of the box - no extra flags needed.

If your child Applications are *generated dynamically* at render time (e.g. a root Application that deploys a Helm chart which templates child Application manifests), the tool cannot see them by default. In that case you can either:

1. **Pre-render** the Application manifests in your CI pipeline before running `argocd-diff-preview`. See [Helm/Kustomize generated Argo CD applications](./generated-applications.md).
2. **Use `--traverse-app-of-apps`** (🧪 experimental) to let the tool discover and render child Applications automatically at runtime.

More info: [App of Apps docs](./app-of-apps.md)

Relevant issues: [#41](https://github.com/dag-andersen/argocd-diff-preview/issues/41), [#75](https://github.com/dag-andersen/argocd-diff-preview/issues/75), [#200](https://github.com/dag-andersen/argocd-diff-preview/issues/200), [#381](https://github.com/dag-andersen/argocd-diff-preview/issues/381)

#### Why can't the tool automatically render my Helm Charts or Kustomize templates?

The tool intentionally avoids guessing how to render Helm charts and Kustomize templates - the configurations are ambiguous by nature, and incorrect assumptions would lead to misleading diffs. See the [generated applications docs](./generated-applications.md#why-cant-the-tool-do-this-automatically) for details.

---

### Does it work with a distributed Argo CD repository setup? (Multi-repo)

**Short answer:** Yes

**Longer answer:** Yes, but it depends on how complicated your setup is. Check out the [multi-repo](./multi-repo.md) documentation to learn how to use the tool with a distributed Argo CD repository setup.

---

### How do I speed up the tool?

**Short answer:** Connect to a pre-installed Argo CD instance and limit the number of applications rendered.

**Longer answer:** There are two main ways to speed up the tool:

1. **Connect to a pre-installed Argo CD instance**: Instead of creating an ephemeral cluster for each run, connect to a pre-installed Argo CD instance. This saves ~60-90 seconds per run. See [Connecting to a cluster with Argo CD pre-installed](./reusing-clusters/connecting.md).

2. **Limit applications rendered**: Rendering the manifests for all applications in the repository on each pull request can be time-consuming. Limiting the number of applications rendered can significantly speed up the process. By default, `argocd-diff-preview` renders all applications in the repository. Check out [Application Selection](./application-selection.md) to learn how to limit the number of applications rendered.
