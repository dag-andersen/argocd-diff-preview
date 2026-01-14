package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/dag-andersen/argocd-diff-preview/pkg/app_selector"
	"github.com/dag-andersen/argocd-diff-preview/pkg/cluster"
	"github.com/dag-andersen/argocd-diff-preview/pkg/k3d"
	"github.com/dag-andersen/argocd-diff-preview/pkg/kind"
	"github.com/dag-andersen/argocd-diff-preview/pkg/minikube"
	"github.com/dag-andersen/argocd-diff-preview/pkg/resource_filter"
)

var (
	// Version is the current version of the tool
	Version = "unknown"
	// Commit is the git commit hash
	Commit = "unknown"
	// BuildDate is the date the binary was built
	BuildDate = "unknown"
)

// defaults
var (
	DefaultTimeout                    = uint64(180)
	DefaultLineCount                  = uint(7)
	DefaultBaseBranch                 = "main"
	DefaultOutputFolder               = "./output"
	DefaultSecretsFolder              = "./secrets"
	DefaultCluster                    = "auto"
	DefaultClusterName                = "argocd-diff-preview"
	DefaultKindOptions                = ""
	DefaultKindInternal               = false
	DefaultK3dOptions                 = ""
	DefaultMaxDiffLength              = uint(65536)
	DefaultArgocdNamespace            = "argocd"
	DefaultArgocdChartVersion         = "latest"
	DefaultArgocdChartName            = "argo"
	DefaultArgocdChartURL             = "https://argoproj.github.io/argo-helm"
	DefaultArgocdChartRepoUsername    = ""
	DefaultArgocdChartRepoPassword    = ""
	DefaultLogFormat                  = "human"
	DefaultTitle                      = "Argo CD Diff Preview"
	DefaultCreateCluster              = true
	DefaultUseArgoCDApi               = false
	DefaultKeepClusterAlive           = false
	DefaultDryRun                     = false
	DefaultAutoDetectFilesChanged     = false
	DefaultWatchIfNoWatchPatternFound = false
	DefaultIgnoreInvalidWatchPattern  = false
	DefaultHideDeletedAppDiff         = false
	DefaultIgnoreResourceRules        = ""
	DefaultArgocdLoginOptions         = ""
)

// RawOptions holds the raw CLI/env inputs - used only for parsing
type RawOptions struct {
	Debug                      bool   `mapstructure:"debug"`
	DryRun                     bool   `mapstructure:"dry-run"`
	Timeout                    uint64 `mapstructure:"timeout"`
	FileRegex                  string `mapstructure:"file-regex"`
	DiffIgnore                 string `mapstructure:"diff-ignore"`
	LineCount                  uint   `mapstructure:"line-count"`
	BaseBranch                 string `mapstructure:"base-branch"`
	TargetBranch               string `mapstructure:"target-branch"`
	Repo                       string `mapstructure:"repo"`
	OutputFolder               string `mapstructure:"output-folder"`
	SecretsFolder              string `mapstructure:"secrets-folder"`
	CreateCluster              bool   `mapstructure:"create-cluster"`
	ClusterType                string `mapstructure:"cluster"`
	ClusterName                string `mapstructure:"cluster-name"`
	KindOptions                string `mapstructure:"kind-options"`
	KindInternal               bool   `mapstructure:"kind-internal"`
	K3dOptions                 string `mapstructure:"k3d-options"`
	MaxDiffLength              uint   `mapstructure:"max-diff-length"`
	Selector                   string `mapstructure:"selector"`
	FilesChanged               string `mapstructure:"files-changed"`
	IgnoreInvalidWatchPattern  bool   `mapstructure:"ignore-invalid-watch-pattern"`
	WatchIfNoWatchPatternFound bool   `mapstructure:"watch-if-no-watch-pattern-found"`
	AutoDetectFilesChanged     bool   `mapstructure:"auto-detect-files-changed"`
	KeepClusterAlive           bool   `mapstructure:"keep-cluster-alive"`
	ArgocdNamespace            string `mapstructure:"argocd-namespace"`
	ArgocdChartVersion         string `mapstructure:"argocd-chart-version"`
	ArgocdChartName            string `mapstructure:"argocd-chart-name"`
	ArgocdChartURL             string `mapstructure:"argocd-chart-url"`
	ArgocdChartRepoUsername    string `mapstructure:"argocd-chart-repo-username"`
	ArgocdChartRepoPassword    string `mapstructure:"argocd-chart-repo-password"`
	ArgocdLoginOptions         string `mapstructure:"argocd-login-options"`
	UseArgoCDApi               bool   `mapstructure:"use-argocd-api"`
	RedirectTargetRevisions    string `mapstructure:"redirect-target-revisions"`
	LogFormat                  string `mapstructure:"log-format"`
	Title                      string `mapstructure:"title"`
	HideDeletedAppDiff         bool   `mapstructure:"hide-deleted-app-diff"`
	IgnoreResourceRules        string `mapstructure:"ignore-resources"`
}

// Config is the final, validated, ready-to-use configuration
type Config struct {
	// Direct passthrough fields
	Debug                      bool
	DryRun                     bool
	Timeout                    uint64
	DiffIgnore                 string
	LineCount                  uint
	BaseBranch                 string
	TargetBranch               string
	Repo                       string
	OutputFolder               string
	SecretsFolder              string
	CreateCluster              bool
	ClusterName                string
	KindOptions                string
	KindInternal               bool
	K3dOptions                 string
	MaxDiffLength              uint
	IgnoreInvalidWatchPattern  bool
	WatchIfNoWatchPatternFound bool
	AutoDetectFilesChanged     bool
	KeepClusterAlive           bool
	ArgocdNamespace            string
	ArgocdChartVersion         string
	ArgocdChartName            string
	ArgocdChartURL             string
	ArgocdChartRepoUsername    string
	ArgocdChartRepoPassword    string
	ArgocdLoginOptions         string
	LogFormat                  string
	Title                      string
	HideDeletedAppDiff         bool
	UseArgoCDApi               bool

	// Parsed/processed fields - no "parsed" prefix needed
	FileRegex           *regexp.Regexp
	Selectors           []app_selector.Selector
	FilesChanged        []string
	RedirectRevisions   []string
	IgnoreResourceRules []resource_filter.IgnoreResourceRule
	ClusterProvider     cluster.Provider
}

// Parse parses command line flags and environment variables, returning a validated Config
func Parse() *Config {
	raw := &RawOptions{}

	// Create root command with the main run functionality directly in it
	rootCmd := &cobra.Command{
		Use:     "argocd-diff-preview",
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

			// Unmarshal viper config into raw options struct
			if err := viper.Unmarshal(raw); err != nil {
				return fmt.Errorf("failed to unmarshal config: %w", err)
			}

			// Check required options
			errors := raw.checkRequired()
			if len(errors) > 0 {
				var errorMsg strings.Builder
				for _, err := range errors {
					fmt.Fprintf(&errorMsg, "'%s', ", err)
				}
				return fmt.Errorf("error parsing command line flags: %s", errorMsg.String())
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
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
	viper.SetDefault("timeout", DefaultTimeout)
	viper.SetDefault("line-count", DefaultLineCount)
	viper.SetDefault("base-branch", DefaultBaseBranch)
	viper.SetDefault("output-folder", DefaultOutputFolder)
	viper.SetDefault("secrets-folder", DefaultSecretsFolder)
	viper.SetDefault("create-cluster", DefaultCreateCluster)
	viper.SetDefault("watch-if-no-watch-pattern-found", DefaultWatchIfNoWatchPatternFound)
	viper.SetDefault("ignore-invalid-watch-pattern", DefaultIgnoreInvalidWatchPattern)
	viper.SetDefault("keep-cluster-alive", DefaultKeepClusterAlive)
	viper.SetDefault("cluster", DefaultCluster)
	viper.SetDefault("cluster-name", DefaultClusterName)
	viper.SetDefault("max-diff-length", DefaultMaxDiffLength)
	viper.SetDefault("argocd-namespace", DefaultArgocdNamespace)
	viper.SetDefault("argocd-chart-version", DefaultArgocdChartVersion)
	viper.SetDefault("argocd-chart-name", DefaultArgocdChartName)
	viper.SetDefault("argocd-chart-url", DefaultArgocdChartURL)
	viper.SetDefault("argocd-chart-repo-username", DefaultArgocdChartRepoUsername)
	viper.SetDefault("argocd-chart-repo-password", DefaultArgocdChartRepoPassword)
	viper.SetDefault("argocd-login-options", DefaultArgocdLoginOptions)
	viper.SetDefault("use-argocd-api", DefaultUseArgoCDApi)
	viper.SetDefault("log-format", DefaultLogFormat)
	viper.SetDefault("title", DefaultTitle)
	viper.SetDefault("dry-run", DefaultDryRun)
	viper.SetDefault("hide-deleted-app-diff", DefaultHideDeletedAppDiff)
	viper.SetDefault("ignore-resources", DefaultIgnoreResourceRules)

	// Basic flags
	rootCmd.Flags().BoolP("debug", "d", false, "Activate debug mode")
	rootCmd.Flags().Bool("dry-run", DefaultDryRun, "Show which applications would be processed without creating a cluster or generating a diff")
	rootCmd.Flags().String("log-format", DefaultLogFormat, "Log format (human or json)")
	rootCmd.Flags().String("timeout", fmt.Sprintf("%d", DefaultTimeout), "Set timeout in seconds")

	// File and diff related
	rootCmd.Flags().StringP("file-regex", "r", "", "Regex to select/filter files. Example: /apps_.*\\.yaml")
	rootCmd.Flags().StringP("diff-ignore", "i", "", "Ignore lines in diff. Example: v[1,9]+.[1,9]+.[1,9]+ for ignoring version changes")
	rootCmd.Flags().StringP("line-count", "c", fmt.Sprintf("%d", DefaultLineCount), "Generate diffs with <n> lines of context")
	rootCmd.Flags().String("ignore-resources", DefaultIgnoreResourceRules, "Ignore resources in diff. Example: 'group:kind:name',group:kind:name")

	// Argo CD related
	rootCmd.Flags().String("argocd-chart-version", "", "Argo CD Helm Chart version")
	rootCmd.Flags().String("argocd-namespace", DefaultArgocdNamespace, "Namespace to use for Argo CD")
	rootCmd.Flags().String("argocd-chart-name", DefaultArgocdChartName, "Argo CD Helm Chart name")
	rootCmd.Flags().String("argocd-chart-url", DefaultArgocdChartURL, "Argo CD Helm Chart URL")
	rootCmd.Flags().String("argocd-chart-repo-username", DefaultArgocdChartRepoUsername, "Argo CD Helm Repo User Name")
	rootCmd.Flags().String("argocd-chart-repo-password", DefaultArgocdChartRepoPassword, "Argo CD Helm Repo Password")
	rootCmd.Flags().String("argocd-login-options", DefaultArgocdLoginOptions, "Additional options to pass to 'argocd login' command")
	// Git related
	rootCmd.Flags().StringP("base-branch", "b", DefaultBaseBranch, "Base branch name")
	rootCmd.Flags().StringP("target-branch", "t", "", "Target branch name (required)")
	rootCmd.Flags().String("repo", "", "Git Repository. Format: OWNER/REPO (required)")

	// Folders
	rootCmd.Flags().StringP("output-folder", "o", DefaultOutputFolder, "Output folder where the diff will be saved")
	rootCmd.Flags().StringP("secrets-folder", "s", DefaultSecretsFolder, "Secrets folder where the secrets are read from")

	// Cluster related
	rootCmd.Flags().Bool("create-cluster", DefaultCreateCluster, "Create a new cluster if it doesn't exist")
	rootCmd.Flags().Bool("use-argocd-api", DefaultUseArgoCDApi, "Use Argo CD API instead of CLI")
	rootCmd.Flags().String("cluster", DefaultCluster, "Local cluster tool. Options: kind, minikube, k3d, auto")
	rootCmd.Flags().String("cluster-name", DefaultClusterName, "Cluster name (only for kind & k3d)")
	rootCmd.Flags().String("kind-options", DefaultKindOptions, "kind options (only for kind)")
	rootCmd.Flags().Bool("kind-internal", DefaultKindInternal, "kind internal kubeconfig mode (only for kind)")
	rootCmd.Flags().String("k3d-options", DefaultK3dOptions, "k3d options (only for k3d)")
	rootCmd.Flags().Bool("keep-cluster-alive", DefaultKeepClusterAlive, "Keep cluster alive after the tool finishes")

	// Other options
	rootCmd.Flags().String("max-diff-length", fmt.Sprintf("%d", DefaultMaxDiffLength), "Max diff message character count")
	rootCmd.Flags().StringP("selector", "l", "", "Label selector to filter on (e.g. key1=value1,key2=value2)")
	rootCmd.Flags().String("files-changed", "", "List of files changed between branches (comma, space or newline separated)")
	rootCmd.Flags().Bool("auto-detect-files-changed", DefaultAutoDetectFilesChanged, "Auto detect files changed between branches")
	rootCmd.Flags().Bool("ignore-invalid-watch-pattern", DefaultIgnoreInvalidWatchPattern, "Ignore invalid watch pattern Regex on Applications")
	rootCmd.Flags().Bool("watch-if-no-watch-pattern-found", DefaultWatchIfNoWatchPatternFound, "Render applications without watch pattern")
	rootCmd.Flags().String("redirect-target-revisions", "", "List of target revisions to redirect")
	rootCmd.Flags().String("title", DefaultTitle, "Custom title for the markdown output")
	rootCmd.Flags().Bool("hide-deleted-app-diff", DefaultHideDeletedAppDiff, "Hide diff content for fully deleted applications (only show deletion header)")

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

	// Convert raw options to final config
	cfg, err := raw.ToConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse configuration")
	}

	// Configure logging based on debug mode and log format
	configureLogging(cfg)

	// Log the final configuration
	cfg.LogConfig()

	return cfg
}

// checkRequired validates that required fields are present
func (o *RawOptions) checkRequired() []string {
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

// ToConfig converts RawOptions to a validated Config
func (o *RawOptions) ToConfig() (*Config, error) {
	cfg := &Config{
		// Direct passthrough fields
		Debug:                      o.Debug,
		DryRun:                     o.DryRun,
		Timeout:                    o.Timeout,
		DiffIgnore:                 o.DiffIgnore,
		LineCount:                  o.LineCount,
		BaseBranch:                 o.BaseBranch,
		TargetBranch:               o.TargetBranch,
		Repo:                       o.Repo,
		OutputFolder:               o.OutputFolder,
		SecretsFolder:              o.SecretsFolder,
		CreateCluster:              o.CreateCluster,
		ClusterName:                o.ClusterName,
		KindOptions:                o.KindOptions,
		KindInternal:               o.KindInternal,
		K3dOptions:                 o.K3dOptions,
		MaxDiffLength:              o.MaxDiffLength,
		IgnoreInvalidWatchPattern:  o.IgnoreInvalidWatchPattern,
		WatchIfNoWatchPatternFound: o.WatchIfNoWatchPatternFound,
		AutoDetectFilesChanged:     o.AutoDetectFilesChanged,
		KeepClusterAlive:           o.KeepClusterAlive,
		ArgocdNamespace:            o.ArgocdNamespace,
		ArgocdChartVersion:         o.ArgocdChartVersion,
		ArgocdChartName:            o.ArgocdChartName,
		ArgocdChartURL:             o.ArgocdChartURL,
		ArgocdChartRepoUsername:    o.ArgocdChartRepoUsername,
		ArgocdChartRepoPassword:    o.ArgocdChartRepoPassword,
		ArgocdLoginOptions:         o.ArgocdLoginOptions,
		LogFormat:                  o.LogFormat,
		Title:                      o.Title,
		HideDeletedAppDiff:         o.HideDeletedAppDiff,
		UseArgoCDApi:               o.UseArgoCDApi,
	}

	var err error

	// Apply defaults for zero values
	if cfg.LineCount <= 0 {
		cfg.LineCount = DefaultLineCount
	}
	if cfg.MaxDiffLength <= 0 {
		cfg.MaxDiffLength = DefaultMaxDiffLength
	}

	// Parse file regex
	cfg.FileRegex, err = o.parseFileRegex()
	if err != nil {
		return nil, fmt.Errorf("invalid file-regex: %w", err)
	}

	// Parse selectors
	cfg.Selectors, err = o.parseSelectors()
	if err != nil {
		return nil, fmt.Errorf("invalid selectors: %w", err)
	}

	// Parse files changed
	cfg.FilesChanged = o.parseFilesChanged()

	// Parse skip resource rules
	cfg.IgnoreResourceRules, err = resource_filter.FromString(o.IgnoreResourceRules)
	if err != nil {
		return nil, fmt.Errorf("invalid ignore-resources: %w", err)
	}

	// Parse redirect revisions
	cfg.RedirectRevisions = o.parseRedirectRevisions()

	// Parse cluster type if we are creating a new cluster
	if cfg.CreateCluster {
		cfg.ClusterProvider, err = o.parseClusterType()
		if err != nil {
			return nil, fmt.Errorf("invalid cluster configuration: %w", err)
		}
	}

	return cfg, nil
}

// parseSelectors parses the selector string into a slice of Selectors
func (o *RawOptions) parseSelectors() ([]app_selector.Selector, error) {
	var selectors []app_selector.Selector
	if o.Selector != "" {
		for s := range strings.SplitSeq(o.Selector, ",") {
			selector, err := app_selector.FromString(strings.TrimSpace(s))
			if err != nil {
				return nil, err
			}
			selectors = append(selectors, *selector)
		}
	}
	return selectors, nil
}

// parseFilesChanged parses the files-changed string into a slice of strings
func (o *RawOptions) parseFilesChanged() []string {
	if o.FilesChanged == "" {
		return nil
	}
	return strings.FieldsFunc(o.FilesChanged, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n'
	})
}

// parseFileRegex returns a compiled regex if set
func (o *RawOptions) parseFileRegex() (*regexp.Regexp, error) {
	if o.FileRegex == "" {
		return nil, nil
	}
	return regexp.Compile(o.FileRegex)
}

// parseRedirectRevisions parses the redirect-target-revisions string into a slice of strings
func (o *RawOptions) parseRedirectRevisions() []string {
	if o.RedirectTargetRevisions == "" {
		return nil
	}
	return strings.Split(o.RedirectTargetRevisions, ",")
}

// parseClusterType parses the cluster type and returns the appropriate cluster provider
func (o *RawOptions) parseClusterType() (cluster.Provider, error) {
	var provider cluster.Provider
	clusterType := strings.ToLower(o.ClusterType)

	switch clusterType {
	case "kind":
		provider = kind.New(o.ClusterName, o.KindOptions, o.KindInternal)
	case "k3d":
		provider = k3d.New(o.ClusterName, o.K3dOptions)
	case "minikube":
		provider = minikube.New()
	case "auto":
		if kind.IsInstalled() {
			provider = kind.New(o.ClusterName, o.KindOptions, o.KindInternal)
			log.Debug().Msg("Using kind as cluster provider (auto-detected)")
		} else if k3d.IsInstalled() {
			provider = k3d.New(o.ClusterName, o.K3dOptions)
			log.Debug().Msg("Using k3d as cluster provider (auto-detected)")
		} else if minikube.IsInstalled() {
			provider = minikube.New()
			log.Debug().Msg("Using minikube as cluster provider (auto-detected)")
		} else {
			return nil, fmt.Errorf("no local cluster tool found. Please install kind, k3d or minikube")
		}
	default:
		return nil, fmt.Errorf("unsupported cluster type: %s", o.ClusterType)
	}

	if !provider.IsInstalled() {
		return nil, fmt.Errorf("%s is not installed", o.ClusterType)
	}

	return provider, nil
}

// configureLogging sets up the logger based on the config
func configureLogging(cfg *Config) {
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, NoColor: true}
	if cfg.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		if cfg.LogFormat == "human" {
			consoleWriter.TimeFormat = time.RFC1123
		}
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if cfg.LogFormat == "human" {
			consoleWriter.PartsExclude = []string{"time", "level"}
		}
	}
	if cfg.LogFormat == "human" {
		consoleWriter.FormatFieldName = func(i any) string { return fmt.Sprintf("(%s: ", i) }
		consoleWriter.FormatFieldValue = func(i any) string { return fmt.Sprintf("%s)", i) }
	}
	log.Logger = log.Output(consoleWriter)
}

// LogConfig logs all the configuration values
func (o *Config) LogConfig() {
	if Version != "unknown" && BuildDate != "unknown" {
		log.Info().Msgf("✨ Running %s (%s) with:", Version, BuildDate)
	} else {
		log.Info().Msg("✨ Running with:")
	}

	if o.DryRun {
		log.Info().Msgf("✨ - dry-run: %t", o.DryRun)
	} else {
		if !o.CreateCluster {
			log.Info().Msgf("✨ - using cluster with Argo CD pre-installed")
		} else {
			log.Info().Msgf("✨ - local-cluster-tool: %s", o.ClusterProvider.GetName())
			log.Info().Msgf("✨ - cluster-name: %s", o.ClusterName)
			if o.ClusterProvider.GetName() == "kind" {
				if o.KindOptions != "" {
					log.Info().Msgf("✨ - kind-options: %s", o.KindOptions)
				}
				if o.KindInternal {
					log.Info().Msgf("✨ - kind-internal: %t", o.KindInternal)
				}
			}
			if o.ClusterProvider.GetName() == "k3d" && o.K3dOptions != "" {
				log.Info().Msgf("✨ - k3d-options: %s", o.K3dOptions)
			}
		}
		if o.UseArgoCDApi {
			log.Info().Msgf("✨ - use-argocd-api: %t", o.UseArgoCDApi)
		}
	}

	log.Info().Msgf("✨ - base-branch: %s", o.BaseBranch)
	log.Info().Msgf("✨ - target-branch: %s", o.TargetBranch)
	log.Info().Msgf("✨ - secrets-folder: %s", o.SecretsFolder)
	log.Info().Msgf("✨ - output-folder: %s", o.OutputFolder)
	log.Info().Msgf("✨ - argocd-namespace: %s", o.ArgocdNamespace)
	log.Info().Msgf("✨ - repo: %s", o.Repo)
	log.Info().Msgf("✨ - timeout: %d seconds", o.Timeout)
	if o.LogFormat != DefaultLogFormat {
		log.Info().Msgf("✨ - log-format: %s", o.LogFormat)
	}
	if o.KeepClusterAlive {
		log.Info().Msgf("✨ - keep-cluster-alive: %t", o.KeepClusterAlive)
	}
	if o.Debug {
		log.Info().Msgf("✨ - debug: %t - This is slower because it will do more checks", o.Debug)
	}
	if o.FileRegex != nil {
		log.Info().Msgf("✨ - file-regex: %s", o.FileRegex.String())
	}
	if o.DiffIgnore != "" {
		log.Info().Msgf("✨ - diff-ignore: %s", o.DiffIgnore)
	}
	if o.LineCount != DefaultLineCount {
		log.Info().Msgf("✨ - line-count: %d", o.LineCount)
	}
	if o.MaxDiffLength != DefaultMaxDiffLength {
		log.Info().Msgf("✨ - max-diff-length: %d", o.MaxDiffLength)
	}
	if len(o.FilesChanged) > 0 {
		log.Info().Msgf("✨ - files-changed: %v", o.FilesChanged)
	} else if o.AutoDetectFilesChanged {
		log.Info().Msgf("✨ - files-changed: auto-detected")
	}
	if len(o.FilesChanged) > 0 || o.AutoDetectFilesChanged {
		if DefaultIgnoreInvalidWatchPattern != o.IgnoreInvalidWatchPattern {
			log.Info().Msg("✨ --- Ignoring applications with invalid watch-pattern annotation")
		}
		if DefaultWatchIfNoWatchPatternFound != o.WatchIfNoWatchPatternFound {
			log.Info().Msgf("✨ --- Rendering applications with no watch-pattern annotation")
		}
	}
	if len(o.Selectors) > 0 {
		selectorStrings := make([]string, len(o.Selectors))
		for i, selector := range o.Selectors {
			selectorStrings[i] = selector.String()
		}
		log.Info().Msgf("✨ - selectors: %s", strings.Join(selectorStrings, ", "))
	}
	if len(o.RedirectRevisions) > 0 {
		log.Info().Msgf("✨ - redirect-target-revisions: %s", o.RedirectRevisions)
	}
	if o.ArgocdChartVersion != DefaultArgocdChartVersion && o.ArgocdChartVersion != "" {
		log.Info().Msgf("✨ - argocd-chart-version: %s", o.ArgocdChartVersion)
	}
	if o.ArgocdChartName != DefaultArgocdChartName {
		log.Info().Msgf("✨ - argocd-chart-name: %s", o.ArgocdChartName)
	}
	if o.ArgocdChartURL != DefaultArgocdChartURL {
		log.Info().Msgf("✨ - argocd-chart-url: %s", o.ArgocdChartURL)
	}
	if o.ArgocdChartRepoUsername != DefaultArgocdChartRepoUsername {
		log.Info().Msgf("✨ - argocd-chart-repo-username: %s", o.ArgocdChartRepoUsername)
	}
	if o.ArgocdChartRepoPassword != DefaultArgocdChartRepoPassword {
		log.Info().Msgf("✨ - argocd-chart-repo-password: *********")
	}
	if o.ArgocdLoginOptions != DefaultArgocdLoginOptions {
		log.Info().Msgf("✨ - argocd-login-options: %s", o.ArgocdLoginOptions)
	}
	if o.Title != DefaultTitle {
		log.Info().Msgf("✨ - title: %s", o.Title)
	}
	if o.HideDeletedAppDiff {
		log.Info().Msgf("✨ - hide-deleted-app-diff: %t", o.HideDeletedAppDiff)
	}
	if len(o.IgnoreResourceRules) > 0 {
		ignoreResourceRuleStrings := make([]string, len(o.IgnoreResourceRules))
		for i, ignoreResourceRule := range o.IgnoreResourceRules {
			ignoreResourceRuleStrings[i] = ignoreResourceRule.String()
		}
		log.Info().Msgf("✨ - ignore-resources: %s", strings.Join(ignoreResourceRuleStrings, ", "))
	}
}
