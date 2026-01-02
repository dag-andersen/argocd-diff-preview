# Frequently Asked Questions (FAQ)

---

### Does the tool work with `ApplicationSets`?

**Short answer:** : Yes.

**Longer Answer:** Yes, but _how well it works_ depends on the complexity of your generators.

- **List generator**:
  Yes, no issues reported

- **Cluster generator**:
  Yes, but you have to add the ClusterSecrets to the `secrets` folder. Similar to how the secrets are provided [here](./getting-started/github-actions-workflow.md#private-repositories-and-helm-charts)

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

### Does it work with the Apps of Apps Pattern?

**Short answer:** Yes, but it depends on your setup.

**Longer answer:** The Apps of Apps Pattern can be configured in many ways, making it challenging to handle all cases. This tool identifies and renders all resources with `kind: Application` or `kind: ApplicationSet`. If your Applications or ApplicationSets are written as plain manifests in your repository, the tool will work seamlessly. However, if you have an Application that deploys a Helm chart, which then deploys the rest of your Applications, you will need to render the Helm chart before running the tool.

Relevant issue: [#75](https://github.com/dag-andersen/argocd-diff-preview/issues/75)

#### Why can't the tool automatically render my Helm Charts or Kustomize templates?

Helm and Kustomize configurations are inherently complex:

- **Helm:** Any YAML file can be used as a values file for Helm charts, making it impossible for the tool to automatically determine which YAML files should be used as values files and which Helm charts they belong to.
- **Kustomize:** Overlays in Kustomize can be chained in various ways. The tool cannot reliably figure out which overlays to use or skip.

Because of this, users must render their Helm charts and Kustomize templates before running the tool.

The tool is rather conservative in making assumptions about how Applications are rendered, with the goal of avoiding false positives.

More info: [docs](./generated-applications.md)

---

### Does it work with a distributed Argo CD repository setup? (Multi-repo)

**Short answer:** Yes

**Longer answer:** Yes, but it deepends on how complicated your setup is. Check out the [multi-repo](./multi-repo.md) documentation to learn how to use the tool with a distributed Argo CD repository setup.

---

### How do I speed up the tool?

**Short answer:** Limit the number of applications rendered.

**Longer answer:** Rendering the manifests for all applications in the repository on each pull request can be time-consuming. Limiting the number of applications rendered can significantly speed up the process. By default, `argocd-diff-preview` renders all applications in the repository.

Check out the [full documentation](./application-selection.md) to learn how to limit the number of applications rendered.
