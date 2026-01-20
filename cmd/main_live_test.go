package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/app_selector"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/diff"
	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
)

type fakeRemoteClient struct {
	receivedMatchNames []string
	liveManifests      []argocd.LiveAppManifest
}

func (f *fakeRemoteClient) MatchApplicationsByName(targetAppNames []string) (map[string]string, []string, error) {
	f.receivedMatchNames = append([]string{}, targetAppNames...)
	matches := make(map[string]string)
	for _, name := range targetAppNames {
		matches[name] = name
	}
	return matches, nil, nil
}

func (f *fakeRemoteClient) FetchLiveManifestsForApps(appNames []string) ([]argocd.LiveAppManifest, error) {
	if f.liveManifests != nil {
		return f.liveManifests, nil
	}
	var results []argocd.LiveAppManifest
	for _, name := range appNames {
		results = append(results, argocd.LiveAppManifest{
			Name:      name,
			Manifests: []unstructured.Unstructured{},
		})
	}
	return results, nil
}

func TestRunLiveComparison_MatchesAfterAppSetConversion(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	targetDir := filepath.Join(tempDir, "target-branch")
	require.NoError(t, os.MkdirAll(targetDir, 0755))

	appYaml := []byte(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app-from-file
spec:
  destination:
    namespace: default
    server: https://kubernetes.default.svc
  source:
    repoURL: https://github.com/phantom/infra
    path: charts/app
    targetRevision: main
`)
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "app.yaml"), appYaml, 0644))

	cfg := &Config{
		CompareLive:  true,
		TargetBranch: "feature",
		BaseBranch:   "main",
		Repo:         "phantom/infra",
		OutputFolder: filepath.Join(tempDir, "output"),
	}

	convertedApp := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name": "expanded-app",
			},
		},
	}

	fakeRemote := &fakeRemoteClient{}

	deps := liveComparisonDeps{
		listChangedFiles: func(folder1 string, folder2 string) ([]string, time.Duration, error) {
			return nil, 0, nil
		},
		createFolder: func(path string, clear bool) error { return nil },
		writeNoAppsFound: func(title string, outputFolder string, selectors []app_selector.Selector, changedFiles []string) error {
			return nil
		},
		newRemoteArgoCD: func(url string, token string, insecure bool) remoteArgoCDClient { return fakeRemote },
		newK8sClient:    func() (*utils.K8sClient, error) { return &utils.K8sClient{}, nil },
		deleteOldApplications: func(client *utils.K8sClient, namespace string, ageInMinutes int) error {
			return nil
		},
		newLocalArgoCD: func(client *utils.K8sClient, cfg *Config) *argocd.ArgoCDInstallation {
			return &argocd.ArgoCDInstallation{}
		},
		installArgoCD: func(argocd *argocd.ArgoCDInstallation, debug bool, secretsFolder string) (time.Duration, error) {
			return 0, nil
		},
		loginArgoCD: func(argocd *argocd.ArgoCDInstallation) (time.Duration, error) {
			return 0, nil
		},
		convertAppSets: func(argocd *argocd.ArgoCDInstallation, apps *argoapplication.ArgoSelection, branch *git.Branch, repo string, tempFolder string, redirectRevisions []string, debug bool, appSelectionOptions argoapplication.ApplicationSelectionOptions) (*argoapplication.ArgoSelection, time.Duration, error) {
			return &argoapplication.ArgoSelection{
				SelectedApps: []argoapplication.ArgoResource{
					*argoapplication.NewArgoResource(&convertedApp, argoapplication.Application, "expanded-app", "expanded-app", "app.yaml", git.Target),
				},
				SkippedApps: []argoapplication.ArgoResource{},
			}, 0, nil
		},
		renderApps: func(argocd *argocd.ArgoCDInstallation, timeout uint64, baseApps []argoapplication.ArgoResource, targetApps []argoapplication.ArgoResource, prefix string, deleteAfterProcessing bool) ([]extract.ExtractedApp, []extract.ExtractedApp, time.Duration, error) {
			targetExtracted := extract.ExtractedApp{
				Id:         "expanded-app",
				Name:       "expanded-app",
				SourcePath: "app.yaml",
				Manifest: []unstructured.Unstructured{
					{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "expanded-app"}}},
				},
				Branch: git.Target,
			}
			return nil, []extract.ExtractedApp{targetExtracted}, 0, nil
		},
		generateDiff: func(title string, outputFolder string, baseBranch *git.Branch, targetBranch *git.Branch, baseApps []diff.AppInfo, targetApps []diff.AppInfo, diffIgnoreRegex *string, lineCount uint, maxCharCount uint, hideDeletedAppDiff bool, statsInfo diff.StatsInfo, selectionInfo diff.SelectionInfo) error {
			return nil
		},
	}

	require.NoError(t, runLiveComparisonWithDeps(cfg, deps))
	assert.Equal(t, []string{"expanded-app"}, fakeRemote.receivedMatchNames)
}

func TestRunLiveComparison_AutoDetectFilesChangedUsesListChangedFiles(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	targetDir := filepath.Join(tempDir, "target-branch")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "app.yaml"), []byte("apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: test\nspec: {}\n"), 0644))

	cfg := &Config{
		CompareLive:            true,
		TargetBranch:           "feature",
		BaseBranch:             "main",
		Repo:                   "phantom/infra",
		OutputFolder:           filepath.Join(tempDir, "output"),
		AutoDetectFilesChanged: true,
		DryRun:                 true,
	}

	called := false
	deps := liveComparisonDeps{
		listChangedFiles: func(folder1 string, folder2 string) ([]string, time.Duration, error) {
			called = true
			return []string{"app.yaml"}, 0, nil
		},
		createFolder:     func(path string, clear bool) error { return nil },
		writeNoAppsFound: func(title string, outputFolder string, selectors []app_selector.Selector, changedFiles []string) error { return nil },
		newRemoteArgoCD:  func(url string, token string, insecure bool) remoteArgoCDClient { return &fakeRemoteClient{} },
		newK8sClient:     func() (*utils.K8sClient, error) { return &utils.K8sClient{}, nil },
		deleteOldApplications: func(client *utils.K8sClient, namespace string, ageInMinutes int) error {
			return nil
		},
		newLocalArgoCD: func(client *utils.K8sClient, cfg *Config) *argocd.ArgoCDInstallation {
			return &argocd.ArgoCDInstallation{}
		},
		installArgoCD: func(argocd *argocd.ArgoCDInstallation, debug bool, secretsFolder string) (time.Duration, error) {
			return 0, nil
		},
		loginArgoCD: func(argocd *argocd.ArgoCDInstallation) (time.Duration, error) {
			return 0, nil
		},
		convertAppSets: func(argocd *argocd.ArgoCDInstallation, apps *argoapplication.ArgoSelection, branch *git.Branch, repo string, tempFolder string, redirectRevisions []string, debug bool, appSelectionOptions argoapplication.ApplicationSelectionOptions) (*argoapplication.ArgoSelection, time.Duration, error) {
			return apps, 0, nil
		},
		renderApps: func(argocd *argocd.ArgoCDInstallation, timeout uint64, baseApps []argoapplication.ArgoResource, targetApps []argoapplication.ArgoResource, prefix string, deleteAfterProcessing bool) ([]extract.ExtractedApp, []extract.ExtractedApp, time.Duration, error) {
			return nil, nil, 0, nil
		},
		generateDiff: func(title string, outputFolder string, baseBranch *git.Branch, targetBranch *git.Branch, baseApps []diff.AppInfo, targetApps []diff.AppInfo, diffIgnoreRegex *string, lineCount uint, maxCharCount uint, hideDeletedAppDiff bool, statsInfo diff.StatsInfo, selectionInfo diff.SelectionInfo) error {
			return nil
		},
	}

	require.NoError(t, runLiveComparisonWithDeps(cfg, deps))
	assert.True(t, called)
}

func TestRunLiveComparison_RemoteAPIIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/applications":
			_, _ = w.Write([]byte(`{"items":[{"metadata":{"name":"app1"}}]}`))
		case "/api/v1/applications/app1/manifests":
			_, _ = w.Write([]byte(`{"manifests":["{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"app1\"}}"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	targetDir := filepath.Join(tempDir, "target-branch")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "app.yaml"), []byte("apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: app1\nspec: {}\n"), 0644))

	cfg := &Config{
		CompareLive:  true,
		TargetBranch: "feature",
		BaseBranch:   "main",
		Repo:         "phantom/infra",
		OutputFolder: filepath.Join(tempDir, "output"),
		LiveArgocdURL:   server.URL,
		LiveArgocdToken: "token",
	}

	convertedApp := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name": "app1",
			},
		},
	}

	var diffCalled bool
	deps := liveComparisonDeps{
		listChangedFiles: func(folder1 string, folder2 string) ([]string, time.Duration, error) {
			return nil, 0, nil
		},
		createFolder:     func(path string, clear bool) error { return nil },
		writeNoAppsFound: func(title string, outputFolder string, selectors []app_selector.Selector, changedFiles []string) error { return nil },
		newRemoteArgoCD:  func(url string, token string, insecure bool) remoteArgoCDClient { return argocd.NewRemoteArgoCD(url, token, insecure) },
		newK8sClient:     func() (*utils.K8sClient, error) { return &utils.K8sClient{}, nil },
		deleteOldApplications: func(client *utils.K8sClient, namespace string, ageInMinutes int) error {
			return nil
		},
		newLocalArgoCD: func(client *utils.K8sClient, cfg *Config) *argocd.ArgoCDInstallation {
			return &argocd.ArgoCDInstallation{}
		},
		installArgoCD: func(argocd *argocd.ArgoCDInstallation, debug bool, secretsFolder string) (time.Duration, error) {
			return 0, nil
		},
		loginArgoCD: func(argocd *argocd.ArgoCDInstallation) (time.Duration, error) {
			return 0, nil
		},
		convertAppSets: func(argocd *argocd.ArgoCDInstallation, apps *argoapplication.ArgoSelection, branch *git.Branch, repo string, tempFolder string, redirectRevisions []string, debug bool, appSelectionOptions argoapplication.ApplicationSelectionOptions) (*argoapplication.ArgoSelection, time.Duration, error) {
			return &argoapplication.ArgoSelection{
				SelectedApps: []argoapplication.ArgoResource{
					*argoapplication.NewArgoResource(&convertedApp, argoapplication.Application, "app1", "app1", "app.yaml", git.Target),
				},
				SkippedApps: []argoapplication.ArgoResource{},
			}, 0, nil
		},
		renderApps: func(argocd *argocd.ArgoCDInstallation, timeout uint64, baseApps []argoapplication.ArgoResource, targetApps []argoapplication.ArgoResource, prefix string, deleteAfterProcessing bool) ([]extract.ExtractedApp, []extract.ExtractedApp, time.Duration, error) {
			targetExtracted := extract.ExtractedApp{
				Id:         "app1",
				Name:       "app1",
				SourcePath: "app.yaml",
				Manifest: []unstructured.Unstructured{
					{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "app1"}}},
				},
				Branch: git.Target,
			}
			return nil, []extract.ExtractedApp{targetExtracted}, 0, nil
		},
		generateDiff: func(title string, outputFolder string, baseBranch *git.Branch, targetBranch *git.Branch, baseApps []diff.AppInfo, targetApps []diff.AppInfo, diffIgnoreRegex *string, lineCount uint, maxCharCount uint, hideDeletedAppDiff bool, statsInfo diff.StatsInfo, selectionInfo diff.SelectionInfo) error {
			diffCalled = true
			assert.Equal(t, "app1", baseApps[0].Name)
			assert.Equal(t, "app1", targetApps[0].Name)
			return nil
		},
	}

	require.NoError(t, runLiveComparisonWithDeps(cfg, deps))
	assert.True(t, diffCalled)
}

func TestRunLiveComparison_SelectionInfoCountsSkippedKinds(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	targetDir := filepath.Join(tempDir, "target-branch")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(targetDir, "app.yaml"), []byte("apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: app1\nspec: {}\n"), 0644))

	cfg := &Config{
		CompareLive:  true,
		TargetBranch: "feature",
		BaseBranch:   "main",
		Repo:         "phantom/infra",
		OutputFolder: filepath.Join(tempDir, "output"),
	}

	convertedApp := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name": "app1",
			},
		},
	}
	skippedApp := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]any{
				"name": "skipped-app",
			},
		},
	}
	skippedAppSet := unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "ApplicationSet",
			"metadata": map[string]any{
				"name": "skipped-appset",
			},
		},
	}

	deps := liveComparisonDeps{
		listChangedFiles: func(folder1 string, folder2 string) ([]string, time.Duration, error) {
			return nil, 0, nil
		},
		createFolder:     func(path string, clear bool) error { return nil },
		writeNoAppsFound: func(title string, outputFolder string, selectors []app_selector.Selector, changedFiles []string) error { return nil },
		newRemoteArgoCD:  func(url string, token string, insecure bool) remoteArgoCDClient { return &fakeRemoteClient{} },
		newK8sClient:     func() (*utils.K8sClient, error) { return &utils.K8sClient{}, nil },
		deleteOldApplications: func(client *utils.K8sClient, namespace string, ageInMinutes int) error {
			return nil
		},
		newLocalArgoCD: func(client *utils.K8sClient, cfg *Config) *argocd.ArgoCDInstallation {
			return &argocd.ArgoCDInstallation{}
		},
		installArgoCD: func(argocd *argocd.ArgoCDInstallation, debug bool, secretsFolder string) (time.Duration, error) {
			return 0, nil
		},
		loginArgoCD: func(argocd *argocd.ArgoCDInstallation) (time.Duration, error) {
			return 0, nil
		},
		convertAppSets: func(argocd *argocd.ArgoCDInstallation, apps *argoapplication.ArgoSelection, branch *git.Branch, repo string, tempFolder string, redirectRevisions []string, debug bool, appSelectionOptions argoapplication.ApplicationSelectionOptions) (*argoapplication.ArgoSelection, time.Duration, error) {
			return &argoapplication.ArgoSelection{
				SelectedApps: []argoapplication.ArgoResource{
					*argoapplication.NewArgoResource(&convertedApp, argoapplication.Application, "app1", "app1", "app.yaml", git.Target),
				},
				SkippedApps: []argoapplication.ArgoResource{
					*argoapplication.NewArgoResource(&skippedApp, argoapplication.Application, "skipped-app", "skipped-app", "skipped.yaml", git.Target),
					*argoapplication.NewArgoResource(&skippedAppSet, argoapplication.ApplicationSet, "skipped-appset", "skipped-appset", "skipped-appset.yaml", git.Target),
				},
			}, 0, nil
		},
		renderApps: func(argocd *argocd.ArgoCDInstallation, timeout uint64, baseApps []argoapplication.ArgoResource, targetApps []argoapplication.ArgoResource, prefix string, deleteAfterProcessing bool) ([]extract.ExtractedApp, []extract.ExtractedApp, time.Duration, error) {
			targetExtracted := extract.ExtractedApp{
				Id:         "app1",
				Name:       "app1",
				SourcePath: "app.yaml",
				Manifest: []unstructured.Unstructured{
					{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "app1"}}},
				},
				Branch: git.Target,
			}
			return nil, []extract.ExtractedApp{targetExtracted}, 0, nil
		},
		generateDiff: func(title string, outputFolder string, baseBranch *git.Branch, targetBranch *git.Branch, baseApps []diff.AppInfo, targetApps []diff.AppInfo, diffIgnoreRegex *string, lineCount uint, maxCharCount uint, hideDeletedAppDiff bool, statsInfo diff.StatsInfo, selectionInfo diff.SelectionInfo) error {
			assert.Equal(t, 1, selectionInfo.Target.SkippedApplications)
			assert.Equal(t, 1, selectionInfo.Target.SkippedApplicationSets)
			return nil
		},
	}

	require.NoError(t, runLiveComparisonWithDeps(cfg, deps))
}
