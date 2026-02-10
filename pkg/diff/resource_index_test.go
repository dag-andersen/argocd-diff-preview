package diff

import (
	"testing"
)

func TestResourceInfo_FormatHeader(t *testing.T) {
	tests := []struct {
		name     string
		info     *ResourceInfo
		expected string
	}{
		{
			name:     "nil resource",
			info:     nil,
			expected: "",
		},
		{
			name:     "empty resource",
			info:     &ResourceInfo{},
			expected: "",
		},
		{
			name: "kind and name only",
			info: &ResourceInfo{
				Kind: "Deployment",
				Name: "my-deploy",
			},
			expected: "@@ Resource: Deployment/my-deploy @@",
		},
		{
			name: "kind, name, and namespace",
			info: &ResourceInfo{
				Kind:      "Deployment",
				Name:      "my-deploy",
				Namespace: "default",
			},
			expected: "@@ Resource: Deployment/my-deploy (default) @@",
		},
		{
			name: "kind only",
			info: &ResourceInfo{
				Kind: "Namespace",
			},
			expected: "@@ Resource: Namespace @@",
		},
		{
			name: "kind and namespace, no name",
			info: &ResourceInfo{
				Kind:      "ConfigMap",
				Namespace: "kube-system",
			},
			expected: "@@ Resource: ConfigMap (kube-system) @@",
		},
		{
			name: "name only",
			info: &ResourceInfo{
				Name: "orphan-resource",
			},
			expected: "@@ Resource: orphan-resource @@",
		},
		{
			name: "cluster-scoped resource (no namespace)",
			info: &ResourceInfo{
				Kind: "ClusterRole",
				Name: "admin-role",
			},
			expected: "@@ Resource: ClusterRole/admin-role @@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.info.FormatHeader()
			if result != tt.expected {
				t.Errorf("FormatHeader() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBuildResourceIndex_SingleResource(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deploy
  namespace: default
spec:
  replicas: 3`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(idx.resources))
	}

	r := idx.resources[0]
	if r.startLine != 0 {
		t.Errorf("expected startLine 0, got %d", r.startLine)
	}
	if r.info.Kind != "Deployment" {
		t.Errorf("expected Kind 'Deployment', got %q", r.info.Kind)
	}
	if r.info.Name != "my-deploy" {
		t.Errorf("expected Name 'my-deploy', got %q", r.info.Name)
	}
	if r.info.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %q", r.info.Namespace)
	}
}

func TestBuildResourceIndex_MultipleResources(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deploy
  namespace: default
spec:
  replicas: 3
---
apiVersion: v1
kind: Service
metadata:
  name: my-svc
  namespace: default
spec:
  ports:
  - port: 80
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
  namespace: kube-system`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(idx.resources))
	}

	// First resource: Deployment
	if idx.resources[0].info.Kind != "Deployment" {
		t.Errorf("resource 0: expected Kind 'Deployment', got %q", idx.resources[0].info.Kind)
	}
	if idx.resources[0].info.Name != "my-deploy" {
		t.Errorf("resource 0: expected Name 'my-deploy', got %q", idx.resources[0].info.Name)
	}
	if idx.resources[0].startLine != 0 {
		t.Errorf("resource 0: expected startLine 0, got %d", idx.resources[0].startLine)
	}

	// Second resource: Service (starts after line 7 which is ---)
	if idx.resources[1].info.Kind != "Service" {
		t.Errorf("resource 1: expected Kind 'Service', got %q", idx.resources[1].info.Kind)
	}
	if idx.resources[1].info.Name != "my-svc" {
		t.Errorf("resource 1: expected Name 'my-svc', got %q", idx.resources[1].info.Name)
	}

	// Third resource: ServiceAccount
	if idx.resources[2].info.Kind != "ServiceAccount" {
		t.Errorf("resource 2: expected Kind 'ServiceAccount', got %q", idx.resources[2].info.Kind)
	}
	if idx.resources[2].info.Name != "my-sa" {
		t.Errorf("resource 2: expected Name 'my-sa', got %q", idx.resources[2].info.Name)
	}
	if idx.resources[2].info.Namespace != "kube-system" {
		t.Errorf("resource 2: expected Namespace 'kube-system', got %q", idx.resources[2].info.Namespace)
	}
}

func TestBuildResourceIndex_EmptyContent(t *testing.T) {
	idx := BuildResourceIndex("")

	if idx == nil {
		t.Fatal("expected non-nil ResourceIndex for empty content")
	}
	if len(idx.resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(idx.resources))
	}
}

func TestBuildResourceIndex_LeadingSeparator(t *testing.T) {
	yaml := `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(idx.resources))
	}

	if idx.resources[0].info.Kind != "ConfigMap" {
		t.Errorf("expected Kind 'ConfigMap', got %q", idx.resources[0].info.Kind)
	}
	if idx.resources[0].startLine != 1 {
		t.Errorf("expected startLine 1, got %d", idx.resources[0].startLine)
	}
}

func TestBuildResourceIndex_NoNamespace(t *testing.T) {
	yaml := `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: admin-role
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(idx.resources))
	}

	if idx.resources[0].info.Namespace != "" {
		t.Errorf("expected empty Namespace, got %q", idx.resources[0].info.Namespace)
	}
}

func TestBuildResourceIndex_QuotedValues(t *testing.T) {
	yaml := `apiVersion: v1
kind: ConfigMap
metadata:
  name: "quoted-name"
  namespace: 'quoted-ns'`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(idx.resources))
	}

	if idx.resources[0].info.Name != "quoted-name" {
		t.Errorf("expected Name 'quoted-name', got %q", idx.resources[0].info.Name)
	}
	if idx.resources[0].info.Namespace != "quoted-ns" {
		t.Errorf("expected Namespace 'quoted-ns', got %q", idx.resources[0].info.Namespace)
	}
}

func TestBuildResourceIndex_WithComments(t *testing.T) {
	yaml := `# This is a deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  # The name of the deployment
  name: commented-deploy
  namespace: default`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(idx.resources))
	}

	if idx.resources[0].info.Kind != "Deployment" {
		t.Errorf("expected Kind 'Deployment', got %q", idx.resources[0].info.Kind)
	}
	if idx.resources[0].info.Name != "commented-deploy" {
		t.Errorf("expected Name 'commented-deploy', got %q", idx.resources[0].info.Name)
	}
}

func TestGetResourceForLine(t *testing.T) {
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy-1
  namespace: ns1
spec:
  replicas: 3
---
apiVersion: v1
kind: Service
metadata:
  name: svc-1
  namespace: ns1`

	idx := BuildResourceIndex(yaml)

	tests := []struct {
		name         string
		lineNum      int
		expectedKind string
		expectedName string
	}{
		{
			name:         "first line of first resource",
			lineNum:      0,
			expectedKind: "Deployment",
			expectedName: "deploy-1",
		},
		{
			name:         "middle of first resource",
			lineNum:      3,
			expectedKind: "Deployment",
			expectedName: "deploy-1",
		},
		{
			name:         "last line of first resource",
			lineNum:      6,
			expectedKind: "Deployment",
			expectedName: "deploy-1",
		},
		{
			name:         "first line of second resource (after ---)",
			lineNum:      8,
			expectedKind: "Service",
			expectedName: "svc-1",
		},
		{
			name:         "last line of second resource",
			lineNum:      12,
			expectedKind: "Service",
			expectedName: "svc-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := idx.GetResourceForLine(tt.lineNum)
			if info == nil {
				t.Fatalf("expected non-nil ResourceInfo for line %d", tt.lineNum)
			}
			if info.Kind != tt.expectedKind {
				t.Errorf("line %d: expected Kind %q, got %q", tt.lineNum, tt.expectedKind, info.Kind)
			}
			if info.Name != tt.expectedName {
				t.Errorf("line %d: expected Name %q, got %q", tt.lineNum, tt.expectedName, info.Name)
			}
		})
	}
}

func TestGetResourceForLine_EmptyIndex(t *testing.T) {
	idx := BuildResourceIndex("")

	info := idx.GetResourceForLine(0)
	if info != nil {
		t.Errorf("expected nil for empty index, got %+v", info)
	}
}

func TestGetResourceForLine_NilIndex(t *testing.T) {
	var idx *ResourceIndex

	info := idx.GetResourceForLine(0)
	if info != nil {
		t.Errorf("expected nil for nil index, got %+v", info)
	}
}

func TestBuildResourceIndex_MalformedYAML(t *testing.T) {
	// Missing kind
	yaml := `apiVersion: v1
metadata:
  name: no-kind-resource`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(idx.resources))
	}

	// Should still capture what we can
	if idx.resources[0].info.Name != "no-kind-resource" {
		t.Errorf("expected Name 'no-kind-resource', got %q", idx.resources[0].info.Name)
	}
	if idx.resources[0].info.Kind != "" {
		t.Errorf("expected empty Kind, got %q", idx.resources[0].info.Kind)
	}
}

func TestBuildResourceIndex_NestedNameField(t *testing.T) {
	// Ensure we only capture the metadata.name, not nested name fields
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: correct-name
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: container-name
        image: nginx`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(idx.resources))
	}

	if idx.resources[0].info.Name != "correct-name" {
		t.Errorf("expected Name 'correct-name', got %q", idx.resources[0].info.Name)
	}
}

func TestBuildResourceIndex_RealWorldHelmOutput(t *testing.T) {
	// Simulates actual Helm template output structure
	yaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/instance: my-app
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/name: myApp
    app.kubernetes.io/version: 1.16.0
    helm.sh/chart: myApp-0.1.0
  name: super-app-name
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/instance: my-app
      app.kubernetes.io/name: myApp
  template:
    metadata:
      labels:
        app.kubernetes.io/instance: my-app
        app.kubernetes.io/managed-by: Helm
        app.kubernetes.io/name: myApp
    spec:
      containers:
      - image: nginx:1.16.0
        name: myapp
        ports:
        - containerPort: 80
          name: http
          protocol: TCP
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/instance: my-app
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/name: myApp
  name: super-app-name
  namespace: default
spec:
  ports:
  - name: http
    port: 80
    protocol: TCP
    targetPort: http
  selector:
    app.kubernetes.io/instance: my-app
    app.kubernetes.io/name: myApp
  type: ClusterIP
---
apiVersion: v1
automountServiceAccountToken: true
kind: ServiceAccount
metadata:
  labels:
    app.kubernetes.io/instance: my-app
    app.kubernetes.io/managed-by: Helm
    app.kubernetes.io/name: myApp
  name: super-app-name
  namespace: default`

	idx := BuildResourceIndex(yaml)

	if len(idx.resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(idx.resources))
	}

	// Verify all resources parsed correctly
	expected := []struct {
		kind      string
		name      string
		namespace string
	}{
		{"Deployment", "super-app-name", "default"},
		{"Service", "super-app-name", "default"},
		{"ServiceAccount", "super-app-name", "default"},
	}

	for i, exp := range expected {
		if idx.resources[i].info.Kind != exp.kind {
			t.Errorf("resource %d: expected Kind %q, got %q", i, exp.kind, idx.resources[i].info.Kind)
		}
		if idx.resources[i].info.Name != exp.name {
			t.Errorf("resource %d: expected Name %q, got %q", i, exp.name, idx.resources[i].info.Name)
		}
		if idx.resources[i].info.Namespace != exp.namespace {
			t.Errorf("resource %d: expected Namespace %q, got %q", i, exp.namespace, idx.resources[i].info.Namespace)
		}
	}
}
