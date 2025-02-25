package main

import (
	"os"
	"time"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/argocd"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/cluster"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/diff"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/extract"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/kind"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/minikube"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/options"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/parsing"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	dirMode  = os.ModePerm // 0755 - read/write/execute for owner, read/execute for group and others
	fileMode = 0644        // 0644 - read/write for owner, read-only for group and others
)

func getClusterProvider(clusterType, clusterName string) cluster.Provider {
	var provider cluster.Provider
	switch clusterType {
	case "kind":
		provider = kind.New(clusterName)
	case "minikube":
		provider = minikube.New()
	case "auto":
		if kind.IsInstalled() {
			provider = kind.New(clusterName)
			clusterType = "kind"
		} else if minikube.IsInstalled() {
			provider = minikube.New()
			clusterType = "minikube"
		} else {
			log.Error().Msg("No local cluster tool found. Please install kind or minikube")
		}
	default:
		log.Error().Msgf("Unsupported cluster type: %s", clusterType)
	}

	if !provider.IsInstalled() {
		log.Error().Msgf("%s is not installed", clusterType)
	}

	return provider
}

func main() {
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		log.Info().Msgf("✨ Total execution time: %s", duration.Round(time.Millisecond))
	}()

	// Parse input options
	opts := options.Parse()

	// Configure logging based on debug mode
	if opts.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC850})
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, PartsExclude: []string{"time", "level"}})
	}

	opts.LogOptions()

	regex := opts.ParseRegex()
	selectors := opts.ParseSelectors()
	filesChanged := opts.ParseFilesChanged()
	redirectRevisions := opts.ParseRedirectRevisions()

	// Create branches
	baseBranch := types.NewBranch(opts.BaseBranch, types.Base)
	targetBranch := types.NewBranch(opts.TargetBranch, types.Target)

	// Get applications for both branches
	baseApps, targetApps, err := parsing.GetApplicationsForBranches(
		opts.ArgocdNamespace,
		baseBranch,
		targetBranch,
		regex,
		selectors,
		filesChanged,
		opts.Repo,
		opts.IgnoreInvalidWatchPattern,
		redirectRevisions,
	)
	if err != nil {
		log.Error().Msgf("Failed to get applications: %v", err)
	}

	foundBaseApps := len(baseApps) > 0
	foundTargetApps := len(targetApps) > 0

	if !foundBaseApps && !foundTargetApps {
		log.Error().Msg("No applications found in either branch")
		os.Exit(0)
	}

	// Create cluster and install Argo CD
	provider := getClusterProvider(opts.ClusterType, opts.ClusterName)
	if err := provider.CreateCluster(); err != nil {
		log.Error().Msgf("Failed to create cluster: %v", err)
	}

	// Install Argo CD
	argocd := argocd.New(opts.ArgocdNamespace, opts.ArgocdChartVersion, "")
	if err := argocd.Install(opts.Debug, opts.SecretsFolder); err != nil {
		log.Error().Msgf("Failed to install Argo CD: %v", err)
	}

	tempFolder := "temp"
	if err := os.MkdirAll(tempFolder, dirMode); err != nil {
		log.Error().Msgf("Failed to create temp folder: %v", err)
	}

	// Generate applications from ApplicationSets
	baseApps, targetApps, err = parsing.ConvertAppSetsToAppsInBothBranches(
		argocd,
		baseApps,
		targetApps,
		baseBranch,
		targetBranch,
		opts.Repo,
		tempFolder,
		redirectRevisions,
	)
	if err != nil {
		log.Error().Msgf("Failed to generate base apps: %v", err)
	}

	// Write applications to files
	if err := utils.WriteApplications(baseApps, baseBranch, tempFolder); err != nil {
		log.Error().Msgf("Failed to write base apps: %v", err)
	}
	if err := utils.WriteApplications(targetApps, targetBranch, tempFolder); err != nil {
		log.Error().Msgf("Failed to write target apps: %v", err)
	}

	// Extract resources from the cluster based on each branch
	if err := extract.GetResourcesFromBothBranches(argocd, baseBranch, targetBranch, opts.Timeout, tempFolder, opts.OutputFolder); err != nil {
		log.Error().Msgf("Failed to get resources: %v", err)
	}

	// Generate diff between base and target branches
	if err := diff.GenerateDiff(
		opts.OutputFolder,
		baseBranch,
		targetBranch,
		&opts.DiffIgnore,
		opts.LineCount,
		opts.MaxDiffLength,
	); err != nil {
		log.Error().Msgf("Failed to generate diff: %v", err)
	}

	if !opts.KeepClusterAlive {
		provider.DeleteCluster(true)
	}
}
