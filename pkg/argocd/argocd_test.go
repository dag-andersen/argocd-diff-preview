package argocd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	argocdconfig "github.com/dag-andersen/argocd-diff-preview/argocd-config"
	"helm.sh/helm/v3/pkg/cli"
)

func TestIsOCIChartURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"plain oci url", "oci://ghcr.io/argoproj/argo-helm/argo-cd", true},
		{"oci url with leading whitespace", "  oci://ghcr.io/x", true},
		{"oci url with trailing whitespace", "oci://ghcr.io/x  ", true},
		{"https url", "https://argoproj.github.io/argo-helm", false},
		{"http url", "http://example.com/charts", false},
		{"empty string", "", false},
		{"uppercase scheme is not recognized", "OCI://ghcr.io/x", false},
		{"oci substring but not scheme", "https://oci.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOCIChartURL(tt.url); got != tt.want {
				t.Errorf("isOCIChartURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestOCIHost(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantHost  string
		wantError bool
	}{
		{"host with path", "oci://ghcr.io/argoproj/argo-helm/argo-cd", "ghcr.io", false},
		{"host only, no path", "oci://ghcr.io", "ghcr.io", false},
		{"host with port", "oci://registry.local:5000/charts/argo-cd", "registry.local:5000", false},
		{"leading and trailing whitespace", "  oci://ghcr.io/x  ", "ghcr.io", false},
		{"empty after scheme", "oci://", "", true},
		{"missing scheme is not validated", "ghcr.io/x", "ghcr.io", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ociHost(tt.url)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.url)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.url, err)
				return
			}
			if got != tt.wantHost {
				t.Errorf("ociHost(%q) = %q, want %q", tt.url, got, tt.wantHost)
			}
		})
	}
}

// TestInstallWithHelm_OCIRequiresExplicitVersion exercises the early-return
// guardrail in installWithHelm. The check fires before any Helm/k8s setup, so
// the test does not need a cluster — only the OCI URL and an empty Version
// (which is what triggers "install latest").
func TestInstallWithHelm_OCIRequiresExplicitVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"empty version", ""},
		{"latest keyword", "latest"},
		{"whitespace only", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ArgoCDInstallation{
				ChartURL: "oci://ghcr.io/argoproj/argo-helm/argo-cd",
				Version:  tt.version,
			}
			err := a.installWithHelm()
			if err == nil {
				t.Fatal("expected error for OCI without --argocd-chart-version, got nil")
			}
			if !strings.Contains(err.Error(), "explicit --argocd-chart-version is required") {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestFindValuesFiles(t *testing.T) {
	tests := []struct {
		name       string
		files      map[string]string
		wantLabels []string
	}{
		{
			name:       "empty config dir uses embedded override",
			files:      map[string]string{},
			wantLabels: []string{embeddedValuesOverrideLabel},
		},
		{
			name: "values file loads before embedded override",
			files: map[string]string{
				valuesFileName: "configs: {}\n",
			},
			wantLabels: []string{valuesFileName, embeddedValuesOverrideLabel},
		},
		{
			name: "user override loads after embedded override",
			files: map[string]string{
				valuesFileName:         "configs: {}\n",
				valuesOverrideFileName: "dex:\n  enabled: true\n",
			},
			wantLabels: []string{valuesFileName, embeddedValuesOverrideLabel, valuesOverrideFileName},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := t.TempDir()
			for fileName, contents := range tt.files {
				if err := os.WriteFile(filepath.Join(configPath, fileName), []byte(contents), 0o644); err != nil {
					t.Fatalf("failed to write %s: %v", fileName, err)
				}
			}

			a := &ArgoCDInstallation{ConfigPath: configPath}
			got, err := a.findValuesFiles()
			if err != nil {
				t.Fatalf("findValuesFiles returned error: %v", err)
			}

			gotLabels := make([]string, 0, len(got))
			for _, valueFile := range got {
				if valueFile == embeddedValuesOverrideLabel {
					gotLabels = append(gotLabels, valueFile)
					continue
				}
				gotLabels = append(gotLabels, filepath.Base(valueFile))
			}

			if !reflect.DeepEqual(gotLabels, tt.wantLabels) {
				t.Fatalf("findValuesFiles labels = %#v, want %#v", gotLabels, tt.wantLabels)
			}
		})
	}
}

func TestFindValuesFiles_MissingConfigDirUsesEmbeddedOverride(t *testing.T) {
	a := &ArgoCDInstallation{ConfigPath: filepath.Join(t.TempDir(), "missing")}

	got, err := a.findValuesFiles()
	if err != nil {
		t.Fatalf("findValuesFiles returned error: %v", err)
	}

	want := []string{embeddedValuesOverrideLabel}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findValuesFiles = %#v, want %#v", got, want)
	}
}

func TestMergeValues_UserOverrideWinsOverEmbeddedOverride(t *testing.T) {
	configPath := t.TempDir()

	valuesContent := []byte(`configs:
  cm:
    kustomize.buildOptions: --enable-helm
`)
	if err := os.WriteFile(filepath.Join(configPath, valuesFileName), valuesContent, 0o644); err != nil {
		t.Fatalf("failed to write values file: %v", err)
	}

	valuesOverrideContent := []byte(`dex:
  enabled: true
applicationSet:
  replicas: 2
configs:
  params:
    hydrator.enabled: "true"
`)
	if err := os.WriteFile(filepath.Join(configPath, valuesOverrideFileName), valuesOverrideContent, 0o644); err != nil {
		t.Fatalf("failed to write override values file: %v", err)
	}

	a := &ArgoCDInstallation{ConfigPath: configPath}
	valuesFiles, err := a.findValuesFiles()
	if err != nil {
		t.Fatalf("findValuesFiles returned error: %v", err)
	}

	got, err := a.mergeValues(cli.New(), valuesFiles)
	if err != nil {
		t.Fatalf("mergeValues returned error: %v", err)
	}

	assertNestedValue(t, got, true, "dex", "enabled")
	assertNestedValue(t, got, int64(2), "applicationSet", "replicas")
	assertNestedValue(t, got, "true", "configs", "params", "hydrator.enabled")
	assertNestedValue(t, got, "--enable-helm", "configs", "cm", "kustomize.buildOptions")
}

func TestEmbeddedValuesOverrideMatchesDefaultConfig(t *testing.T) {
	repoValues, err := os.ReadFile(filepath.Join("..", "..", "argocd-config", valuesOverrideFileName))
	if err != nil {
		t.Fatalf("failed to read repository values override: %v", err)
	}

	if string(argocdconfig.ValuesOverride) != string(repoValues) {
		t.Fatal("embedded values override differs from argocd-config/values-override.yaml")
	}
}

func assertNestedValue(t *testing.T, values map[string]interface{}, want interface{}, path ...string) {
	t.Helper()

	var current interface{} = values
	for _, key := range path {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			t.Fatalf("path %v hit non-map value %#v", path, current)
		}
		current = currentMap[key]
	}

	if !reflect.DeepEqual(current, want) {
		if fmt.Sprint(current) == fmt.Sprint(want) {
			return
		}
		t.Fatalf("path %v = %#v, want %#v", path, current, want)
	}
}
