package repository

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesRepo(t *testing.T) {
	cases := []struct {
		name    string
		repoURL string
		repo    string
		want    bool
	}{
		{
			name:    "full URL matches full URL",
			repoURL: "https://github.com/org/repo.git",
			repo:    "https://github.com/org/repo.git",
			want:    true,
		},
		{
			name:    "slug matches full URL",
			repoURL: "https://github.com/org/repo.git",
			repo:    "org/repo",
			want:    true,
		},
		{
			name:    "case insensitive",
			repoURL: "https://github.com/Org/Repo.git",
			repo:    "org/repo",
			want:    true,
		},
		{
			name:    "SSH URL matches slug",
			repoURL: "git@github.com:org/repo.git",
			repo:    "org/repo",
			want:    true,
		},
		{
			name:    "GitLab nested group matches full path",
			repoURL: "https://gitlab.example.com/platform/team/repo.git",
			repo:    "platform/team/repo",
			want:    true,
		},
		{
			name:    "Bitbucket full URL with slug",
			repoURL: "https://bitbucket.org/team/repo.git",
			repo:    "team/repo",
			want:    true,
		},
		{
			name:    "different repos do not match",
			repoURL: "https://github.com/org/repo-a.git",
			repo:    "org/repo-b",
			want:    false,
		},
		{
			name:    "repo prefix does not match different repo",
			repoURL: "https://github.com/org/helm-charts-deploy.git",
			repo:    "org/helm-charts",
			want:    false,
		},
		{
			name:    "repo name containing repo does not match",
			repoURL: "https://github.com/org/monorepo-values.git",
			repo:    "org/monorepo",
			want:    false,
		},
		{
			name:    "different orgs do not match",
			repoURL: "https://github.com/org-a/repo.git",
			repo:    "org-b/repo",
			want:    false,
		},
		{
			name:    "empty repo does not match",
			repoURL: "https://github.com/org/repo.git",
			repo:    "",
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, matchesRepo(tc.repoURL, tc.repo), "repoURL=%q repo=%q", tc.repoURL, tc.repo)
		})
	}
}

func TestSelectorWithRegex(t *testing.T) {
	cases := []struct {
		name    string
		regex   string
		repoURL string
	}{
		{
			name:    "GitHub SSH URL",
			regex:   `^git@github\.com:my-org/my-repo-[^/]+-overrides$`,
			repoURL: "git@github.com:my-org/my-repo-{{.metadata.annotations.repo}}-overrides.git",
		},
		{
			name:    "GitLab HTTPS URL with nested groups",
			regex:   `^https://gitlab\.example\.com/platform/team/my-repo-[^/]+-overrides$`,
			repoURL: "https://gitlab.example.com/platform/team/my-repo-{{.metadata.annotations.repo}}-overrides.git",
		},
		{
			name:    "GitLab SSH URL with nested groups",
			regex:   `^git@gitlab\.example\.com:platform/team/my-repo-[^/]+-overrides$`,
			repoURL: "git@gitlab.example.com:platform/team/my-repo-{{.metadata.annotations.repo}}-overrides.git",
		},
		{
			name:    "Bitbucket HTTPS URL",
			regex:   `^https://bitbucket\.org/team/my-repo-[^/]+-overrides$`,
			repoURL: "https://bitbucket.org/team/my-repo-{{.metadata.annotations.repo}}-overrides.git",
		},
		{
			name:    "Azure DevOps HTTPS URL",
			regex:   `^https://dev\.azure\.com/org/project/_git/my-repo-[^/]+-overrides$`,
			repoURL: "https://dev.azure.com/org/project/_git/my-repo-{{.metadata.annotations.repo}}-overrides.git",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selector, err := NewSelector("org/repo", tc.regex)
			assert.NoError(t, err)

			assert.True(t, selector.Matches(tc.repoURL))
			assert.False(t, selector.Matches(strings.TrimSuffix(tc.repoURL, "-overrides.git")+"-overrides-extra.git"))
			assert.False(t, selector.Matches("https://github.com/org/repo.git"), "repo-regex overrides default repo matching")
		})
	}
}

func TestNewSelectorInvalidRegex(t *testing.T) {
	_, err := NewSelector("org/repo", "[")
	assert.Error(t, err)
}
