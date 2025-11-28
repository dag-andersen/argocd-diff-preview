## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± argocd-helm-chart (+2449|-78)
```

<details>
<summary>argocd-helm-chart (examples/with-crds/applicaiton.yaml)</summary>
<br>

```diff
@@ Application modified: argocd-helm-chart (examples/with-crds/applicaiton.yaml) @@
                       maxDuration:
                         description: MaxDuration is the maximum amount of time allowed
                           for the backoff strategy
                         type: string
                     type: object
                   limit:
                     description: Limit is the maximum number of attempts for retrying
                       a failed sync. If set to 0, no retries will be performed.
                     format: int64
                     type: integer
+                  refresh:
+                    description: 'Refresh indicates if the latest revision should
+                      be used on retry instead of the initial one (default: false)'
+                    type: boolean
                 type: object
               sync:
                 description: Sync contains parameters for the operation
                 properties:
                   autoHealAttemptsCount:
                     description: SelfHealAttemptsCount contains the number of auto-heal
                       attempts
                     format: int64
                     type: integer
                   dryRun:
@@ skipped 178 lines (128 -> 305) @@
                               (Helm's --pass-credentials)
                             type: boolean
                           releaseName:
                             description: ReleaseName is the Helm release name to use.
                               If omitted it will use the application name
                             type: string
                           skipCrds:
                             description: SkipCrds skips custom resource definition
                               installation step (Helm's --skip-crds)
                             type: boolean
+                          skipSchemaValidation:
+                            description: SkipSchemaValidation skips JSON schema validation
+                              (Helm's --skip-schema-validation)
+                            type: boolean
+                          skipTests:
+                            description: SkipTests skips test manifest installation
+                              step (Helm's --skip-tests).
+                            type: boolean
                           valueFiles:
                             description: ValuesFiles is a list of Helm value files
                               to use when generating a template
                             items:
                               type: string
                             type: array
                           values:
                             description: Values specifies Helm values to be passed
                               to helm template, typically defined as a block. ValuesObject
                               takes precedence over Values, so use one or the other.
@@ skipped 43 lines (334 -> 376) @@
                             type: array
                           forceCommonAnnotations:
                             description: ForceCommonAnnotations specifies whether
                               to force applying common annotations to resources for
                               Kustomize apps
                             type: boolean
                           forceCommonLabels:
                             description: ForceCommonLabels specifies whether to force
                               applying common labels to resources for Kustomize apps
                             type: boolean
+                          ignoreMissingComponents:
+                            description: IgnoreMissingComponents prevents kustomize
+                              from failing when components do not exist locally by
+                              not appending them to kustomization file
+                            type: boolean
                           images:
                             description: Images is a list of Kustomize image override
                               specifications
                             items:
                               description: KustomizeImage represents a Kustomize image
                                 definition in the format []<image_name>:<image_tag>
                               type: string
                             type: array
                           kubeVersion:
                             description: |-
                               KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                               uses the Kubernetes version of the target cluster.
                             type: string
+                          labelIncludeTemplates:
+                            description: LabelIncludeTemplates specifies whether to
+                              apply common labels to resource templates or not
+                            type: boolean
                           labelWithoutSelector:
                             description: LabelWithoutSelector specifies whether to
                               apply common labels to resource selectors or not
                             type: boolean
                           namePrefix:
                             description: NamePrefix is a prefix appended to resources
                               for Kustomize apps
                             type: string
                           nameSuffix:
                             description: NameSuffix is a suffix appended to resources
@@ skipped 51 lines (419 -> 469) @@
                               required:
                               - count
                               - name
                               type: object
                             type: array
                           version:
                             description: Version controls which version of Kustomize
                               to use for rendering manifests
                             type: string
                         type: object
+                      name:
+                        description: Name is used to refer to a source and is displayed
+                          in the UI. It is used in multi-source Applications.
+                        type: string
                       path:
                         description: Path is a directory path within the Git repository,
                           and is only valid for applications sourced from Git.
                         type: string
                       plugin:
                         description: Plugin holds config management plugin specific
                           options
                         properties:
                           env:
                             description: Env is a list of environment variable entries
@@ skipped 199 lines (494 -> 692) @@
                                 domains (Helm's --pass-credentials)
                               type: boolean
                             releaseName:
                               description: ReleaseName is the Helm release name to
                                 use. If omitted it will use the application name
                               type: string
                             skipCrds:
                               description: SkipCrds skips custom resource definition
                                 installation step (Helm's --skip-crds)
                               type: boolean
+                            skipSchemaValidation:
+                              description: SkipSchemaValidation skips JSON schema
+                                validation (Helm's --skip-schema-validation)
+                              type: boolean
+                            skipTests:
+                              description: SkipTests skips test manifest installation
+                                step (Helm's --skip-tests).
+                              type: boolean
                             valueFiles:
                               description: ValuesFiles is a list of Helm value files
                                 to use when generating a template
                               items:
                                 type: string
                               type: array
                             values:
                               description: Values specifies Helm values to be passed
                                 to helm template, typically defined as a block. ValuesObject
                                 takes precedence over Values, so use one or the other.
@@ skipped 45 lines (721 -> 765) @@
                             forceCommonAnnotations:
                               description: ForceCommonAnnotations specifies whether
                                 to force applying common annotations to resources
                                 for Kustomize apps
                               type: boolean
                             forceCommonLabels:
                               description: ForceCommonLabels specifies whether to
                                 force applying common labels to resources for Kustomize
                                 apps
                               type: boolean
+                            ignoreMissingComponents:
+                              description: IgnoreMissingComponents prevents kustomize
+                                from failing when components do not exist locally
+                                by not appending them to kustomization file
+                              type: boolean
                             images:
                               description: Images is a list of Kustomize image override
                                 specifications
                               items:
                                 description: KustomizeImage represents a Kustomize
                                   image definition in the format []<image_name>:<image_tag>
                                 type: string
                               type: array
                             kubeVersion:
                               description: |-
                                 KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                                 uses the Kubernetes version of the target cluster.
                               type: string
+                            labelIncludeTemplates:
+                              description: LabelIncludeTemplates specifies whether
+                                to apply common labels to resource templates or not
+                              type: boolean
                             labelWithoutSelector:
                               description: LabelWithoutSelector specifies whether
                                 to apply common labels to resource selectors or not
                               type: boolean
                             namePrefix:
                               description: NamePrefix is a prefix appended to resources
                                 for Kustomize apps
                               type: string
                             nameSuffix:
                               description: NameSuffix is a suffix appended to resources
@@ skipped 51 lines (808 -> 858) @@
                                 required:
                                 - count
                                 - name
                                 type: object
                               type: array
                             version:
                               description: Version controls which version of Kustomize
                                 to use for rendering manifests
                               type: string
                           type: object
+                        name:
+                          description: Name is used to refer to a source and is displayed
+                            in the UI. It is used in multi-source Applications.
+                          type: string
                         path:
                           description: Path is a directory path within the Git repository,
                             and is only valid for applications sourced from Git.
                           type: string
                         plugin:
                           description: Plugin holds config management plugin specific
                             options
                           properties:
                             env:
                               description: Env is a list of environment variable entries
@@ skipped 312 lines (883 -> 1194) @@
                           (Helm's --pass-credentials)
                         type: boolean
                       releaseName:
                         description: ReleaseName is the Helm release name to use.
                           If omitted it will use the application name
                         type: string
                       skipCrds:
                         description: SkipCrds skips custom resource definition installation
                           step (Helm's --skip-crds)
                         type: boolean
+                      skipSchemaValidation:
+                        description: SkipSchemaValidation skips JSON schema validation
+                          (Helm's --skip-schema-validation)
+                        type: boolean
+                      skipTests:
+                        description: SkipTests skips test manifest installation step
+                          (Helm's --skip-tests).
+                        type: boolean
                       valueFiles:
                         description: ValuesFiles is a list of Helm value files to
                           use when generating a template
                         items:
                           type: string
                         type: array
                       values:
                         description: Values specifies Helm values to be passed to
                           helm template, typically defined as a block. ValuesObject
                           takes precedence over Values, so use one or the other.
@@ skipped 42 lines (1223 -> 1264) @@
                           type: string
                         type: array
                       forceCommonAnnotations:
                         description: ForceCommonAnnotations specifies whether to force
                           applying common annotations to resources for Kustomize apps
                         type: boolean
                       forceCommonLabels:
                         description: ForceCommonLabels specifies whether to force
                           applying common labels to resources for Kustomize apps
                         type: boolean
+                      ignoreMissingComponents:
+                        description: IgnoreMissingComponents prevents kustomize from
+                          failing when components do not exist locally by not appending
+                          them to kustomization file
+                        type: boolean
                       images:
                         description: Images is a list of Kustomize image override
                           specifications
                         items:
                           description: KustomizeImage represents a Kustomize image
                             definition in the format []<image_name>:<image_tag>
                           type: string
                         type: array
                       kubeVersion:
                         description: |-
                           KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                           uses the Kubernetes version of the target cluster.
                         type: string
+                      labelIncludeTemplates:
+                        description: LabelIncludeTemplates specifies whether to apply
+                          common labels to resource templates or not
+                        type: boolean
                       labelWithoutSelector:
                         description: LabelWithoutSelector specifies whether to apply
                           common labels to resource selectors or not
                         type: boolean
                       namePrefix:
                         description: NamePrefix is a prefix appended to resources
                           for Kustomize apps
                         type: string
                       nameSuffix:
                         description: NameSuffix is a suffix appended to resources
@@ skipped 51 lines (1307 -> 1357) @@
                           required:
                           - count
                           - name
                           type: object
                         type: array
                       version:
                         description: Version controls which version of Kustomize to
                           use for rendering manifests
                         type: string
                     type: object
+                  name:
+                    description: Name is used to refer to a source and is displayed
+                      in the UI. It is used in multi-source Applications.
+                    type: string
                   path:
                     description: Path is a directory path within the Git repository,
                       and is only valid for applications sourced from Git.
                     type: string
                   plugin:
                     description: Plugin holds config management plugin specific options
                     properties:
                       env:
                         description: Env is a list of environment variable entries
                         items:
@@ skipped 46 lines (1382 -> 1427) @@
                     type: string
                   targetRevision:
                     description: |-
                       TargetRevision defines the revision of the source to sync the application to.
                       In case of Git, this can be commit, tag, or branch. If omitted, will equal to HEAD.
                       In case of Helm, this is a semver tag for the Chart's version.
                     type: string
                 required:
                 - repoURL
                 type: object
+              sourceHydrator:
+                description: SourceHydrator provides a way to push hydrated manifests
+                  back to git before syncing them to the cluster.
+                properties:
+                  drySource:
+                    description: DrySource specifies where the dry "don't repeat yourself"
+                      manifest source lives.
+                    properties:
+                      path:
+                        description: Path is a directory path within the Git repository
+                          where the manifests are located
+                        type: string
+                      repoURL:
+                        description: RepoURL is the URL to the git repository that
+                          contains the application manifests
+                        type: string
+                      targetRevision:
+                        description: TargetRevision defines the revision of the source
+                          to hydrate
+                        type: string
+                    required:
+                    - path
+                    - repoURL
+                    - targetRevision
+                    type: object
+                  hydrateTo:
+                    description: |-
+                      HydrateTo specifies an optional "staging" location to push hydrated manifests to. An external system would then
+                      have to move manifests to the SyncSource, e.g. by pull request.
+                    properties:
+                      targetBranch:
+                        description: TargetBranch is the branch to which hydrated
+                          manifests should be committed
+                        type: string
+                    required:
+                    - targetBranch
+                    type: object
+                  syncSource:
+                    description: SyncSource specifies where to sync hydrated manifests
+                      from.
+                    properties:
+                      path:
+                        description: |-
+                          Path is a directory path within the git repository where hydrated manifests should be committed to and synced
+                          from. The Path should never point to the root of the repo. If hydrateTo is set, this is just the path from which
+                          hydrated manifests will be synced.
+                        minLength: 1
+                        pattern: ^.{2,}|[]$
+                        type: string
+                      targetBranch:
+                        description: |-
+                          TargetBranch is the branch from which hydrated manifests will be synced.
+                          If HydrateTo is not set, this is also the branch to which hydrated manifests are committed.
+                        type: string
+                    required:
+                    - path
+                    - targetBranch
+                    type: object
+                required:
+                - drySource
+                - syncSource
+                type: object
               sources:
                 description: Sources is a reference to the location of the application's
                   manifests or chart
                 items:
                   description: ApplicationSource contains all required information
                     about the source of an application
                   properties:
                     chart:
                       description: Chart is a Helm chart name, and must be specified
                         for applications sourced from a Helm repo.
@@ skipped 125 lines (1510 -> 1634) @@
                             (Helm's --pass-credentials)
                           type: boolean
                         releaseName:
                           description: ReleaseName is the Helm release name to use.
                             If omitted it will use the application name
                           type: string
                         skipCrds:
                           description: SkipCrds skips custom resource definition installation
                             step (Helm's --skip-crds)
                           type: boolean
+                        skipSchemaValidation:
+                          description: SkipSchemaValidation skips JSON schema validation
+                            (Helm's --skip-schema-validation)
+                          type: boolean
+                        skipTests:
+                          description: SkipTests skips test manifest installation
+                            step (Helm's --skip-tests).
+                          type: boolean
                         valueFiles:
                           description: ValuesFiles is a list of Helm value files to
                             use when generating a template
                           items:
                             type: string
                           type: array
                         values:
                           description: Values specifies Helm values to be passed to
                             helm template, typically defined as a block. ValuesObject
                             takes precedence over Values, so use one or the other.
@@ skipped 43 lines (1663 -> 1705) @@
                           type: array
                         forceCommonAnnotations:
                           description: ForceCommonAnnotations specifies whether to
                             force applying common annotations to resources for Kustomize
                             apps
                           type: boolean
                         forceCommonLabels:
                           description: ForceCommonLabels specifies whether to force
                             applying common labels to resources for Kustomize apps
                           type: boolean
+                        ignoreMissingComponents:
+                          description: IgnoreMissingComponents prevents kustomize
+                            from failing when components do not exist locally by not
+                            appending them to kustomization file
+                          type: boolean
                         images:
                           description: Images is a list of Kustomize image override
                             specifications
                           items:
                             description: KustomizeImage represents a Kustomize image
                               definition in the format []<image_name>:<image_tag>
                             type: string
                           type: array
                         kubeVersion:
                           description: |-
                             KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                             uses the Kubernetes version of the target cluster.
                           type: string
+                        labelIncludeTemplates:
+                          description: LabelIncludeTemplates specifies whether to
+                            apply common labels to resource templates or not
+                          type: boolean
                         labelWithoutSelector:
                           description: LabelWithoutSelector specifies whether to apply
                             common labels to resource selectors or not
                           type: boolean
                         namePrefix:
                           description: NamePrefix is a prefix appended to resources
                             for Kustomize apps
                           type: string
                         nameSuffix:
                           description: NameSuffix is a suffix appended to resources
@@ skipped 51 lines (1748 -> 1798) @@
                             required:
                             - count
                             - name
                             type: object
                           type: array
                         version:
                           description: Version controls which version of Kustomize
                             to use for rendering manifests
                           type: string
                       type: object
+                    name:
+                      description: Name is used to refer to a source and is displayed
+                        in the UI. It is used in multi-source Applications.
+                      type: string
                     path:
                       description: Path is a directory path within the Git repository,
                         and is only valid for applications sourced from Git.
                       type: string
                     plugin:
                       description: Plugin holds config management plugin specific
                         options
                       properties:
                         env:
                           description: Env is a list of environment variable entries
@@ skipped 61 lines (1823 -> 1883) @@
                 description: SyncPolicy controls when and how a sync will be performed
                 properties:
                   automated:
                     description: Automated will keep an application synced to the
                       target revision
                     properties:
                       allowEmpty:
                         description: 'AllowEmpty allows apps have zero live resources
                           (default: false)'
                         type: boolean
+                      enabled:
+                        description: Enable allows apps to explicitly control automated
+                          sync
+                        type: boolean
                       prune:
                         description: 'Prune specifies whether to delete resources
                           from the cluster that are not found in the sources anymore
                           as part of automated sync (default: false)'
                         type: boolean
                       selfHeal:
                         description: 'SelfHeal specifies whether to revert resources
                           back to their desired state upon modification in the cluster
                           (default: false)'
                         type: boolean
@@ skipped 31 lines (1908 -> 1938) @@
                           maxDuration:
                             description: MaxDuration is the maximum amount of time
                               allowed for the backoff strategy
                             type: string
                         type: object
                       limit:
                         description: Limit is the maximum number of attempts for retrying
                           a failed sync. If set to 0, no retries will be performed.
                         format: int64
                         type: integer
+                      refresh:
+                        description: 'Refresh indicates if the latest revision should
+                          be used on retry instead of the initial one (default: false)'
+                        type: boolean
                     type: object
                   syncOptions:
                     description: Options allow you to specify whole app sync-options
                     items:
                       type: string
                     type: array
                 type: object
             required:
             - destination
             - project
@@ skipped 26 lines (1963 -> 1988) @@
                   type: object
                 type: array
               controllerNamespace:
                 description: ControllerNamespace indicates the namespace in which
                   the application controller is located
                 type: string
               health:
                 description: Health contains information about the application's current
                   health status
                 properties:
+                  lastTransitionTime:
+                    description: LastTransitionTime is the time the HealthStatus was
+                      set or updated
+                    format: date-time
+                    type: string
                   message:
-                    description: Message is a human-readable informational message
-                      describing the health status
+                    description: |-
+                      Message is a human-readable informational message describing the health status
+
+                      Deprecated: this field is not used and will be removed in a future release.
                     type: string
                   status:
-                    description: Status holds the status code of the application or
-                      resource
+                    description: Status holds the status code of the application
                     type: string
                 type: object
               history:
                 description: History contains information about the application's
                   sync history
                 items:
                   description: RevisionHistory contains history information about
                     a previous sync
                   properties:
                     deployStartedAt:
@@ skipped 170 lines (2026 -> 2195) @@
                                 domains (Helm's --pass-credentials)
                               type: boolean
                             releaseName:
                               description: ReleaseName is the Helm release name to
                                 use. If omitted it will use the application name
                               type: string
                             skipCrds:
                               description: SkipCrds skips custom resource definition
                                 installation step (Helm's --skip-crds)
                               type: boolean
+                            skipSchemaValidation:
+                              description: SkipSchemaValidation skips JSON schema
+                                validation (Helm's --skip-schema-validation)
+                              type: boolean
+                            skipTests:
+                              description: SkipTests skips test manifest installation
+                                step (Helm's --skip-tests).
+                              type: boolean
                             valueFiles:
                               description: ValuesFiles is a list of Helm value files
                                 to use when generating a template
                               items:
                                 type: string
                               type: array
                             values:
                               description: Values specifies Helm values to be passed
                                 to helm template, typically defined as a block. ValuesObject
                                 takes precedence over Values, so use one or the other.
@@ skipped 45 lines (2224 -> 2268) @@
                             forceCommonAnnotations:
                               description: ForceCommonAnnotations specifies whether
                                 to force applying common annotations to resources
                                 for Kustomize apps
                               type: boolean
                             forceCommonLabels:
                               description: ForceCommonLabels specifies whether to
                                 force applying common labels to resources for Kustomize
                                 apps
                               type: boolean
+                            ignoreMissingComponents:
+                              description: IgnoreMissingComponents prevents kustomize
+                                from failing when components do not exist locally
+                                by not appending them to kustomization file
+                              type: boolean
                             images:
                               description: Images is a list of Kustomize image override
                                 specifications
                               items:
                                 description: KustomizeImage represents a Kustomize
                                   image definition in the format []<image_name>:<image_tag>
                                 type: string
                               type: array
                             kubeVersion:
                               description: |-
                                 KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                                 uses the Kubernetes version of the target cluster.
                               type: string
+                            labelIncludeTemplates:
+                              description: LabelIncludeTemplates specifies whether
+                                to apply common labels to resource templates or not
+                              type: boolean
                             labelWithoutSelector:
                               description: LabelWithoutSelector specifies whether
                                 to apply common labels to resource selectors or not
                               type: boolean
                             namePrefix:
                               description: NamePrefix is a prefix appended to resources
                                 for Kustomize apps
                               type: string
                             nameSuffix:
                               description: NameSuffix is a suffix appended to resources
@@ skipped 51 lines (2311 -> 2361) @@
                                 required:
                                 - count
                                 - name
                                 type: object
                               type: array
                             version:
                               description: Version controls which version of Kustomize
                                 to use for rendering manifests
                               type: string
                           type: object
+                        name:
+                          description: Name is used to refer to a source and is displayed
+                            in the UI. It is used in multi-source Applications.
+                          type: string
                         path:
                           description: Path is a directory path within the Git repository,
                             and is only valid for applications sourced from Git.
                           type: string
                         plugin:
                           description: Plugin holds config management plugin specific
                             options
                           properties:
                             env:
                               description: Env is a list of environment variable entries
@@ skipped 200 lines (2386 -> 2585) @@
                                   domains (Helm's --pass-credentials)
                                 type: boolean
                               releaseName:
                                 description: ReleaseName is the Helm release name
                                   to use. If omitted it will use the application name
                                 type: string
                               skipCrds:
                                 description: SkipCrds skips custom resource definition
                                   installation step (Helm's --skip-crds)
                                 type: boolean
+                              skipSchemaValidation:
+                                description: SkipSchemaValidation skips JSON schema
+                                  validation (Helm's --skip-schema-validation)
+                                type: boolean
+                              skipTests:
+                                description: SkipTests skips test manifest installation
+                                  step (Helm's --skip-tests).
+                                type: boolean
                               valueFiles:
                                 description: ValuesFiles is a list of Helm value files
                                   to use when generating a template
                                 items:
                                   type: string
                                 type: array
                               values:
                                 description: Values specifies Helm values to be passed
                                   to helm template, typically defined as a block.
                                   ValuesObject takes precedence over Values, so use
@@ skipped 46 lines (2614 -> 2659) @@
                               forceCommonAnnotations:
                                 description: ForceCommonAnnotations specifies whether
                                   to force applying common annotations to resources
                                   for Kustomize apps
                                 type: boolean
                               forceCommonLabels:
                                 description: ForceCommonLabels specifies whether to
                                   force applying common labels to resources for Kustomize
                                   apps
                                 type: boolean
+                              ignoreMissingComponents:
+                                description: IgnoreMissingComponents prevents kustomize
+                                  from failing when components do not exist locally
+                                  by not appending them to kustomization file
+                                type: boolean
                               images:
                                 description: Images is a list of Kustomize image override
                                   specifications
                                 items:
                                   description: KustomizeImage represents a Kustomize
                                     image definition in the format []<image_name>:<image_tag>
                                   type: string
                                 type: array
                               kubeVersion:
                                 description: |-
                                   KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                                   uses the Kubernetes version of the target cluster.
                                 type: string
+                              labelIncludeTemplates:
+                                description: LabelIncludeTemplates specifies whether
+                                  to apply common labels to resource templates or
+                                  not
+                                type: boolean
                               labelWithoutSelector:
                                 description: LabelWithoutSelector specifies whether
                                   to apply common labels to resource selectors or
                                   not
                                 type: boolean
                               namePrefix:
                                 description: NamePrefix is a prefix appended to resources
                                   for Kustomize apps
                                 type: string
                               nameSuffix:
@@ skipped 52 lines (2703 -> 2754) @@
                                   required:
                                   - count
                                   - name
                                   type: object
                                 type: array
                               version:
                                 description: Version controls which version of Kustomize
                                   to use for rendering manifests
                                 type: string
                             type: object
+                          name:
+                            description: Name is used to refer to a source and is
+                              displayed in the UI. It is used in multi-source Applications.
+                            type: string
                           path:
                             description: Path is a directory path within the Git repository,
                               and is only valid for applications sourced from Git.
                             type: string
                           plugin:
                             description: Plugin holds config management plugin specific
                               options
                             properties:
                               env:
                                 description: Env is a list of environment variable
@@ skipped 136 lines (2779 -> 2914) @@
                                 description: MaxDuration is the maximum amount of
                                   time allowed for the backoff strategy
                                 type: string
                             type: object
                           limit:
                             description: Limit is the maximum number of attempts for
                               retrying a failed sync. If set to 0, no retries will
                               be performed.
                             format: int64
                             type: integer
+                          refresh:
+                            description: 'Refresh indicates if the latest revision
+                              should be used on retry instead of the initial one (default:
+                              false)'
+                            type: boolean
                         type: object
                       sync:
                         description: Sync contains parameters for the operation
                         properties:
                           autoHealAttemptsCount:
                             description: SelfHealAttemptsCount contains the number
                               of auto-heal attempts
                             format: int64
                             type: integer
                           dryRun:
@@ skipped 192 lines (2940 -> 3131) @@
                                     type: boolean
                                   releaseName:
                                     description: ReleaseName is the Helm release name
                                       to use. If omitted it will use the application
                                       name
                                     type: string
                                   skipCrds:
                                     description: SkipCrds skips custom resource definition
                                       installation step (Helm's --skip-crds)
                                     type: boolean
+                                  skipSchemaValidation:
+                                    description: SkipSchemaValidation skips JSON schema
+                                      validation (Helm's --skip-schema-validation)
+                                    type: boolean
+                                  skipTests:
+                                    description: SkipTests skips test manifest installation
+                                      step (Helm's --skip-tests).
+                                    type: boolean
                                   valueFiles:
                                     description: ValuesFiles is a list of Helm value
                                       files to use when generating a template
                                     items:
                                       type: string
                                     type: array
                                   values:
                                     description: Values specifies Helm values to be
                                       passed to helm template, typically defined as
                                       a block. ValuesObject takes precedence over
@@ skipped 47 lines (3160 -> 3206) @@
                                   forceCommonAnnotations:
                                     description: ForceCommonAnnotations specifies
                                       whether to force applying common annotations
                                       to resources for Kustomize apps
                                     type: boolean
                                   forceCommonLabels:
                                     description: ForceCommonLabels specifies whether
                                       to force applying common labels to resources
                                       for Kustomize apps
                                     type: boolean
+                                  ignoreMissingComponents:
+                                    description: IgnoreMissingComponents prevents
+                                      kustomize from failing when components do not
+                                      exist locally by not appending them to kustomization
+                                      file
+                                    type: boolean
                                   images:
                                     description: Images is a list of Kustomize image
                                       override specifications
                                     items:
                                       description: KustomizeImage represents a Kustomize
                                         image definition in the format []<image_name>:<image_tag>
                                       type: string
                                     type: array
                                   kubeVersion:
                                     description: |-
                                       KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                                       uses the Kubernetes version of the target cluster.
                                     type: string
+                                  labelIncludeTemplates:
+                                    description: LabelIncludeTemplates specifies whether
+                                      to apply common labels to resource templates
+                                      or not
+                                    type: boolean
                                   labelWithoutSelector:
                                     description: LabelWithoutSelector specifies whether
                                       to apply common labels to resource selectors
                                       or not
                                     type: boolean
                                   namePrefix:
                                     description: NamePrefix is a prefix appended to
                                       resources for Kustomize apps
                                     type: string
                                   nameSuffix:
@@ skipped 52 lines (3251 -> 3302) @@
                                       required:
                                       - count
                                       - name
                                       type: object
                                     type: array
                                   version:
                                     description: Version controls which version of
                                       Kustomize to use for rendering manifests
                                     type: string
                                 type: object
+                              name:
+                                description: Name is used to refer to a source and
+                                  is displayed in the UI. It is used in multi-source
+                                  Applications.
+                                type: string
                               path:
                                 description: Path is a directory path within the Git
                                   repository, and is only valid for applications sourced
                                   from Git.
                                 type: string
                               plugin:
                                 description: Plugin holds config management plugin
                                   specific options
                                 properties:
                                   env:
@@ skipped 215 lines (3328 -> 3542) @@
                                       type: boolean
                                     releaseName:
                                       description: ReleaseName is the Helm release
                                         name to use. If omitted it will use the application
                                         name
                                       type: string
                                     skipCrds:
                                       description: SkipCrds skips custom resource
                                         definition installation step (Helm's --skip-crds)
                                       type: boolean
+                                    skipSchemaValidation:
+                                      description: SkipSchemaValidation skips JSON
+                                        schema validation (Helm's --skip-schema-validation)
+                                      type: boolean
+                                    skipTests:
+                                      description: SkipTests skips test manifest installation
+                                        step (Helm's --skip-tests).
+                                      type: boolean
                                     valueFiles:
                                       description: ValuesFiles is a list of Helm value
                                         files to use when generating a template
                                       items:
                                         type: string
                                       type: array
                                     values:
                                       description: Values specifies Helm values to
                                         be passed to helm template, typically defined
                                         as a block. ValuesObject takes precedence
@@ skipped 49 lines (3571 -> 3619) @@
                                     forceCommonAnnotations:
                                       description: ForceCommonAnnotations specifies
                                         whether to force applying common annotations
                                         to resources for Kustomize apps
                                       type: boolean
                                     forceCommonLabels:
                                       description: ForceCommonLabels specifies whether
                                         to force applying common labels to resources
                                         for Kustomize apps
                                       type: boolean
+                                    ignoreMissingComponents:
+                                      description: IgnoreMissingComponents prevents
+                                        kustomize from failing when components do
+                                        not exist locally by not appending them to
+                                        kustomization file
+                                      type: boolean
                                     images:
                                       description: Images is a list of Kustomize image
                                         override specifications
                                       items:
                                         description: KustomizeImage represents a Kustomize
                                           image definition in the format []<image_name>:<image_tag>
                                         type: string
                                       type: array
                                     kubeVersion:
                                       description: |-
                                         KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                                         uses the Kubernetes version of the target cluster.
                                       type: string
+                                    labelIncludeTemplates:
+                                      description: LabelIncludeTemplates specifies
+                                        whether to apply common labels to resource
+                                        templates or not
+                                      type: boolean
                                     labelWithoutSelector:
                                       description: LabelWithoutSelector specifies
                                         whether to apply common labels to resource
                                         selectors or not
                                       type: boolean
                                     namePrefix:
                                       description: NamePrefix is a prefix appended
                                         to resources for Kustomize apps
                                       type: string
                                     nameSuffix:
@@ skipped 53 lines (3664 -> 3716) @@
                                         required:
                                         - count
                                         - name
                                         type: object
                                       type: array
                                     version:
                                       description: Version controls which version
                                         of Kustomize to use for rendering manifests
                                       type: string
                                   type: object
+                                name:
+                                  description: Name is used to refer to a source and
+                                    is displayed in the UI. It is used in multi-source
+                                    Applications.
+                                  type: string
                                 path:
                                   description: Path is a directory path within the
                                     Git repository, and is only valid for applications
                                     sourced from Git.
                                   type: string
                                 plugin:
                                   description: Plugin holds config management plugin
                                     specific options
                                   properties:
                                     env:
@@ skipped 137 lines (3742 -> 3878) @@
                               type: string
                             hookPhase:
                               description: |-
                                 HookPhase contains the state of any operation associated with this resource OR hook
                                 This can also contain values for non-hook resources.
                               type: string
                             hookType:
                               description: HookType specifies the type of the hook.
                                 Empty for non-hook resources
                               type: string
+                            images:
+                              description: Images contains the images related to the
+                                ResourceResult
+                              items:
+                                type: string
+                              type: array
                             kind:
                               description: Kind specifies the API kind of the resource
                               type: string
                             message:
                               description: Message contains an informational or error
                                 message for the last sync OR operation
                               type: string
                             name:
                               description: Name specifies the name of the resource
                               type: string
@@ skipped 172 lines (3905 -> 4076) @@
                                   domains (Helm's --pass-credentials)
                                 type: boolean
                               releaseName:
                                 description: ReleaseName is the Helm release name
                                   to use. If omitted it will use the application name
                                 type: string
                               skipCrds:
                                 description: SkipCrds skips custom resource definition
                                   installation step (Helm's --skip-crds)
                                 type: boolean
+                              skipSchemaValidation:
+                                description: SkipSchemaValidation skips JSON schema
+                                  validation (Helm's --skip-schema-validation)
+                                type: boolean
+                              skipTests:
+                                description: SkipTests skips test manifest installation
+                                  step (Helm's --skip-tests).
+                                type: boolean
                               valueFiles:
                                 description: ValuesFiles is a list of Helm value files
                                   to use when generating a template
                                 items:
                                   type: string
                                 type: array
                               values:
                                 description: Values specifies Helm values to be passed
                                   to helm template, typically defined as a block.
                                   ValuesObject takes precedence over Values, so use
@@ skipped 45 lines (4105 -> 4149) @@
                                 type: array
                               forceCommonAnnotations:
                                 description: ForceCommonAnnotations specifies whether
                                   to force applying common annotations to resources
                                   for Kustomize apps
                                 type: boolean
                               forceCommonLabels:
                                 description: ForceCommonLabels specifies whether to
                                   force applying common labels to resources for Kustomize
                                   apps
+                                type: boolean
+                              ignoreMissingComponents:
+                                description: IgnoreMissingComponents prevents kustomize
+                                  from failing when components do not exist locally
+                                  by not appending them to kustomization file
                                 type: boolean
                               images:
                                 description: Images is a list of Kustomize image override
                                   specifications
                                 items:
                                   description: KustomizeImage represents a Kustomize
                                     image definition in the format []<image_name>:<image_tag>
                                   type: string
                                 type: array
                               kubeVersion:
                                 description: |-
                                   KubeVersion specifies the Kubernetes API version to pass to Helm when templating manifests. By default, Argo CD
                                   uses the Kubernetes version of the target cluster.
                                 type: string
+                              labelIncludeTemplates:
+                                description: LabelIncludeTemplates specifies whether
+                                  to apply common labels to resource templates or
+                                  not
+                                type: boolean
                               labelWithoutSelector:
                                 description: LabelWithoutSelector specifies whether
                                   to apply common labels to resource selectors or
                                   not
                                 type: boolean
                               namePrefix:
                                 description: NamePrefix is a prefix appended to resources
                                   for Kustomize apps
                                 type: string
                               nameSuffix:
@@ skipped 52 lines (4194 -> 4245) @@
                                   required:
                                   - count
                                   - name
                                   type: object
                                 type: array
                               version:
                                 description: Version controls which version of Kustomize
                                   to use for rendering manifests
                                 type: string
                             type: object
+                          name:
+                            description: Name is used to refer to a source and is
+                              displayed in the UI. It is used in multi-source Applic
🚨 Diff is too long
```

</details>

⚠️⚠️⚠️ Diff exceeds max length of 65536 characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag

_Stats_:
[], [], [], [], []
