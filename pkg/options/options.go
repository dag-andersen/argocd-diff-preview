package options

import (
	"flag"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/rs/zerolog/log"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
)

type Options struct {
	Debug                     bool   `env:"DEBUG"`
	Timeout                   uint64 `env:"TIMEOUT" envDefault:"180"`
	FileRegex                 string `env:"FILE_REGEX"`
	DiffIgnore                string `env:"DIFF_IGNORE"`
	LineCount                 uint   `env:"LINE_COUNT" envDefault:"10"`
	ArgocdChartVersion        string `env:"ARGOCD_CHART_VERSION"`
	BaseBranch                string `env:"BASE_BRANCH" envDefault:"main"`
	TargetBranch              string `env:"TARGET_BRANCH"`
	Repo                      string `env:"REPO"`
	OutputFolder              string `env:"OUTPUT_FOLDER" envDefault:"./output"`
	SecretsFolder             string `env:"SECRETS_FOLDER" envDefault:"./secrets"`
	ClusterType               string `env:"CLUSTER" envDefault:"auto"`
	ClusterName               string `env:"CLUSTER_NAME" envDefault:"argocd-diff-preview"`
	MaxDiffLength             uint   `env:"MAX_DIFF_LENGTH" envDefault:"65536"`
	Selector                  string `env:"SELECTOR"`
	FilesChanged              string `env:"FILES_CHANGED"`
	IgnoreInvalidWatchPattern bool   `env:"IGNORE_INVALID_WATCH_PATTERN"`
	KeepClusterAlive          bool   `env:"KEEP_CLUSTER_ALIVE"`
	ArgocdNamespace           string `env:"ARGOCD_NAMESPACE" envDefault:"argocd"`
	RedirectTargetRevisions   string `env:"REDIRECT_TARGET_REVISIONS"`
}

// Parse parses command line flags and environment variables
func Parse() *Options {
	// First parse environment variables
	opts := &Options{}
	if err := env.Parse(opts); err != nil {
		log.Warn().Err(err).Msg("Error parsing environment variables")
	}

	// Then parse command line flags (they will override env vars)

	// Basic flags
	flag.BoolVar(&opts.Debug, "debug", opts.Debug, "Activate debug mode")
	flag.BoolVar(&opts.Debug, "d", opts.Debug, "Activate debug mode (shorthand)")
	flag.Uint64Var(&opts.Timeout, "timeout", opts.Timeout, "Set timeout in seconds")

	// File and diff related
	flag.StringVar(&opts.FileRegex, "file-regex", opts.FileRegex, "Regex to filter files. Example: /apps_.*\\.yaml")
	flag.StringVar(&opts.FileRegex, "r", opts.FileRegex, "Regex to filter files (shorthand)")
	flag.StringVar(&opts.DiffIgnore, "diff-ignore", opts.DiffIgnore, "Ignore lines in diff. Example: v[1,9]+.[1,9]+.[1,9]+ for ignoring version changes")
	flag.StringVar(&opts.DiffIgnore, "i", opts.DiffIgnore, "Ignore lines in diff (shorthand)")
	flag.UintVar(&opts.LineCount, "line-count", opts.LineCount, "Generate diffs with <n> lines of context")
	flag.UintVar(&opts.LineCount, "c", opts.LineCount, "Generate diffs with <n> lines of context (shorthand)")

	// Argo CD related
	flag.StringVar(&opts.ArgocdChartVersion, "argocd-chart-version", opts.ArgocdChartVersion, "Argo CD Helm Chart version")
	flag.StringVar(&opts.ArgocdNamespace, "argocd-namespace", opts.ArgocdNamespace, "Namespace to use for Argo CD")

	// Git related
	flag.StringVar(&opts.BaseBranch, "base-branch", opts.BaseBranch, "Base branch name")
	flag.StringVar(&opts.BaseBranch, "b", opts.BaseBranch, "Base branch name (shorthand)")
	flag.StringVar(&opts.TargetBranch, "target-branch", opts.TargetBranch, "Target branch name")
	flag.StringVar(&opts.TargetBranch, "t", opts.TargetBranch, "Target branch name (shorthand)")
	flag.StringVar(&opts.Repo, "repo", opts.Repo, "Git Repository. Format: OWNER/REPO")

	// Folders
	flag.StringVar(&opts.OutputFolder, "output-folder", opts.OutputFolder, "Output folder where the diff will be saved")
	flag.StringVar(&opts.OutputFolder, "o", opts.OutputFolder, "Output folder (shorthand)")
	flag.StringVar(&opts.SecretsFolder, "secrets-folder", opts.SecretsFolder, "Secrets folder where the secrets are read from")
	flag.StringVar(&opts.SecretsFolder, "s", opts.SecretsFolder, "Secrets folder (shorthand)")

	// Cluster related
	flag.StringVar(&opts.ClusterType, "cluster", opts.ClusterType, "Local cluster tool. Options: kind, minikube, auto")
	flag.StringVar(&opts.ClusterName, "name", opts.ClusterName, "Cluster name (only for kind)")
	flag.BoolVar(&opts.KeepClusterAlive, "keep-cluster-alive", opts.KeepClusterAlive, "Keep cluster alive after the tool finishes")

	// Other options
	flag.UintVar(&opts.MaxDiffLength, "max-diff-length", opts.MaxDiffLength, "Max diff message character count")
	flag.StringVar(&opts.Selector, "selector", opts.Selector, "Label selector to filter on (e.g. key1=value1,key2=value2)")
	flag.StringVar(&opts.Selector, "l", opts.Selector, "Label selector (shorthand)")
	flag.StringVar(&opts.FilesChanged, "files-changed", opts.FilesChanged, "List of files changed between branches (comma or space separated)")
	flag.BoolVar(&opts.IgnoreInvalidWatchPattern, "ignore-invalid-watch-pattern", opts.IgnoreInvalidWatchPattern, "Ignore invalid watch pattern Regex on Applications")
	flag.StringVar(&opts.RedirectTargetRevisions, "redirect-target-revisions", opts.RedirectTargetRevisions, "List of target revisions to redirect")

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
