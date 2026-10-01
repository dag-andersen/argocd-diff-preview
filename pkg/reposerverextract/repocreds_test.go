package reposerverextract

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/db"
	argosettings "github.com/argoproj/argo-cd/v3/util/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeLookup struct {
	mu    sync.Mutex
	calls int
	repo  *v1alpha1.Repository
	err   error
}

func (f *fakeLookup) GetRepository(_ context.Context, repoURL, _ string) (*v1alpha1.Repository, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	r := *f.repo
	r.Repo = repoURL
	return &r, nil
}

func TestGetRepo_ExactRepositorySkipsLookup(t *testing.T) {
	const url = "https://charts.example.com/my-chart"
	lookup := &fakeLookup{repo: &v1alpha1.Repository{Username: "looked-up"}}
	rc := &RepoCreds{
		reposByURL: map[string]*v1alpha1.Repository{
			normalizeRepoURL(url): {Repo: url, Username: "exact-user"},
		},
		lookup: lookup,
	}

	assert.Equal(t, "exact-user", rc.GetRepo(url).Username)
	assert.Equal(t, 0, lookup.calls)
}

// A URL not seen at startup, as for a child Application found during
// app-of-apps traversal, is looked up once and then served from the cache.
func TestGetRepo_LooksUpUnseenURLOnce(t *testing.T) {
	lookup := &fakeLookup{repo: &v1alpha1.Repository{Username: "robot", Password: "token"}}
	rc := &RepoCreds{reposByURL: map[string]*v1alpha1.Repository{}, lookup: lookup}

	got := rc.GetRepo("https://github.com/org/child.git")
	assert.Equal(t, "robot", got.Username)
	assert.Equal(t, "token", got.Password)

	rc.GetRepo("https://github.com/org/child")
	assert.Equal(t, 1, lookup.calls, "URLs differing only by .git share a cache entry")
}

func TestGetRepo_LookupErrorReturnsStub(t *testing.T) {
	lookup := &fakeLookup{err: errors.New("boom")}
	rc := &RepoCreds{reposByURL: map[string]*v1alpha1.Repository{}, lookup: lookup}

	got := rc.GetRepo("https://charts.example.com/chart")
	assert.Equal(t, "https://charts.example.com/chart", got.Repo)
	assert.Empty(t, got.Username)
}

func TestGetRepo_NoLookupReturnsStub(t *testing.T) {
	rc := &RepoCreds{reposByURL: map[string]*v1alpha1.Repository{}}

	got := rc.GetRepo("https://charts.example.com/chart")
	assert.Equal(t, "https://charts.example.com/chart", got.Repo)
	assert.Empty(t, got.Username)
}

func TestGetRepo_NilRepoCreds(t *testing.T) {
	var rc *RepoCreds
	got := rc.GetRepo("https://github.com/org/repo")
	assert.Equal(t, "https://github.com/org/repo", got.Repo)
	assert.Empty(t, got.Username)
}

func TestGetRepo_ConcurrentLookups(t *testing.T) {
	lookup := &fakeLookup{repo: &v1alpha1.Repository{Username: "robot"}}
	rc := &RepoCreds{reposByURL: map[string]*v1alpha1.Repository{}, lookup: lookup}

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			assert.Equal(t, "robot", rc.GetRepo("https://charts.example.com/chart").Username)
		})
	}
	wg.Wait()
	assert.Equal(t, 1, lookup.calls)
}

// The real Argo CD lookup, after the context its settings informers were
// started with has ended (as it has by the time traversal finds children):
// a scheme-less OCI chart URL one level below a repo-creds template at the
// registry host, as with ECR.
func TestGetRepo_ArgoDBAppliesRepoCredsTemplateAfterContextEnds(t *testing.T) {
	const namespace = "argocd"
	const registryHost = "123456789012.dkr.ecr.us-east-1.amazonaws.com"
	const childChartURL = registryHost + "/mirror/charts/n8n"

	clientset := fake.NewClientset(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: "argocd-cm", Namespace: namespace,
			Labels: map[string]string{"app.kubernetes.io/part-of": "argocd"},
		}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: namespace}},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ecr-creds", Namespace: namespace,
				Labels: map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":       []byte(registryHost),
				"type":      []byte("helm"),
				"enableOCI": []byte("true"),
				"username":  []byte("AWS"),
				"password":  []byte("ecr-token"),
			},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	argoDB := db.NewDB(namespace, argosettings.NewSettingsManager(ctx, clientset, namespace), clientset)
	_, err := argoDB.ListRepositories(ctx) // syncs the informers, as FetchRepoCreds does
	require.NoError(t, err)
	cancel()

	rc := &RepoCreds{reposByURL: map[string]*v1alpha1.Repository{}, lookup: argoDB}
	got := rc.GetRepo(childChartURL)
	assert.Equal(t, "AWS", got.Username)
	assert.Equal(t, "ecr-token", got.Password)
	assert.True(t, got.EnableOCI)
	assert.Equal(t, childChartURL, got.Repo)
}
