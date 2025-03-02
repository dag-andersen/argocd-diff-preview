package main

import (
	"os"
	"time"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/argocd"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/diff"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/extract"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/options"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/parsing"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

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
	clusterProvider := opts.ParseClusterType()

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
		log.Info().Msg("Found no applications to process in either branch")

		// Write a message to the output file when no applications are found
		if err := utils.CreateFolder(opts.OutputFolder); err != nil {
			log.Error().Msgf("Failed to create output folder: %v", err)
		}

		if err := diff.WriteNoAppsFoundMessage(opts.OutputFolder, selectors, filesChanged); err != nil {
			log.Error().Msgf("Failed to write no apps found message: %v", err)
		}

		os.Exit(0)
	}

	// Create cluster and install Argo CD
	if err := clusterProvider.CreateCluster(); err != nil {
		log.Error().Msgf("Failed to create cluster: %v", err)
	}

	// Install Argo CD
	argocd := argocd.New(opts.ArgocdNamespace, opts.ArgocdChartVersion, "")
	if err := argocd.Install(opts.Debug, opts.SecretsFolder); err != nil {
		log.Fatal().Msgf("Failed to install Argo CD: %v", err)
	}

	tempFolder := "temp"
	if err := utils.CreateFolder(tempFolder); err != nil {
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
		opts.Debug,
	)
	if err != nil {
		log.Fatal().Msgf("Failed to generate base apps: %v", err)
	}

	if err := utils.CreateFolder(opts.OutputFolder); err != nil {
		log.Error().Msgf("Failed to create output folder: %v", err)
	}

	// Write applications to files
	if err := utils.WriteApplications(baseApps, baseBranch, tempFolder); err != nil {
		log.Fatal().Msgf("Failed to write base apps: %v", err)
	}
	if err := utils.WriteApplications(targetApps, targetBranch, tempFolder); err != nil {
		log.Fatal().Msgf("Failed to write target apps: %v", err)
	}

	// Extract resources from the cluster based on each branch
	if err := extract.GetResourcesFromBothBranches(argocd, baseBranch, targetBranch, opts.Timeout, tempFolder, opts.OutputFolder); err != nil {
		log.Fatal().Msgf("Failed to get resources: %v", err)
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
		log.Fatal().Msgf("Failed to generate diff: %v", err)
	}

	if !opts.KeepClusterAlive {
		clusterProvider.DeleteCluster(true)
	}
}
