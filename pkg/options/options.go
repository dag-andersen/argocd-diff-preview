package options

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
)

type Options struct {
	Debug                     bool
	Timeout                   uint64
	FileRegex                 string
	DiffIgnore                string
	LineCount                 uint
	ArgocdChartVersion        string
	BaseBranch                string
	TargetBranch              string
	Repo                      string
	OutputFolder              string
	SecretsFolder             string
	ClusterType               string
	ClusterName               string
	MaxDiffLength             uint
	Selector                  string
	FilesChanged              string
	IgnoreInvalidWatchPattern bool
	KeepClusterAlive          bool
	ArgocdNamespace           string
	RedirectTargetRevisions   string
}

// Parse parses command line flags and environment variables
func Parse() *Options {
	opts := &Options{}

	// Basic flags
	flag.BoolVar(&opts.Debug, "debug", false, "Activate debug mode")
	flag.BoolVar(&opts.Debug, "d", false, "Activate debug mode (shorthand)")
	flag.Uint64Var(&opts.Timeout, "timeout", 180, "Set timeout in seconds")

	// File and diff related
	flag.StringVar(&opts.FileRegex, "file-regex", "", "Regex to filter files. Example: /apps_.*\\.yaml")
	flag.StringVar(&opts.FileRegex, "r", "", "Regex to filter files (shorthand)")
	flag.StringVar(&opts.DiffIgnore, "diff-ignore", "", "Ignore lines in diff. Example: v[1,9]+.[1,9]+.[1,9]+ for ignoring version changes")
	flag.StringVar(&opts.DiffIgnore, "i", "", "Ignore lines in diff (shorthand)")
	flag.UintVar(&opts.LineCount, "line-count", 10, "Generate diffs with <n> lines of context")
	flag.UintVar(&opts.LineCount, "c", 10, "Generate diffs with <n> lines of context (shorthand)")

	// Argo CD related
	flag.StringVar(&opts.ArgocdChartVersion, "argocd-chart-version", "", "Argo CD Helm Chart version")
	flag.StringVar(&opts.ArgocdNamespace, "argocd-namespace", "argocd", "Namespace to use for Argo CD")

	// Git related
	flag.StringVar(&opts.BaseBranch, "base-branch", "main", "Base branch name")
	flag.StringVar(&opts.BaseBranch, "b", "main", "Base branch name (shorthand)")
	flag.StringVar(&opts.TargetBranch, "target-branch", "", "Target branch name")
	flag.StringVar(&opts.TargetBranch, "t", "", "Target branch name (shorthand)")
	flag.StringVar(&opts.Repo, "repo", "", "Git Repository. Format: OWNER/REPO")

	// Folders
	flag.StringVar(&opts.OutputFolder, "output-folder", "./output", "Output folder where the diff will be saved")
	flag.StringVar(&opts.OutputFolder, "o", "./output", "Output folder (shorthand)")
	flag.StringVar(&opts.SecretsFolder, "secrets-folder", "./secrets", "Secrets folder where the secrets are read from")
	flag.StringVar(&opts.SecretsFolder, "s", "./secrets", "Secrets folder (shorthand)")

	// Cluster related
	flag.StringVar(&opts.ClusterType, "cluster", "auto", "Local cluster tool. Options: kind, minikube, auto")
	flag.StringVar(&opts.ClusterName, "name", "argocd-diff-preview", "Cluster name (only for kind)")
	flag.BoolVar(&opts.KeepClusterAlive, "keep-cluster-alive", false, "Keep cluster alive after the tool finishes")

	// Other options
	flag.UintVar(&opts.MaxDiffLength, "max-diff-length", 65536, "Max diff message character count")
	flag.StringVar(&opts.Selector, "selector", "", "Label selector to filter on (e.g. key1=value1,key2=value2)")
	flag.StringVar(&opts.Selector, "l", "", "Label selector (shorthand)")
	flag.StringVar(&opts.FilesChanged, "files-changed", "", "List of files changed between branches (comma or space separated)")
	flag.BoolVar(&opts.IgnoreInvalidWatchPattern, "ignore-invalid-watch-pattern", false, "Ignore invalid watch pattern Regex on Applications")
	flag.StringVar(&opts.RedirectTargetRevisions, "redirect-target-revisions", "", "List of target revisions to redirect")

	// Parse environment variables
	if envVal := os.Getenv("TIMEOUT"); envVal != "" {
		if val := parseUint64(envVal); val != 0 {
			opts.Timeout = val
		}
	}
	// Add similar environment variable parsing for other flags...

	flag.Parse()
	return opts
}

// ParseSelectors parses the selector string into a slice of Selectors
func (o *Options) ParseSelectors() []types.Selector {
	var selectors []types.Selector
	if o.Selector != "" {
		for _, s := range strings.Split(o.Selector, ",") {
			selector, err := types.FromString(strings.TrimSpace(s))
			if err != nil {
				log.Fatal().Err(err).Msg("Invalid selector format")
			}
			selectors = append(selectors, *selector)
		}
	}
	return selectors
}

// ParseFilesChanged parses the files-changed string into a slice of strings
func (o *Options) ParseFilesChanged() []string {
	if o.FilesChanged == "" {
		return nil
	}
	return strings.FieldsFunc(o.FilesChanged, func(r rune) bool {
		return r == ',' || r == ' '
	})
}

// ParseRegex returns a pointer to the regex string if set
func (o *Options) ParseRegex() *string {
	if o.FileRegex == "" {
		return nil
	}
	return &o.FileRegex
}

// ParseRedirectRevisions parses the redirect-target-revisions string into a slice of strings
func (o *Options) ParseRedirectRevisions() []string {
	if o.RedirectTargetRevisions == "" {
		return nil
	}
	return strings.Split(o.RedirectTargetRevisions, ",")
}

// LogOptions logs all the options
func (o *Options) LogOptions() {
	log.Info().Msg("✨ Running with:")
	log.Info().Msgf("✨ - local-cluster-tool: %s", o.ClusterType)
	log.Info().Msgf("✨ - cluster-name: %s", o.ClusterName)
	log.Info().Msgf("✨ - base-branch: %s", o.BaseBranch)
	log.Info().Msgf("✨ - target-branch: %s", o.TargetBranch)
	log.Info().Msgf("✨ - secrets-folder: %s", o.SecretsFolder)
	log.Info().Msgf("✨ - output-folder: %s", o.OutputFolder)
	log.Info().Msgf("✨ - argocd-namespace: %s", o.ArgocdNamespace)
	log.Info().Msgf("✨ - repo: %s", o.Repo)
	log.Info().Msgf("✨ - timeout: %d seconds", o.Timeout)

	if o.KeepClusterAlive {
		log.Info().Msg("✨ - keep-cluster-alive: true")
	}
	if o.Debug {
		log.Info().Msg("✨ - debug: true")
	}
	if o.FileRegex != "" {
		log.Info().Msgf("✨ - file-regex: %s", o.FileRegex)
	}
	if o.DiffIgnore != "" {
		log.Info().Msgf("✨ - diff-ignore: %s", o.DiffIgnore)
	}
	if o.LineCount > 0 {
		log.Info().Msgf("✨ - line-count: %d", o.LineCount)
	}
	if o.ArgocdChartVersion != "" {
		log.Info().Msgf("✨ - argocd-version: %s", o.ArgocdChartVersion)
	}
	if o.MaxDiffLength > 0 {
		log.Info().Msgf("✨ - max-diff-length: %d", o.MaxDiffLength)
	}
	if o.FilesChanged != "" {
		log.Info().Msgf("✨ - files-changed: %s", o.FilesChanged)
	}
	if o.IgnoreInvalidWatchPattern {
		log.Info().Msg("✨ Ignoring invalid watch patterns Regex on Applications")
	}
}

func parseUint64(s string) uint64 {
	var val uint64
	if _, err := fmt.Sscanf(s, "%d", &val); err == nil {
		return val
	}
	return 0
}
