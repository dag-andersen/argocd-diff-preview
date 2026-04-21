package argoapplication

import (
	"fmt"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestDuplicateGeneratedApplicationNames(t *testing.T) {
	apps := []ArgoResource{
		createGeneratedApplicationForTest("dupe", "set.yaml"),
		createGeneratedApplicationForTest("unique", "set.yaml"),
		createGeneratedApplicationForTest("dupe", "set.yaml"),
		createGeneratedApplicationForTest("dupe-2", "set.yaml"),
		createGeneratedApplicationForTest("dupe-2", "set.yaml"),
	}

	assert.Equal(t, []string{"dupe", "dupe-2"}, duplicateGeneratedApplicationNames(apps))
}

func TestValidateGeneratedApplicationNames(t *testing.T) {
	appSet := createGeneratedAppSetForTest("duplicate-generated-apps", "appset.yaml")
	branch := git.NewBranch("feature", git.Target)

	t.Run("warn mode continues", func(t *testing.T) {
		apps := []ArgoResource{
			createGeneratedApplicationForTest("dupe", "appset.yaml"),
			createGeneratedApplicationForTest("dupe", "appset.yaml"),
		}

		require.NoError(t, validateGeneratedApplicationNames(appSet, apps, branch, false))
	})

	t.Run("strict mode fails", func(t *testing.T) {
		apps := []ArgoResource{
			createGeneratedApplicationForTest("dupe", "appset.yaml"),
			createGeneratedApplicationForTest("dupe", "appset.yaml"),
			createGeneratedApplicationForTest("other", "appset.yaml"),
			createGeneratedApplicationForTest("other", "appset.yaml"),
		}

		err := validateGeneratedApplicationNames(appSet, apps, branch, true)
		require.EqualError(t, err, "ApplicationSet duplicate-generated-apps [t|appset.yaml] generated applications with duplicate names: dupe, other")
	})

	t.Run("same generated name in separate applicationsets is allowed", func(t *testing.T) {
		baseAppSet := createGeneratedAppSetForTest("base-appset", "base-appset.yaml")
		targetAppSet := createGeneratedAppSetForTest("target-appset", "target-appset.yaml")

		baseApps := []ArgoResource{createGeneratedApplicationForTest("app-1", "base-appset.yaml")}
		targetApps := []ArgoResource{createGeneratedApplicationForTest("app-1", "target-appset.yaml")}

		require.NoError(t, validateGeneratedApplicationNames(baseAppSet, baseApps, git.NewBranch("main", git.Base), true))
		require.NoError(t, validateGeneratedApplicationNames(targetAppSet, targetApps, git.NewBranch("feature", git.Target), true))
	})
}

func createGeneratedApplicationForTest(name, fileName string) ArgoResource {
	yamlStr := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ` + name + `
spec:
  destination:
    namespace: default`

	var y unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(yamlStr), &y); err != nil {
		panic(fmt.Sprintf("failed to unmarshal yaml in test: %v", err))
	}

	return ArgoResource{
		Yaml:     &y,
		Kind:     Application,
		Id:       name,
		Name:     name,
		FileName: fileName,
		Branch:   git.Target,
	}
}

func createGeneratedAppSetForTest(name, fileName string) ArgoResource {
	yamlStr := `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: ` + name

	var y unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(yamlStr), &y); err != nil {
		panic(fmt.Sprintf("failed to unmarshal yaml in test: %v", err))
	}

	return ArgoResource{
		Yaml:     &y,
		Kind:     ApplicationSet,
		Id:       name,
		Name:     name,
		FileName: fileName,
		Branch:   git.Target,
	}
}
