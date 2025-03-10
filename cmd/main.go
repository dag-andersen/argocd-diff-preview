package main

import (
	"os"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplicaiton"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/diff"
	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/types"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
)

func main() {
	startTime := time.Now()

	opts := Parse()
	if opts == nil {
		return
	}

	defer func() {
		duration := time.Since(startTime)
		log.Info().Msgf("✨ Total execution time: %s", duration.Round(time.Millisecond))
	}()

	if err := run(opts); err != nil {
		log.Error().Msgf("❌ %v", err)
		if !opts.Debug {
			log.Info().Msg("🕵️ Run with '--debug' for more details")
		} else {
			log.Info().Msg("🐛 If you believe this error is caused by a bug, please open an issue on GitHub")
		}
		os.Exit(1)
	}
}

func run(opts *Options) error {

	// Get the parsed values from the options
	fileRegex := opts.GetFileRegex()
	selectors := opts.GetSelectors()
	filesChanged := opts.GetFilesChanged()
	redirectRevisions := opts.GetRedirectRevisions()
	clusterProvider := opts.GetClusterProvider()

	// Create branches
	baseBranch := types.NewBranch(opts.BaseBranch, types.Base)
	targetBranch := types.NewBranch(opts.TargetBranch, types.Target)

	// Get applications for both branches
	baseApps, targetApps, err := argoapplicaiton.GetApplicationsForBranches(
		opts.ArgocdNamespace,
		baseBranch,
		targetBranch,
		fileRegex,
		selectors,
		filesChanged,
		opts.Repo,
		opts.IgnoreInvalidWatchPattern,
		redirectRevisions,
	)
	if err != nil {
		log.Error().Msgf("❌ Failed to get applications")
		return err
	}

	foundBaseApps := len(baseApps) > 0
	foundTargetApps := len(targetApps) > 0

	if !foundBaseApps && !foundTargetApps {
		log.Info().Msg("Found no applications to process in either branch")

		// Write a message to the output file when no applications are found
		if err := utils.CreateFolder(opts.OutputFolder); err != nil {
			log.Error().Msgf("❌ Failed to create output folder: %s", opts.OutputFolder)
			return err
		}

		if err := diff.WriteNoAppsFoundMessage(opts.OutputFolder, selectors, filesChanged); err != nil {
			log.Error().Msgf("❌ Failed to write no apps found message")
			return err
		}

		return nil
	}

	// Create cluster and install Argo CD
	if err := clusterProvider.CreateCluster(); err != nil {
		log.Error().Msgf("❌ Failed to create cluster")
		return err
	}
	defer func() {
		if !opts.KeepClusterAlive {
			clusterProvider.DeleteCluster(true)
		} else {
			log.Info().Msg("🧟‍♂️ Cluster will be kept alive after the tool finishes")
		}
	}()

	// Install Argo CD
	argocd := argocd.New(opts.ArgocdNamespace, opts.ArgocdChartVersion, "")
	if err := argocd.Install(opts.Debug, opts.SecretsFolder); err != nil {
		log.Error().Msgf("❌ Failed to install Argo CD")
		return err
	}

	tempFolder := "temp"
	if err := utils.CreateFolder(tempFolder); err != nil {
		log.Error().Msgf("❌ Failed to create temp folder: %s", tempFolder)
		return err
	}

	// Generate applications from ApplicationSets
	baseApps, targetApps, err = argoapplicaiton.ConvertAppSetsToAppsInBothBranches(
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
		log.Error().Msgf("❌ Failed to generate apps from ApplicationSets")
		return err
	}

	if err := utils.CreateFolder(opts.OutputFolder); err != nil {
		log.Error().Msgf("❌ Failed to create output folder: %s", opts.OutputFolder)
		return err
	}

	// Write applications to files
	if err := argoapplicaiton.WriteApplications(baseApps, baseBranch, tempFolder); err != nil {
		log.Error().Msg("❌ Failed to write base apps")
		return err
	}
	if err := argoapplicaiton.WriteApplications(targetApps, targetBranch, tempFolder); err != nil {
		log.Error().Msg("❌ Failed to write target apps")
		return err
	}

	// Extract resources from the cluster based on each branch
	if err := extract.GetResourcesFromBothBranches(argocd, baseBranch, targetBranch, opts.Timeout, tempFolder, opts.OutputFolder); err != nil {
		log.Error().Msg("❌ Failed to extract resources")
		return err
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
		log.Error().Msg("❌ Failed to generate diff")
		return err
	}

	return nil
}
