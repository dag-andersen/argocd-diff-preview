package reposerverextract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestLocalRefSourcesAvailableIgnoresOptionalMissingValueFile(t *testing.T) {
	branchFolder := t.TempDir()
	primary := v1alpha1.ApplicationSource{
		Helm: &v1alpha1.ApplicationSourceHelm{
			ValueFiles:              []string{"$values/optional.yaml"},
			IgnoreMissingValueFiles: true,
		},
	}
	refs := []v1alpha1.ApplicationSource{{Ref: "values"}}

	available, reason := localRefSourcesAvailable(branchFolder, primary, refs)
	if !available {
		t.Fatalf("optional missing value file should keep the local render path available: %s", reason)
	}
}

func TestLocalRefSourcesAvailableRejectsRequiredMissingValueFile(t *testing.T) {
	branchFolder := t.TempDir()
	primary := v1alpha1.ApplicationSource{
		Helm: &v1alpha1.ApplicationSourceHelm{
			ValueFiles: []string{"$values/required.yaml"},
		},
	}
	refs := []v1alpha1.ApplicationSource{{Ref: "values"}}

	available, reason := localRefSourcesAvailable(branchFolder, primary, refs)
	if available {
		t.Fatal("required missing value file should make the local render path unavailable")
	}
	if !strings.Contains(reason, "required.yaml") || !strings.Contains(reason, "does not exist") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestLocalRefSourcesAvailableRejectsMissingRefSource(t *testing.T) {
	branchFolder := t.TempDir()
	primary := v1alpha1.ApplicationSource{}
	refs := []v1alpha1.ApplicationSource{{Ref: "values", Path: "missing-ref"}}

	available, reason := localRefSourcesAvailable(branchFolder, primary, refs)
	if available {
		t.Fatal("missing ref source should make the local render path unavailable")
	}
	if !strings.Contains(reason, "missing-ref") || !strings.Contains(reason, "does not exist") {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestLocalRefSourcesAvailableAcceptsExistingRequiredValueFile(t *testing.T) {
	branchFolder := t.TempDir()
	valueFile := filepath.Join(branchFolder, "required.yaml")
	if err := os.WriteFile(valueFile, []byte("key: value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	primary := v1alpha1.ApplicationSource{
		Helm: &v1alpha1.ApplicationSourceHelm{
			ValueFiles: []string{"$values/required.yaml"},
		},
	}
	refs := []v1alpha1.ApplicationSource{{Ref: "values"}}

	available, reason := localRefSourcesAvailable(branchFolder, primary, refs)
	if !available {
		t.Fatalf("existing required value file should keep the local render path available: %s", reason)
	}
}
