## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± argocd-helm-chart (+2436|-64)
```

<details>
<summary>argocd-helm-chart (examples/with-crds/applicaiton.yaml)</summary>
<br>

#### Deployment/argocd-helm-chart-applicationset-controller (argocd)
```diff
             configMapKeyRef:
               key: applicationsetcontroller.log.format
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATIONSET_CONTROLLER_LOGLEVEL
           valueFrom:
             configMapKeyRef:
               key: applicationsetcontroller.log.level
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_LOG_FORMAT_TIMESTAMP
+          valueFrom:
+            configMapKeyRef:
+              key: log.format.timestamp
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_APPLICATIONSET_CONTROLLER_DRY_RUN
           valueFrom:
             configMapKeyRef:
               key: applicationsetcontroller.dryrun
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_GIT_MODULES_ENABLED
           valueFrom:
             configMapKeyRef:
               key: applicationsetcontroller.enable.git.submodule
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_PROGRESSIVE_SYNCS
           valueFrom:
             configMapKeyRef:
               key: applicationsetcontroller.enable.progressive.syncs
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_APPLICATIONSET_CONTROLLER_TOKENREF_STRICT_MODE
+          valueFrom:
+            configMapKeyRef:
+              key: applicationsetcontroller.enable.tokenref.strict.mode
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_NEW_GIT_FILE_GLOBBING
           valueFrom:
             configMapKeyRef:
               key: applicationsetcontroller.enable.new.git.file.globbing
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATIONSET_CONTROLLER_REPO_SERVER_PLAINTEXT
           valueFrom:
@@ skipped 36 lines (151 -> 186) @@
             configMapKeyRef:
               key: applicationsetcontroller.allowed.scm.providers
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_SCM_PROVIDERS
           valueFrom:
             configMapKeyRef:
               key: applicationsetcontroller.enable.scm.providers
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_GITHUB_API_METRICS
+          valueFrom:
+            configMapKeyRef:
+              key: applicationsetcontroller.enable.github.api.metrics
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_APPLICATIONSET_CONTROLLER_WEBHOOK_PARALLELISM_LIMIT
           valueFrom:
             configMapKeyRef:
               key: applicationsetcontroller.webhook.parallelism.limit
               name: argocd-cmd-params-cm
               optional: true
-        image: quay.io/argoproj/argocd:v2.13.1
+        - name: ARGOCD_APPLICATIONSET_CONTROLLER_REQUEUE_AFTER
+          valueFrom:
+            configMapKeyRef:
+              key: applicationsetcontroller.requeue.after
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_APPLICATIONSET_CONTROLLER_MAX_RESOURCES_STATUS_COUNT
+          valueFrom:
+            configMapKeyRef:
+              key: applicationsetcontroller.status.max.resources.count
+              name: argocd-cmd-params-cm
+              optional: true
+        image: quay.io/argoproj/argocd:v3.2.0
         imagePullPolicy: IfNotPresent
         name: applicationset-controller
         ports:
         - containerPort: 8080
           name: metrics
           protocol: TCP
         - containerPort: 8081
           name: probe
           protocol: TCP
         - containerPort: 7000
@@ skipped 13 lines (233 -> 245) @@
         - mountPath: /app/config/ssh
           name: ssh-known-hosts
         - mountPath: /app/config/tls
           name: tls-certs
         - mountPath: /app/config/gpg/source
           name: gpg-keys
         - mountPath: /app/config/gpg/keys
           name: gpg-keyring
         - mountPath: /app/config/reposerver/tls
           name: argocd-repo-server-tls
+        - mountPath: /home/argocd/params
+          name: argocd-cmd-params-cm
         - mountPath: /tmp
           name: tmp
       dnsPolicy: ClusterFirst
+      nodeSelector:
+        kubernetes.io/os: linux
       serviceAccountName: argocd-applicationset-controller
       terminationGracePeriodSeconds: 30
       volumes:
       - configMap:
           name: argocd-ssh-known-hosts-cm
         name: ssh-known-hosts
       - configMap:
           name: argocd-tls-certs-cm
         name: tls-certs
       - configMap:
           name: argocd-gpg-keys-cm
         name: gpg-keys
       - emptyDir: {}
         name: gpg-keyring
       - emptyDir: {}
         name: tmp
       - name: argocd-repo-server-tls
         secret:
           items:
           - key: tls.crt
             path: tls.crt
           - key: tls.key
             path: tls.key
           - key: ca.crt
             path: ca.crt
           optional: true
           secretName: argocd-repo-server-tls
+      - configMap:
+          items:
+          - key: applicationsetcontroller.profile.enabled
+            path: profiler.enabled
+          name: argocd-cmd-params-cm
+          optional: true
+        name: argocd-cmd-params-cm
```
#### Deployment/argocd-helm-chart-dex-server (argocd)
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/component: dex-server
     app.kubernetes.io/instance: argocd-helm-chart
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: argocd-dex-server
     app.kubernetes.io/part-of: argocd
@@ skipped 34 lines (307 -> 340) @@
                 matchLabels:
                   app.kubernetes.io/name: argocd-dex-server
               topologyKey: kubernetes.io/hostname
             weight: 100
       automountServiceAccountToken: true
       containers:
       - args:
         - rundex
         command:
         - /shared/argocd-dex
-        - --logformat=text
-        - --loglevel=info
         env:
         - name: ARGOCD_DEX_SERVER_LOGFORMAT
           valueFrom:
             configMapKeyRef:
               key: dexserver.log.format
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_DEX_SERVER_LOGLEVEL
           valueFrom:
             configMapKeyRef:
               key: dexserver.log.level
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_LOG_FORMAT_TIMESTAMP
+          valueFrom:
+            configMapKeyRef:
+              key: log.format.timestamp
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_DEX_SERVER_DISABLE_TLS
           valueFrom:
             configMapKeyRef:
               key: dexserver.disable.tls
               name: argocd-cmd-params-cm
               optional: true
-        image: ghcr.io/dexidp/dex:v2.41.1
+        image: ghcr.io/dexidp/dex:v2.44.0
         imagePullPolicy: IfNotPresent
         name: dex-server
         ports:
         - containerPort: 5556
           name: http
           protocol: TCP
         - containerPort: 5557
           name: grpc
           protocol: TCP
         - containerPort: 5558
@@ skipped 16 lines (390 -> 405) @@
           name: dexconfig
         - mountPath: /tls
           name: argocd-dex-server-tls
       dnsPolicy: ClusterFirst
       initContainers:
       - command:
         - /bin/cp
         - -n
         - /usr/local/bin/argocd
         - /shared/argocd-dex
-        image: quay.io/argoproj/argocd:v2.13.1
+        image: quay.io/argoproj/argocd:v3.2.0
         imagePullPolicy: IfNotPresent
         name: copyutil
         resources: {}
         securityContext:
           allowPrivilegeEscalation: false
           capabilities:
             drop:
             - ALL
           readOnlyRootFilesystem: true
           runAsNonRoot: true
           seccompProfile:
             type: RuntimeDefault
         volumeMounts:
         - mountPath: /shared
           name: static-files
         - mountPath: /tmp
           name: dexconfig
+      nodeSelector:
+        kubernetes.io/os: linux
       serviceAccountName: argocd-dex-server
       terminationGracePeriodSeconds: 30
       volumes:
       - emptyDir: {}
         name: static-files
       - emptyDir: {}
         name: dexconfig
       - name: argocd-dex-server-tls
         secret:
           items:
@@ skipped 52 lines (447 -> 498) @@
```
#### Deployment/argocd-helm-chart-notifications-controller (argocd)
```diff
               labelSelector:
                 matchLabels:
                   app.kubernetes.io/name: argocd-notifications-controller
               topologyKey: kubernetes.io/hostname
             weight: 100
       automountServiceAccountToken: true
       containers:
       - args:
         - /usr/local/bin/argocd-notifications
         - --metrics-port=9001
-        - --loglevel=info
-        - --logformat=text
         - --namespace=argocd
         - --argocd-repo-server=argocd-helm-chart-repo-server:8081
         - --secret-name=argocd-notifications-secret
         env:
         - name: ARGOCD_NOTIFICATIONS_CONTROLLER_LOGLEVEL
           valueFrom:
             configMapKeyRef:
               key: notificationscontroller.log.level
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_NOTIFICATIONS_CONTROLLER_LOGFORMAT
           valueFrom:
             configMapKeyRef:
               key: notificationscontroller.log.format
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_LOG_FORMAT_TIMESTAMP
+          valueFrom:
+            configMapKeyRef:
+              key: log.format.timestamp
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_APPLICATION_NAMESPACES
           valueFrom:
             configMapKeyRef:
               key: application.namespaces
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_NOTIFICATION_CONTROLLER_SELF_SERVICE_NOTIFICATION_ENABLED
           valueFrom:
             configMapKeyRef:
               key: notificationscontroller.selfservice.enabled
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_NOTIFICATION_CONTROLLER_REPO_SERVER_PLAINTEXT
           valueFrom:
             configMapKeyRef:
               key: notificationscontroller.repo.server.plaintext
               name: argocd-cmd-params-cm
               optional: true
-        image: quay.io/argoproj/argocd:v2.13.1
+        image: quay.io/argoproj/argocd:v3.2.0
         imagePullPolicy: IfNotPresent
         name: notifications-controller
         ports:
         - containerPort: 9001
           name: metrics
           protocol: TCP
         resources: {}
         securityContext:
           allowPrivilegeEscalation: false
           capabilities:
             drop:
             - ALL
           readOnlyRootFilesystem: true
           runAsNonRoot: true
           seccompProfile:
             type: RuntimeDefault
         volumeMounts:
         - mountPath: /app/config/tls
           name: tls-certs
         - mountPath: /app/config/reposerver/tls
           name: argocd-repo-server-tls
         workingDir: /app
       dnsPolicy: ClusterFirst
+      nodeSelector:
+        kubernetes.io/os: linux
       serviceAccountName: argocd-notifications-controller
       terminationGracePeriodSeconds: 30
       volumes:
       - configMap:
           name: argocd-tls-certs-cm
         name: tls-certs
       - name: argocd-repo-server-tls
         secret:
           items:
           - key: tls.crt
@@ skipped 55 lines (588 -> 642) @@
```
#### Deployment/argocd-helm-chart-redis (argocd)
```diff
         - ""
         - --appendonly
         - "no"
         - --requirepass $(REDIS_PASSWORD)
         env:
         - name: REDIS_PASSWORD
           valueFrom:
             secretKeyRef:
               key: auth
               name: argocd-redis
-        image: public.ecr.aws/docker/library/redis:7.4.1-alpine
+        image: ecr-public.aws.com/docker/library/redis:8.2.2-alpine
         imagePullPolicy: IfNotPresent
         name: redis
         ports:
         - containerPort: 6379
           name: redis
           protocol: TCP
         resources: {}
         securityContext:
           allowPrivilegeEscalation: false
           capabilities:
             drop:
             - ALL
           readOnlyRootFilesystem: true
         volumeMounts:
         - mountPath: /health
           name: health
       dnsPolicy: ClusterFirst
+      nodeSelector:
+        kubernetes.io/os: linux
       securityContext:
         runAsNonRoot: true
         runAsUser: 999
         seccompProfile:
           type: RuntimeDefault
       serviceAccountName: default
       terminationGracePeriodSeconds: 30
       volumes:
       - configMap:
           defaultMode: 493
@@ skipped 67 lines (684 -> 750) @@
```
#### Deployment/argocd-helm-chart-repo-server (argocd)
```diff
         - name: ARGOCD_REPO_SERVER_LOGFORMAT
           valueFrom:
             configMapKeyRef:
               key: reposerver.log.format
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_REPO_SERVER_LOGLEVEL
           valueFrom:
             configMapKeyRef:
               key: reposerver.log.level
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_LOG_FORMAT_TIMESTAMP
+          valueFrom:
+            configMapKeyRef:
+              key: log.format.timestamp
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_REPO_SERVER_PARALLELISM_LIMIT
           valueFrom:
             configMapKeyRef:
               key: reposerver.parallelism.limit
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_REPO_SERVER_LISTEN_ADDRESS
           valueFrom:
@@ skipped 59 lines (777 -> 835) @@
           valueFrom:
             secretKeyRef:
               key: redis-username
               name: argocd-redis
               optional: true
         - name: REDIS_PASSWORD
           valueFrom:
             secretKeyRef:
               key: auth
               name: argocd-redis
-              optional: true
+              optional: false
         - name: REDIS_SENTINEL_USERNAME
           valueFrom:
             secretKeyRef:
               key: redis-sentinel-username
               name: argocd-helm-chart-redis
               optional: true
         - name: REDIS_SENTINEL_PASSWORD
           valueFrom:
             secretKeyRef:
               key: redis-sentinel-password
@@ skipped 16 lines (858 -> 873) @@
             configMapKeyRef:
               key: otlp.insecure
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_REPO_SERVER_OTLP_HEADERS
           valueFrom:
             configMapKeyRef:
               key: otlp.headers
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_REPO_SERVER_OTLP_ATTRS
+          valueFrom:
+            configMapKeyRef:
+              key: otlp.attrs
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_REPO_SERVER_MAX_COMBINED_DIRECTORY_MANIFESTS_SIZE
           valueFrom:
             configMapKeyRef:
               key: reposerver.max.combined.directory.manifests.size
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_REPO_SERVER_PLUGIN_TAR_EXCLUSIONS
           valueFrom:
             configMapKeyRef:
               key: reposerver.plugin.tar.exclusions
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_REPO_SERVER_PLUGIN_USE_MANIFEST_GENERATE_PATHS
+          valueFrom:
+            configMapKeyRef:
+              key: reposerver.plugin.use.manifest.generate.paths
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_REPO_SERVER_ALLOW_OUT_OF_BOUNDS_SYMLINKS
           valueFrom:
             configMapKeyRef:
               key: reposerver.allow.oob.symlinks
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_REPO_SERVER_STREAMED_MANIFEST_MAX_TAR_SIZE
           valueFrom:
             configMapKeyRef:
               key: reposerver.streamed.manifest.max.tar.size
@@ skipped 28 lines (918 -> 945) @@
             configMapKeyRef:
               key: reposerver.git.lsremote.parallelism.limit
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_GIT_REQUEST_TIMEOUT
           valueFrom:
             configMapKeyRef:
               key: reposerver.git.request.timeout
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_REPO_SERVER_OCI_MANIFEST_MAX_EXTRACTED_SIZE
+          valueFrom:
+            configMapKeyRef:
+              key: reposerver.oci.manifest.max.extracted.size
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_REPO_SERVER_DISABLE_OCI_MANIFEST_MAX_EXTRACTED_SIZE
+          valueFrom:
+            configMapKeyRef:
+              key: reposerver.disable.oci.manifest.max.extracted.size
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_REPO_SERVER_OCI_LAYER_MEDIA_TYPES
+          valueFrom:
+            configMapKeyRef:
+              key: reposerver.oci.layer.media.types
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_REVISION_CACHE_LOCK_TIMEOUT
           valueFrom:
             configMapKeyRef:
               key: reposerver.revision.cache.lock.timeout
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_REPO_SERVER_INCLUDE_HIDDEN_DIRECTORIES
           valueFrom:
             configMapKeyRef:
               key: reposerver.include.hidden.directories
               name: argocd-cmd-params-cm
               optional: true
         - name: HELM_CACHE_HOME
           value: /helm-working-dir
         - name: HELM_CONFIG_HOME
           value: /helm-working-dir
         - name: HELM_DATA_HOME
           value: /helm-working-dir
-        image: quay.io/argoproj/argocd:v2.13.1
+        image: quay.io/argoproj/argocd:v3.2.0
         imagePullPolicy: IfNotPresent
         livenessProbe:
           failureThreshold: 3
           httpGet:
             path: /healthz?full=true
             port: metrics
           initialDelaySeconds: 10
           periodSeconds: 10
           successThreshold: 1
           timeoutSeconds: 1
@@ skipped 41 lines (1004 -> 1044) @@
           name: plugins
         - mountPath: /tmp
           name: tmp
       dnsPolicy: ClusterFirst
       initContainers:
       - command:
         - /bin/cp
         - -n
         - /usr/local/bin/argocd
         - /var/run/argocd/argocd-cmp-server
-        image: quay.io/argoproj/argocd:v2.13.1
+        image: quay.io/argoproj/argocd:v3.2.0
         imagePullPolicy: IfNotPresent
         name: copyutil
         resources: {}
         securityContext:
           allowPrivilegeEscalation: false
           capabilities:
             drop:
             - ALL
           readOnlyRootFilesystem: true
           runAsNonRoot: true
           seccompProfile:
             type: RuntimeDefault
         volumeMounts:
         - mountPath: /var/run/argocd
           name: var-files
+      nodeSelector:
+        kubernetes.io/os: linux
       serviceAccountName: argocd-helm-chart-repo-server
       terminationGracePeriodSeconds: 30
       volumes:
       - emptyDir: {}
         name: helm-working-dir
       - emptyDir: {}
         name: plugins
       - emptyDir: {}
         name: var-files
       - emptyDir: {}
@@ skipped 198 lines (1084 -> 1281) @@
```
#### Deployment/argocd-helm-chart-server (argocd)
```diff
             configMapKeyRef:
               key: server.connection.status.cache.expiration
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_SERVER_OIDC_CACHE_EXPIRATION
           valueFrom:
             configMapKeyRef:
               key: server.oidc.cache.expiration
               name: argocd-cmd-params-cm
               optional: true
-        - name: ARGOCD_SERVER_LOGIN_ATTEMPTS_EXPIRATION
-          valueFrom:
-            configMapKeyRef:
-              key: server.login.attempts.expiration
-              name: argocd-cmd-params-cm
-              optional: true
         - name: ARGOCD_SERVER_STATIC_ASSETS
           valueFrom:
             configMapKeyRef:
               key: server.staticassets
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APP_STATE_CACHE_EXPIRATION
           valueFrom:
             configMapKeyRef:
               key: server.app.state.cache.expiration
@@ skipped 21 lines (1308 -> 1328) @@
           valueFrom:
             secretKeyRef:
               key: redis-username
               name: argocd-redis
               optional: true
         - name: REDIS_PASSWORD
           valueFrom:
             secretKeyRef:
               key: auth
               name: argocd-redis
-              optional: true
+              optional: false
         - name: REDIS_SENTINEL_USERNAME
           valueFrom:
             secretKeyRef:
               key: redis-sentinel-username
               name: argocd-helm-chart-redis
               optional: true
         - name: REDIS_SENTINEL_PASSWORD
           valueFrom:
             secretKeyRef:
               key: redis-sentinel-password
@@ skipped 32 lines (1351 -> 1382) @@
         - name: ARGOCD_SERVER_OTLP_INSECURE
           valueFrom:
             configMapKeyRef:
               key: otlp.insecure
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_SERVER_OTLP_HEADERS
           valueFrom:
             configMapKeyRef:
               key: otlp.headers
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_SERVER_OTLP_ATTRS
+          valueFrom:
+            configMapKeyRef:
+              key: otlp.attrs
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATION_NAMESPACES
           valueFrom:
             configMapKeyRef:
               key: application.namespaces
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_SERVER_ENABLE_PROXY_EXTENSION
           valueFrom:
@@ skipped 42 lines (1409 -> 1450) @@
             configMapKeyRef:
               key: applicationsetcontroller.allowed.scm.providers
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_SCM_PROVIDERS
           valueFrom:
             configMapKeyRef:
               key: applicationsetcontroller.enable.scm.providers
               name: argocd-cmd-params-cm
               optional: true
-        image: quay.io/argoproj/argocd:v2.13.1
+        - name: ARGOCD_APPLICATIONSET_CONTROLLER_ENABLE_GITHUB_API_METRICS
+          valueFrom:
+            configMapKeyRef:
+              key: applicationsetcontroller.enable.github.api.metrics
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_HYDRATOR_ENABLED
+          valueFrom:
+            configMapKeyRef:
+              key: hydrator.enabled
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_SYNC_WITH_REPLACE_ALLOWED
+          valueFrom:
+            configMapKeyRef:
+              key: server.sync.replace.allowed
+              name: argocd-cmd-params-cm
+              optional: true
+        image: quay.io/argoproj/argocd:v3.2.0
         imagePullPolicy: IfNotPresent
         livenessProbe:
           failureThreshold: 3
           httpGet:
             path: /healthz?full=true
             port: server
           initialDelaySeconds: 10
           periodSeconds: 10
           successThreshold: 1
           timeoutSeconds: 1
@@ skipped 35 lines (1491 -> 1525) @@
           name: argocd-dex-server-tls
         - mountPath: /home/argocd
           name: plugins-home
         - mountPath: /shared/app/custom
           name: styles
         - mountPath: /tmp
           name: tmp
         - mountPath: /home/argocd/params
           name: argocd-cmd-params-cm
       dnsPolicy: ClusterFirst
+      nodeSelector:
+        kubernetes.io/os: linux
       serviceAccountName: argocd-server
       terminationGracePeriodSeconds: 30
       volumes:
       - emptyDir: {}
         name: plugins-home
       - emptyDir: {}
         name: tmp
       - configMap:
           name: argocd-ssh-known-hosts-cm
         name: ssh-known-hosts
@@ skipped 145 lines (1548 -> 1692) @@
```
#### StatefulSet/argocd-helm-chart-application-controller (argocd)
```diff
             configMapKeyRef:
               key: controller.log.format
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATION_CONTROLLER_LOGLEVEL
           valueFrom:
             configMapKeyRef:
               key: controller.log.level
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_LOG_FORMAT_TIMESTAMP
+          valueFrom:
+            configMapKeyRef:
+              key: log.format.timestamp
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_APPLICATION_CONTROLLER_METRICS_CACHE_EXPIRATION
           valueFrom:
             configMapKeyRef:
               key: controller.metrics.cache.expiration
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_TIMEOUT_SECONDS
           valueFrom:
             configMapKeyRef:
               key: controller.self.heal.timeout.seconds
@@ skipped 10 lines (1719 -> 1728) @@
             configMapKeyRef:
               key: controller.self.heal.backoff.factor
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_BACKOFF_CAP_SECONDS
           valueFrom:
             configMapKeyRef:
               key: controller.self.heal.backoff.cap.seconds
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_APPLICATION_CONTROLLER_SELF_HEAL_BACKOFF_COOLDOWN_SECONDS
+          valueFrom:
+            configMapKeyRef:
+              key: controller.self.heal.backoff.cooldown.seconds
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_SYNC_WAVE_DELAY
+          valueFrom:
+            configMapKeyRef:
+              key: controller.sync.wave.delay.seconds
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_APPLICATION_CONTROLLER_SYNC_TIMEOUT
+          valueFrom:
+            configMapKeyRef:
+              key: controller.sync.timeout.seconds
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_PLAINTEXT
           valueFrom:
             configMapKeyRef:
               key: controller.repo.server.plaintext
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATION_CONTROLLER_REPO_SERVER_STRICT_TLS
           valueFrom:
             configMapKeyRef:
               key: controller.repo.server.strict.tls
@@ skipped 33 lines (1767 -> 1799) @@
           valueFrom:
             secretKeyRef:
               key: redis-username
               name: argocd-redis
               optional: true
         - name: REDIS_PASSWORD
           valueFrom:
             secretKeyRef:
               key: auth
               name: argocd-redis
-              optional: true
+              optional: false
         - name: REDIS_SENTINEL_USERNAME
           valueFrom:
             secretKeyRef:
               key: redis-sentinel-username
               name: argocd-helm-chart-redis
               optional: true
         - name: REDIS_SENTINEL_PASSWORD
           valueFrom:
             secretKeyRef:
               key: redis-sentinel-password
@@ skipped 16 lines (1822 -> 1837) @@
             configMapKeyRef:
               key: otlp.insecure
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_APPLICATION_CONTROLLER_OTLP_HEADERS
           valueFrom:
             configMapKeyRef:
               key: otlp.headers
               name: argocd-cmd-params-cm
               optional: true
+        - name: ARGOCD_APPLICATION_CONTROLLER_OTLP_ATTRS
+          valueFrom:
+            configMapKeyRef:
+              key: otlp.attrs
+              name: argocd-cmd-params-cm
+              optional: true
         - name: ARGOCD_APPLICATION_NAMESPACES
           valueFrom:
             configMapKeyRef:
               key: application.namespaces
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_CONTROLLER_SHARDING_ALGORITHM
           valueFrom:
             configMapKeyRef:
               key: controller.sharding.algorithm
@@ skipped 22 lines (1864 -> 1885) @@
             configMapKeyRef:
               key: controller.diff.server.side
               name: argocd-cmd-params-cm
               optional: true
         - name: ARGOCD_IGNORE_NORMALIZER_JQ_TIMEOUT
           valueFrom:
             configMapKeyRef:
               key: controller.ignore.normalizer.jq.timeout
               name: argocd-cmd-params-cm
               optional: true
-        image: quay.io/argoproj/argocd:v2.13.1
+        - name: ARGOCD_HYDRATOR_ENABLED
+          valueFrom:
+            configMapKeyRef:
+              key: hydrator.enabled
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_CLUSTER_CACHE_BATCH_EVENTS_PROCESSING
+          valueFrom:
+            configMapKeyRef:
+              key: controller.cluster.cache.batch.events.processing
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_CLUSTER_CACHE_EVENTS_PROCESSING_INTERVAL
+          valueFrom:
+            configMapKeyRef:
+              key: controller.cluster.cache.events.processing.interval
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: ARGOCD_APPLICATION_CONTROLLER_COMMIT_SERVER
+          valueFrom:
+            configMapKeyRef:
+              key: commit.server
+              name: argocd-cmd-params-cm
+              optional: true
+        - name: KUBECACHEDIR
+          value: /tmp/kubecache
+        image: quay.io/argoproj/argocd:v3.2.0
         imagePullPolicy: IfNotPresent
         name: application-controller
         ports:
         - containerPort: 8082
           name: metrics
           protocol: TCP
         readinessProbe:
           failureThreshold: 3
           httpGet:
             path: /healthz
@@ skipped 12 lines (1934 -> 1945) @@
           runAsNonRoot: true
           seccompProfile:
             type: RuntimeDefault
         volumeMounts:
         - mountPath: /app/config/controller/tls
           name: argocd-repo-server-tls
         - mountPath: /home/argocd
           name: argocd-home
         - mountPath: /home/argocd/params
           name: argocd-cmd-params-cm
+        - mountPath: /tmp
+          name: argocd-application-controller-tmp
         workingDir: /home/argocd
       dnsPolicy: ClusterFirst
+      nodeSelector:
+        kubernetes.io/os: linux
       serviceAccountName: argocd-application-controller
       terminationGracePeriodSeconds: 30
       volumes:
       - emptyDir: {}
         name: argocd-home
+      - emptyDir: {}
+        name: argocd-application-controller-tmp
       - name: argocd-repo-server-tls
         secret:
           items:
           - key: tls.crt
             path: tls.crt
           - key: tls.key
             path: tls.key
           - key: ca.crt
             path: ca.crt
           optional: true
@@ skipped 99 lines (1979 -> 2077) @@
```
#### ClusterRole/argocd-helm-chart-server
```diff
   name: argocd-helm-chart-server
 rules:
 - apiGroups:
   - '*'
   resources:
   - '*'
   verbs:
   - delete
   - get
   - patch
-  - list
 - apiGroups:
   - ""
   resources:
   - events
   verbs:
   - list
   - create
 - apiGroups:
   - ""
   resources:
@@ skipped 115 lines (2099 -> 2213) @@
```
#### Role/argocd-helm-chart-application-controller (argocd)
```diff
   - secrets
   - configmaps
   verbs:
   - get
   - list
   - watch
 - apiGroups:
   - argoproj.io
   resources:
   - applications
+  - applicationsets
   - appprojects
   verbs:
   - create
   - get
   - list
   - watch
   - update
   - patch
   - delete
 - apiGroups:
@@ skipped 96 lines (2235 -> 2330) @@
```
#### Role/argocd-helm-chart-applicationset-controller (argocd)
```diff
   verbs:
   - get
   - list
   - watch
 - apiGroups:
   - coordination.k8s.io
   resources:
   - leases
   verbs:
   - create
-  - delete
+- apiGroups:
+  - coordination.k8s.io
+  resourceNames:
+  - 58ac56fa.applicationsets.argoproj.io
+  resources:
+  - leases
+  verbs:
   - get
-  - list
-  - patch
   - update
-  - watch
+  - create
```
#### Role/argocd-helm-chart-dex-server (argocd)
```diff
 apiVersion: rbac.authorization.k8s.io/v1
 kind: Role
 metadata:
   labels:
     app.kubernetes.io/component: dex-server
     app.kubernetes.io/instance: argocd-helm-chart
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: argocd-dex-server
     app.kubernetes.io/part-of: argocd
@@ skipped 275 lines (2365 -> 2639) @@
```
#### RoleBinding/argocd-helm-chart-server (argocd)
```diff
   name: argocd-helm-chart-server
 subjects:
 - kind: ServiceAccount
   name: argocd-server
   namespace: argocd
```
#### ConfigMap/argocd-cm (argocd)
```diff
 apiVersion: v1
 data:
   admin.enabled: "true"
   application.instanceLabelKey: argocd.argoproj.io/instance
+  application.sync.impersonation.enabled: "false"
   exec.enabled: "false"
-  server.rbac.log.enforce.enable: "false"
+  resource.customizations.ignoreResourceUpdates.ConfigMap: |
+    jqPathExpressions:
+      # Ignore the cluster-autoscaler status
+      - '.metadata.annotations."cluster-autoscaler.kubernetes.io/last-updated"'
+      # Ignore the annotation of the legacy Leases election
+      - '.metadata.annotations."control-plane.alpha.kubernetes.io/leader"'
+  resource.customizations.ignoreResourceUpdates.Endpoints: |
+    jsonPointers:
+      - /metadata
+      - /subsets
+  resource.customizations.ignoreResourceUpdates.all: |
+    jsonPointers:
+      - /status
+  resource.customizations.ignoreResourceUpdates.apps_ReplicaSet: |
+    jqPathExpressions:
+      - '.metadata.annotations."deployment.kubernetes.io/desired-replicas"'
+      - '.metadata.annotations."deployment.kubernetes.io/max-replicas"'
+      - '.metadata.annotations."rollout.argoproj.io/desired-replicas"'
+  resource.customizations.ignoreResourceUpdates.argoproj.io_Application: |
+    jqPathExpressions:
+      - '.metadata.annotations."notified.notifications.argoproj.io"'
+      - '.metadata.annotations."argocd.argoproj.io/refresh"'
+      - '.metadata.annotations."argocd.argoproj.io/hydrate"'
+      - '.operation'
+  resource.customizations.ignoreResourceUpdates.argoproj.io_Rollout: |
+    jqPathExpressions:
+      - '.metadata.annotations."notified.notifications.argoproj.io"'
+  resource.customizations.ignoreResourceUpdates.autoscaling_HorizontalPodAutoscaler: |
+    jqPathExpressions:
+      - '.metadata.annotations."autoscaling.alpha.kubernetes.io/behavior"'
+      - '.metadata.annotations."autoscaling.alpha.kubernetes.io/conditions"'
+      - '.metadata.annotations."autoscaling.alpha.kubernetes.io/metrics"'
+      - '.metadata.annotations."autoscaling.alpha.kubernetes.io/current-metrics"'
+  resource.customizations.ignoreResourceUpdates.discovery.k8s.io_EndpointSlice: |
+    jsonPointers:
+      - /metadata
+      - /endpoints
+      - /ports
+  resource.exclusions: |
+    ### Network resources created by the Kubernetes control plane and excluded to reduce the number of watched events and UI clutter
+    - apiGroups:
+      - ''
+      - discovery.k8s.io
+      kinds:
+      - Endpoints
+      - EndpointSlice
+    ### Internal Kubernetes resources excluded reduce the number of watched events
+    - apiGroups:
+      - coordination.k8s.io
+      kinds:
+      - Lease
+    ### Internal Kubernetes Authz/Authn resources excluded reduce the number of watched events
+    - apiGroups:
+      - authentication.k8s.io
+      - authorization.k8s.io
+      kinds:
+      - SelfSubjectReview
+      - TokenReview
+      - LocalSubjectAccessReview
+      - SelfSubjectAccessReview
+      - SelfSubjectRulesReview
+      - SubjectAccessReview
+    ### Intermediate Certificate Request excluded reduce the number of watched events
+    - apiGroups:
+      - certificates.k8s.io
+      kinds:
+      - CertificateSigningRequest
+    - apiGroups:
+      - cert-manager.io
+      kinds:
+      - CertificateRequest
+    ### Cilium internal resources excluded reduce the number of watched events and UI Clutter
+    - apiGroups:
+      - cilium.io
+      kinds:
+      - CiliumIdentity
+      - CiliumEndpoint
+      - CiliumEndpointSlice
+    ### Kyverno intermediate and reporting resources excluded reduce the number of watched events and improve performance
+    - apiGroups:
+      - kyverno.io
+      - reports.kyverno.io
+      - wgpolicyk8s.io
+      kinds:
+      - PolicyReport
+      - ClusterPolicyReport
+      - EphemeralReport
+      - ClusterEphemeralReport
+      - AdmissionReport
+      - ClusterAdmissionReport
+      - BackgroundScanReport
+      - ClusterBackgroundScanReport
+      - UpdateRequest
   statusbadge.enabled: "false"
   timeout.hard.reconciliation: 0s
   timeout.reconciliation: 180s
   url: https://argocd.example.com
 kind: ConfigMap
 metadata:
   labels:
     app.kubernetes.io/component: server
     app.kubernetes.io/instance: argocd-helm-chart
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: argocd-cm
     app.kubernetes.io/part-of: argocd
-    app.kubernetes.io/version: v2.13.1
-    helm.sh/chart: argo-cd-7.7.7
+    app.kubernetes.io/version: v3.2.0
+    helm.sh/chart: argo-cd-9.1.4
   name: argocd-cm
   namespace: argocd
```
#### ConfigMap/argocd-cmd-params-cm (argocd)
```diff
 apiVersion: v1
 data:
-  application.namespaces: ""
   applicationsetcontroller.enable.leader.election: "false"
-  applicationsetcontroller.enable.progressive.syncs: "false"
   applicationsetcontroller.log.format: text
   applicationsetcontroller.log.level: info
-  applicationsetcontroller.namespaces: ""
-  applicationsetcontroller.policy: sync
-  controller.ignore.normalizer.jq.timeout: 1s
+  commitserver.log.format: text
+  commitserver.log.level: info
   controller.log.format: text
   controller.log.level: info
-  controller.operation.processors: "10"
-  controller.repo.server.timeout.seconds: "60"
-  controller.self.heal.timeout.seconds: "5"
-  controller.status.processors: "20"
-  otlp.address: ""
+  dexserver.log.format: text
+  dexserver.log.level: info
+  notificationscontroller.log.format: text
+  notificationscontroller.log.level: info
   redis.server: argocd-helm-chart-redis:6379
   repo.server: argocd-helm-chart-repo-server:8081
   reposerver.log.format: text
   reposerver.log.level: info
-  reposerver.parallelism.limit: "0"
-  server.basehref: /
   server.dex.server: https://argocd-helm-chart-dex-server:5556
   server.dex.server.strict.tls: "false"
-  server.disable.auth: "false"
-  server.enable.gzip: "true"
-  server.enable.proxy.extension: "false"
-  server.insecure: "false"
   server.log.format: text
   server.log.level: info
   server.repo.server.strict.tls: "false"
-  server.rootpath: ""
-  server.staticassets: /shared/app
-  server.x.frame.options: sameorigin
 kind: ConfigMap
 metadata:
   labels:
     app.kubernetes.io/component: server
     app.kubernetes.io/instance: argocd-helm-chart
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: argocd-cmd-params-cm
     app.kubernetes.io/part-of: argocd
-    app.kubernetes.io/version: v2.13.1
-    helm.sh/chart: argo-cd-7.7.7
@@ skipped 524 lines (2816 -> 3339) @@
```
#### CustomResourceDefinition/applications.argoproj.io
```diff
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
@@ skipped 178 lines (3364 -> 3541) @@
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
@@ skipped 43 lines (3570 -> 3612) @@
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
                                 definition in the format [old_image_name=]<image_name>:<image_tag>
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
@@ skipped 51 lines (3655 -> 3705) @@
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
@@ skipped 199 lines (3730 -> 3928) @@
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
@@ skipped 45 lines (3957 -> 4001) @@
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
                                   image definition in the format [old_image_name=]<image_name>:<image_tag>
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
@@ skipped 51 lines (4044 -> 4094) @@
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
@@ skipped 312 lines (4119 -> 4430) @@
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
@@ skipped 42 lines (4459 -> 4500) @@
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
                             definition in the format [old_image_name=]<image_name>:<image_tag>
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
@@ skipped 51 lines (4543 -> 4593) @@
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
@@ skipped 46 lines (4618 -> 4663) @@
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
+                        pattern: ^.{2,}|[^./]$
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
@@ skipped 125 lines (4746 -> 4870) @@
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
@@ skipped 43 lines (4899 -> 4941) @@
                           type: array
                         forceCommonAnnotations:
                           description: ForceCommonAnnotations specifies whether to
                             force applying common annotations to resources for Kustomize
                             apps
🚨 Diff is too long
</details>

⚠️⚠️⚠️ Diff exceeds max length of 65536 characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag

_Stats_:
[Applications: 2], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
