package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/diff"
	"github.com/dag-andersen/argocd-diff-preview/pkg/duplicates"
	"github.com/dag-andersen/argocd-diff-preview/pkg/extract"
	"github.com/dag-andersen/argocd-diff-preview/pkg/fileparsing"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/resource_filter"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

func main() {
	startTime := time.Now()

	cfg := Parse()
	if cfg == nil {
		return
	}

	defer func() {
		duration := time.Since(startTime)
		log.Info().Msgf("✨ Total execution time: %s", duration.Round(time.Second))
	}()

	if err := run(cfg); err != nil {
		log.Error().Msgf("❌ %v", err)
		helpMessage := extract.GetHelpMessage(err)
		if helpMessage != "" {
			log.Info().Msgf("💡 Help: %s", helpMessage)
		}
		if !cfg.Debug {
			log.Info().Msg("🕵️ Run with '--debug' for more details")
		} else {
			log.Info().Msg("🐛 If you believe this error is caused by a bug, please open an issue on GitHub")
		}
		os.Exit(1)
	}
}

func run(cfg *Config) error {
	startTime := time.Now()

	// Get values directly from the config - no getters needed
	fileRegex := cfg.FileRegex
	selectors := cfg.Selectors
	filesChanged := cfg.FilesChanged
	redirectRevisions := cfg.RedirectRevisions
	clusterProvider := cfg.ClusterProvider

	// Create unique ID only consisting of lowercase letters of 5 characters
	uniqueID := uuid.New().String()[:5]

	if !cfg.CreateCluster && !cfg.DryRun {
		log.Info().Msgf("🔑 Unique ID for this run: %s", uniqueID)
	}

	// Create branches
	baseBranch := git.NewBranch(cfg.BaseBranch, git.Base)
	targetBranch := git.NewBranch(cfg.TargetBranch, git.Target)

	if cfg.AutoDetectFilesChanged && len(filesChanged) == 0 {
		log.Info().Msg("🔍 Auto-detecting changed files")
		cf, duration, err := fileparsing.ListChangedFiles(baseBranch.FolderName(), targetBranch.FolderName())
		if err != nil {
			log.Error().Msgf("❌ Failed to auto-detect changed files: %s", err)
			return err
		}
		log.Info().Msgf("🔍 Found %d changed files in %s", len(cf), duration.Round(time.Second))
		filesChanged = cf
	}

	// Check if users limited the Application Selection
	searchIsLimited := len(selectors) > 0 || len(filesChanged) > 0 || fileRegex != nil

	appSelectionOptions := argoapplication.ApplicationSelectionOptions{
		Selector:                   selectors,
		FileRegex:                  fileRegex,
		FilesChanged:               filesChanged,
		IgnoreInvalidWatchPattern:  cfg.IgnoreInvalidWatchPattern,
		WatchIfNoWatchPatternFound: cfg.WatchIfNoWatchPatternFound,
	}

	// Get applications for both branches
	baseApps, targetApps, err := argoapplication.GetApplicationsForBranches(
		cfg.ArgocdNamespace,
		baseBranch,
		targetBranch,
		appSelectionOptions,
		cfg.Repo,
		redirectRevisions,
	)
	if err != nil {
		log.Error().Msgf("❌ Failed to get applications")
		return err
	}

	baseApps, targetApps = duplicates.RemoveIdenticalCopiesBetweenBranches(baseApps, targetApps)

	// If dry-run is enabled, show which applications would be processed and exit
	if cfg.DryRun {
		log.Info().Msg("💨 This is a dry run. The following application[sets] would be processed:")
		if len(baseApps.SelectedApps) > 0 {
			log.Info().Msgf("👇 Base Branch ('%s'):", baseBranch.Name)
			for _, app := range baseApps.SelectedApps {
				log.Info().Msgf("  - %s: %s (%s)", app.Kind.ShortName(), app.Name, app.FileName)
			}
		} else {
			log.Info().Msgf("🤷 No applications selected for the base branch ('%s').", baseBranch.Name)
		}

		if len(targetApps.SelectedApps) > 0 {
			log.Info().Msgf("👇 Target Branch ('%s'):", targetBranch.Name)
			for _, app := range targetApps.SelectedApps {
				log.Info().Msgf("  - %s: %s (%s)", app.Kind.ShortName(), app.Name, app.FileName)
			}
		} else {
			log.Info().Msgf("🤷 No applications selected for the target branch ('%s').", targetBranch.Name)
		}

		log.Info().Msg("✅ Dry run complete. No cluster was created and no diff was generated.")
		return nil
	}

	// Return if no applications are found
	foundBaseApps := len(baseApps.SelectedApps) > 0
	foundTargetApps := len(targetApps.SelectedApps) > 0
	if !foundBaseApps && !foundTargetApps {
		log.Info().Msg("👀 Found no applications to process in either branch")

		// Write a message to the output file when no applications are found
		if err := utils.CreateFolder(cfg.OutputFolder, true); err != nil {
			log.Error().Msgf("❌ Failed to create output folder: %s", cfg.OutputFolder)
			return err
		}

		if err := diff.WriteNoAppsFoundMessage(cfg.Title, cfg.OutputFolder, selectors, filesChanged); err != nil {
			log.Error().Msgf("❌ Failed to write no apps found message")
			return err
		}

		return nil
	}

	var clusterCreationDuration time.Duration
	if cfg.CreateCluster {
		// Create cluster and install Argo CD
		duration, err := clusterProvider.CreateCluster()
		if err != nil {
			log.Error().Msgf("❌ Failed to create cluster")
			return err
		}
		clusterCreationDuration = duration
	}

	defer func() {
		if cfg.CreateCluster {
			if !cfg.KeepClusterAlive {
				clusterProvider.DeleteCluster(true)
			} else {
				log.Info().Msg("🧟 Cluster will be kept alive after the tool finishes")
			}
		}
	}()

	// create k8s client
	k8sClient, err := utils.NewK8sClient(cfg.DisableClientThrottling)
	if err != nil {
		log.Error().Err(err).Msgf("❌ Failed to create k8s client")
		return err
	}

	// Delete old applications
	if !cfg.CreateCluster {
		ageInMinutes := 20
		if err := k8sClient.DeleteAllApplicationsOlderThan(cfg.ArgocdNamespace, ageInMinutes); err != nil {
			log.Error().Msgf("❌ Failed to delete old applications")
			return err
		}
	}

	argocd := argocd.New(
		k8sClient,
		cfg.ArgocdNamespace,
		cfg.ArgocdChartVersion,
		cfg.ArgocdChartName,
		cfg.ArgocdChartURL,
		cfg.ArgocdChartRepoUsername,
		cfg.ArgocdChartRepoPassword,
		cfg.ArgocdLoginOptions,
		cfg.UseArgoCDApi,
		cfg.ArgocdAuthToken,
	)

	// Ensure cleanup is performed when we exit (e.g., stopping port forwards)
	defer argocd.Cleanup()

	var argocdInstallationDuration time.Duration
	if cfg.CreateCluster {
		// Install Argo CD
		duration, err := argocd.Install(cfg.Debug, cfg.SecretsFolder)
		if err != nil {
			log.Error().Msgf("❌ Failed to install Argo CD")
			return err
		}
		argocdInstallationDuration = duration
	} else {
		duration, err := argocd.OnlyLogin()
		if err != nil {
			log.Error().Msgf("❌ Failed to login to Argo CD")
			return err
		}
		argocdInstallationDuration = duration
	}

	tempFolder := "temp"
	if err := utils.CreateFolder(tempFolder, true); err != nil {
		log.Error().Msgf("❌ Failed to clear temp folder: ./%s", tempFolder)
		return err
	}

	// Generate applications from ApplicationSets
	baseApps, targetApps, convertAppSetsToAppsDuration, err := argoapplication.ConvertAppSetsToAppsInBothBranches(
		argocd,
		baseApps,
		targetApps,
		baseBranch,
		targetBranch,
		cfg.Repo,
		tempFolder,
		redirectRevisions,
		cfg.Debug,
		appSelectionOptions,
	)
	if err != nil {
		log.Error().Msgf("❌ Failed to generate apps from ApplicationSets")
		return err
	}

	// Check for duplicates again
	baseApps, targetApps = duplicates.RemoveIdenticalCopiesBetweenBranches(baseApps, targetApps)

	// Return if no applications are found
	foundBaseApps = len(baseApps.SelectedApps) > 0
	foundTargetApps = len(targetApps.SelectedApps) > 0
	if !foundBaseApps && !foundTargetApps {
		log.Info().Msg("👀 Found no applications to render")

		// Write a message to the output file when no applications are found
		if err := utils.CreateFolder(cfg.OutputFolder, true); err != nil {
			log.Error().Msgf("❌ Failed to create output folder: %s", cfg.OutputFolder)
			return err
		}

		if err := diff.WriteNoAppsFoundMessage(cfg.Title, cfg.OutputFolder, selectors, filesChanged); err != nil {
			log.Error().Msgf("❌ Failed to write no apps found message")
			return err
		}

		return nil
	}

	// enure unique ids
	baseApps.SelectedApps = argoapplication.UniqueIds(baseApps.SelectedApps, baseBranch)
	targetApps.SelectedApps = argoapplication.UniqueIds(targetApps.SelectedApps, targetBranch)

	if err := utils.CreateFolder(cfg.OutputFolder, true); err != nil {
		log.Error().Msgf("❌ Failed to create output folder: %s", cfg.OutputFolder)
		return err
	}

	// Advice the user to limit the Application Selection
	if !searchIsLimited && (len(baseApps.SelectedApps) > 50 || len(targetApps.SelectedApps) > 50) {
		log.Warn().Msgf("💡 You are rendering %d Applications. You might want to limit the Application rendered on each run.", len(baseApps.SelectedApps)+len(targetApps.SelectedApps))
		log.Warn().Msg("💡 Check out the documentation under section `Application Selection` for more information.")
	}

	// For debugging purposes, we can still write the manifests to files
	if cfg.Debug {
		// Generate application manifests as strings
		baseManifest := argoapplication.ApplicationsToString(baseApps.SelectedApps)
		targetManifest := argoapplication.ApplicationsToString(targetApps.SelectedApps)
		if err := utils.WriteFile(fmt.Sprintf("%s/%s.yaml", tempFolder, baseBranch.FolderName()), baseManifest); err != nil {
			log.Error().Msg("❌ Failed to write base apps")
			return err
		}
		if err := utils.WriteFile(fmt.Sprintf("%s/%s.yaml", tempFolder, targetBranch.FolderName()), targetManifest); err != nil {
			log.Error().Msg("❌ Failed to write target apps")
			return err
		}
	}

	// Store info about how many aps were skipped
	selectionInfo := diff.ConvertArgoSelectionToSelectionInfo(baseApps, targetApps)

	// Extract resources from the cluster based on each branch, passing the manifests directly
	deleteAfterProcessing := !cfg.CreateCluster
	baseManifests, targetManifests, extractDuration, err := extract.RenderApplicationsFromBothBranches(
		argocd,
		cfg.Timeout,
		baseApps.SelectedApps,
		targetApps.SelectedApps,
		uniqueID,
		deleteAfterProcessing,
	)
	if err != nil {
		log.Error().Msg("❌ Failed to extract resources")
		return err
	}

	baseAppInfos, err := convertExtractedAppsToAppInfos(baseManifests, cfg.IgnoreResourceRules)
	if err != nil {
		log.Error().Msg("❌ Failed to convert extracted apps to yaml")
		return err
	}
	targetAppInfos, err := convertExtractedAppsToAppInfos(targetManifests, cfg.IgnoreResourceRules)
	if err != nil {
		log.Error().Msg("❌ Failed to convert extracted apps to yaml")
		return err
	}

	// Print manifests output
	{
		var baseAppCombinedYaml []string
		var targetAppCombinedYaml []string
		for _, app := range baseAppInfos {
			if app.FileContent != "" {
				baseAppCombinedYaml = append(baseAppCombinedYaml, app.FileContent)
			}
		}
		for _, app := range targetAppInfos {
			if app.FileContent != "" {
				targetAppCombinedYaml = append(targetAppCombinedYaml, app.FileContent)
			}
		}
		if err := utils.WriteFile(fmt.Sprintf("%s/%s.yaml", cfg.OutputFolder, baseBranch.FolderName()), strings.Join(baseAppCombinedYaml, "\n---\n")); err != nil {
			log.Error().Msg("❌ Failed to write base manifests")
			return err
		}
		if err := utils.WriteFile(fmt.Sprintf("%s/%s.yaml", cfg.OutputFolder, targetBranch.FolderName()), strings.Join(targetAppCombinedYaml, "\n---\n")); err != nil {
			log.Error().Msg("❌ Failed to write target manifests")
			return err
		}
	}

	// Create info box for storing run time information
	statsInfo := diff.StatsInfo{
		FullDuration:               time.Since(startTime),
		ExtractDuration:            extractDuration + convertAppSetsToAppsDuration,
		ArgoCDInstallationDuration: argocdInstallationDuration,
		ClusterCreationDuration:    clusterCreationDuration,
		ApplicationCount:           len(baseApps.SelectedApps) + len(targetApps.SelectedApps),
	}

	// Generate diff between base and target branches
	if err := diff.GenerateDiff(
		cfg.Title,
		cfg.OutputFolder,
		baseBranch,
		targetBranch,
		baseAppInfos,
		targetAppInfos,
		&cfg.DiffIgnore,
		cfg.LineCount,
		cfg.MaxDiffLength,
		cfg.HideDeletedAppDiff,
		statsInfo,
		selectionInfo,
		cfg.ArgocdUIURL,
	); err != nil {
		log.Error().Msg("❌ Failed to generate diff")
		return err
	}

	log.Info().Msgf("⏰ Run time stats: %s", statsInfo.Stats())

	return nil
}

// convertExtractedAppsToAppInfos converts a list of ExtractedApp to a list of AppInfo
func convertExtractedAppsToAppInfos(extractedApps []extract.ExtractedApp, ignoreResourceRules []resource_filter.IgnoreResourceRule) ([]diff.AppInfo, error) {
	appInfos := make([]diff.AppInfo, len(extractedApps))
	for i, extractedApp := range extractedApps {
		manifestString, err := extractedApp.FlattenToString(ignoreResourceRules)
		if err != nil {
			return nil, err
		}
		appInfos[i] = diff.AppInfo{
			Id:          extractedApp.Id,
			Name:        extractedApp.Name,
			SourcePath:  extractedApp.SourcePath,
			FileContent: manifestString,
		}
	}
	return appInfos, nil
}
