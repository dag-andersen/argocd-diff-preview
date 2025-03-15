package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dag-andersen/argocd-diff-preview/pkg/cluster"
	"github.com/dag-andersen/argocd-diff-preview/pkg/kind"
	"github.com/dag-andersen/argocd-diff-preview/pkg/minikube"
	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
)

var (
	// Version is the current version of the tool
	Version = "unknown"
	// Commit is the git commit hash
	Commit = "unknown"
	// BuildDate is the date the binary was built
	BuildDate = "unknown"
)

type Options struct {
	Debug                     bool   `mapstructure:"debug"`
	Timeout                   uint64 `mapstructure:"timeout"`
	FileRegex                 string `mapstructure:"file-regex"`
	DiffIgnore                string `mapstructure:"diff-ignore"`
	LineCount                 uint   `mapstructure:"line-count"`
	ArgocdChartVersion        string `mapstructure:"argocd-chart-version"`
	BaseBranch                string `mapstructure:"base-branch"`
	TargetBranch              string `mapstructure:"target-branch"`
	Repo                      string `mapstructure:"repo"`
	OutputFolder              string `mapstructure:"output-folder"`
	SecretsFolder             string `mapstructure:"secrets-folder"`
	ClusterType               string `mapstructure:"cluster"`
	ClusterName               string `mapstructure:"cluster-name"`
	MaxDiffLength             uint   `mapstructure:"max-diff-length"`
	Selector                  string `mapstructure:"selector"`
	FilesChanged              string `mapstructure:"files-changed"`
	IgnoreInvalidWatchPattern bool   `mapstructure:"ignore-invalid-watch-pattern"`
	KeepClusterAlive          bool   `mapstructure:"keep-cluster-alive"`
	ArgocdNamespace           string `mapstructure:"argocd-namespace"`
	RedirectTargetRevisions   string `mapstructure:"redirect-target-revisions"`

	// We'll store the parsed data in these fields
	parsedFileRegex         *string
	parsedSelectors         []types.Selector
	parsedFilesChanged      []string
	parsedRedirectRevisions []string
	clusterProvider         cluster.Provider
}

// Parse parses command line flags and environment variables
func Parse() *Options {
	opts := &Options{}

	// Create root command with the main run functionality directly in it
	rootCmd := &cobra.Command{
		Use:   "argocd-diff-preview",
		Short: "A tool to preview changes in Argo CD applications between branches",
		Long: `argocd-diff-preview is a tool that helps you preview changes in Argo CD applications
between two branches. It creates a local Kubernetes cluster, installs Argo CD,
and generates a diff of the resources that would be applied.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip validation if we're just showing help
			if cmd.Flags().Changed("help") {
				return nil
			}

			// Bind all flags to viper
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}

			// Unmarshal viper config into options struct
			if err := viper.Unmarshal(opts); err != nil {
				return fmt.Errorf("failed to unmarshal config: %w", err)
			}

			// Check required options
			errors := opts.CheckRequired()
			if len(errors) > 0 {
				var errorMsg = ""
				for _, err := range errors {
					errorMsg += fmt.Sprintf("'%s', ", err)
				}
				return fmt.Errorf("error parsing command line flags: %s", errorMsg)
			}

			// Configure logging based on debug mode
			if opts.Debug {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
				log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC850})
			} else {
				zerolog.SetGlobalLevel(zerolog.InfoLevel)
				log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, PartsExclude: []string{"time", "level"}})
			}

			// Parse all dependent options
			var err error

			// Parse regex
			opts.parsedFileRegex = opts.ParseFileRegex()

			// Parse selectors
			opts.parsedSelectors, err = opts.ParseSelectors()
			if err != nil {
				return fmt.Errorf("failed to parse selectors: %w", err)
			}

			// Parse files changed
			opts.parsedFilesChanged = opts.ParseFilesChanged()

			// Parse redirect revisions
			opts.parsedRedirectRevisions = opts.ParseRedirectRevisions()

			// Parse cluster type
			opts.clusterProvider, err = opts.ParseClusterType()
			if err != nil {
				return fmt.Errorf("failed to parse cluster type: %w", err)
			}

			// Log options
			opts.LogOptions()

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// We don't need to do anything here - this is just to make sure help works correctly
			// The actual execution logic will be handled in main.go using the parsed options
			return nil
		},
		// Don't show usage on errors
		SilenceUsage: true,
	}

	// Create our own help command that exits after showing help
	defaultHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		defaultHelpFunc(cmd, args)
		os.Exit(0)
	})

	// Set up viper to read env variables
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Configure default values in viper
	viper.SetDefault("timeout", 180)
	viper.SetDefault("line-count", 10)
	viper.SetDefault("base-branch", "main")
	viper.SetDefault("output-folder", "./output")
	viper.SetDefault("secrets-folder", "./secrets")
	viper.SetDefault("cluster", "auto")
	viper.SetDefault("cluster-name", "argocd-diff-preview")
	viper.SetDefault("max-diff-length", 65536)
	viper.SetDefault("argocd-namespace", "argocd")

	// Basic flags
	rootCmd.Flags().BoolP("debug", "d", false, "Activate debug mode")
	rootCmd.Flags().String("timeout", "180", "Set timeout in seconds")

	// File and diff related
	rootCmd.Flags().StringP("file-regex", "r", "", "Regex to filter files. Example: /apps_.*\\.yaml")
	rootCmd.Flags().StringP("diff-ignore", "i", "", "Ignore lines in diff. Example: v[1,9]+.[1,9]+.[1,9]+ for ignoring version changes")
	rootCmd.Flags().StringP("line-count", "c", "10", "Generate diffs with <n> lines of context")

	// Argo CD related
	rootCmd.Flags().String("argocd-chart-version", "", "Argo CD Helm Chart version")
	rootCmd.Flags().String("argocd-namespace", "argocd", "Namespace to use for Argo CD")

	// Git related
	rootCmd.Flags().StringP("base-branch", "b", "main", "Base branch name")
	rootCmd.Flags().StringP("target-branch", "t", "", "Target branch name")
	rootCmd.Flags().String("repo", "", "Git Repository. Format: OWNER/REPO")

	// Folders
	rootCmd.Flags().StringP("output-folder", "o", "./output", "Output folder where the diff will be saved")
	rootCmd.Flags().StringP("secrets-folder", "s", "./secrets", "Secrets folder where the secrets are read from")

	// Cluster related
	rootCmd.Flags().String("cluster", "auto", "Local cluster tool. Options: kind, minikube, auto")
	rootCmd.Flags().String("cluster-name", "argocd-diff-preview", "Cluster name (only for kind)")
	rootCmd.Flags().Bool("keep-cluster-alive", false, "Keep cluster alive after the tool finishes")

	// Other options
	rootCmd.Flags().String("max-diff-length", "65536", "Max diff message character count")
	rootCmd.Flags().StringP("selector", "l", "", "Label selector to filter on (e.g. key1=value1,key2=value2)")
	rootCmd.Flags().String("files-changed", "", "List of files changed between branches (comma or space separated)")
	rootCmd.Flags().Bool("ignore-invalid-watch-pattern", false, "Ignore invalid watch pattern Regex on Applications")
	rootCmd.Flags().String("redirect-target-revisions", "", "List of target revisions to redirect")

	// Check if version flag was specified directly
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			fmt.Println(rootCmd.Version)
			os.Exit(0)
		}
	}

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Failed to execute command")
	}

	return opts
}

func (o *Options) CheckRequired() []string {
	var errors []string
	if o.BaseBranch == "" {
		errors = append(errors, "base-branch")
	}
	if o.TargetBranch == "" {
		errors = append(errors, "target-branch")
	}
	if o.Repo == "" {
		errors = append(errors, "repo")
	}
	return errors
}

// ParseSelectors parses the selector string into a slice of Selectors
func (o *Options) ParseSelectors() ([]types.Selector, error) {
	var selectors []types.Selector
	if o.Selector != "" {
		for _, s := range strings.Split(o.Selector, ",") {
			selector, err := types.FromString(strings.TrimSpace(s))
			if err != nil {
				return nil, err
			}
			selectors = append(selectors, *selector)
		}
	}
	return selectors, nil
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

// ParseFileRegex returns a pointer to the regex string if set, and validates that it's a valid regex pattern
func (o *Options) ParseFileRegex() *string {
	if o.FileRegex == "" {
		return nil
	}

	// Try to compile the regex to validate it
	if _, err := regexp.Compile(o.FileRegex); err != nil {
		log.Fatal().Err(err).Msgf("Invalid regex pattern: %s", o.FileRegex)
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

// ParseClusterType parses the cluster type and returns the appropriate cluster provider
func (o *Options) ParseClusterType() (cluster.Provider, error) {
	var provider cluster.Provider
	switch o.ClusterType {
	case "kind":
		provider = kind.New(o.ClusterName)
	case "minikube":
		provider = minikube.New()
	case "auto":
		if kind.IsInstalled() {
			provider = kind.New(o.ClusterName)
			log.Debug().Msg("Using kind as cluster provider")
		} else if minikube.IsInstalled() {
			provider = minikube.New()
			log.Debug().Msg("Using minikube as cluster provider")
		} else {
			return nil, fmt.Errorf("no local cluster tool found. Please install kind or minikube")
		}
	default:
		return nil, fmt.Errorf("unsupported cluster type: %s", o.ClusterType)
	}

	if !provider.IsInstalled() {
		return nil, fmt.Errorf("%s is not installed", o.ClusterType)
	}

	return provider, nil
}

// LogOptions logs all the options
func (o *Options) LogOptions() {
	if Version != "unknown" && BuildDate != "unknown" {
		log.Info().Msgf("✨ Running %s (%s) with:", Version, BuildDate)
	} else {
		log.Info().Msg("✨ Running with:")
	}
	log.Info().Msgf("✨ - local-cluster-tool: %s", o.clusterProvider.GetName())
	log.Info().Msgf("✨ - cluster-name: %s", o.ClusterName)
	log.Info().Msgf("✨ - base-branch: %s", o.BaseBranch)
	log.Info().Msgf("✨ - target-branch: %s", o.TargetBranch)
	log.Info().Msgf("✨ - secrets-folder: %s", o.SecretsFolder)
	log.Info().Msgf("✨ - output-folder: %s", o.OutputFolder)
	log.Info().Msgf("✨ - argocd-namespace: %s", o.ArgocdNamespace)
	log.Info().Msgf("✨ - repo: %s", o.Repo)
	log.Info().Msgf("✨ - timeout: %d seconds", o.Timeout)

	if o.KeepClusterAlive {
		log.Info().Msgf("✨ - keep-cluster-alive: %t", o.KeepClusterAlive)
	}
	if o.Debug {
		log.Info().Msgf("✨ - debug: %t", o.Debug)
	}
	if o.parsedFileRegex != nil {
		log.Info().Msgf("✨ - file-regex: %s", *o.parsedFileRegex)
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
	if len(o.parsedFilesChanged) > 0 {
		log.Info().Msgf("✨ - files-changed: %s", o.FilesChanged)
	}
	if len(o.parsedSelectors) > 0 {
		log.Info().Msgf("✨ - selectors: %s", o.parsedSelectors)
	}
	if o.parsedRedirectRevisions != nil {
		log.Info().Msgf("✨ - redirect-target-revisions: %s", o.parsedRedirectRevisions)
	}
	if o.IgnoreInvalidWatchPattern {
		log.Info().Msg("✨ Ignoring invalid watch patterns Regex on Applications")
	}
}

// GetFileRegex returns the parsed regex
func (o *Options) GetFileRegex() *string {
	return o.parsedFileRegex
}

// GetSelectors returns the parsed selectors
func (o *Options) GetSelectors() []types.Selector {
	return o.parsedSelectors
}

// GetFilesChanged returns the parsed files changed
func (o *Options) GetFilesChanged() []string {
	return o.parsedFilesChanged
}

// GetRedirectRevisions returns the parsed redirect revisions
func (o *Options) GetRedirectRevisions() []string {
	return o.parsedRedirectRevisions
}

// GetClusterProvider returns the cluster provider
func (o *Options) GetClusterProvider() cluster.Provider {
	return o.clusterProvider
}
