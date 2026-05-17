package repository

import (
	"regexp"
	"strings"
)

type Selector struct {
	Repo  string
	Regex *regexp.Regexp
}

func NewSelector(repo, regex string) (*Selector, error) {
	selector := &Selector{Repo: repo}
	if strings.TrimSpace(regex) != "" {
		compiled, err := regexp.Compile(regex)
		if err != nil {
			return nil, err
		}
		selector.Regex = compiled
	}
	return selector, nil
}

func (s *Selector) Matches(repoURL string) bool {
	if s == nil {
		return false
	}
	if s.Regex != nil {
		return s.Regex.MatchString(normalizeRepoMatchInput(repoURL))
	}
	if s.Repo == "" {
		return true
	}
	return MatchesRepo(repoURL, s.Repo)
}

func (s *Selector) String() string {
	if s == nil {
		return "<nil>"
	}
	if s.Regex != nil {
		return s.Regex.String()
	}
	return s.Repo
}

// MatchesRepo reports whether repoURL contains repo as complete repository path
// segments. repo can be either a full URL or a short path such as owner/repo.
//
// This intentionally uses bounded matching instead of strings.Contains so
// owner/repo matches https://example.com/owner/repo.git, but does not match
// https://example.com/owner/repo-deploy.git.
func MatchesRepo(repoURL, repo string) bool {
	normalizedURL := normalizeRepoMatchInput(repoURL)
	normalizedRepo := normalizeRepoMatchInput(repo)
	if normalizedURL == "" || normalizedRepo == "" {
		return false
	}

	pattern := `(^|[:/])` + regexp.QuoteMeta(normalizedRepo) + `($|/)`
	return regexp.MustCompile(pattern).MatchString(normalizedURL)
}

func normalizeRepoMatchInput(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = strings.Trim(input, "/")
	input = strings.TrimSuffix(input, ".git")
	return input
}
