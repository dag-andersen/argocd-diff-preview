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
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

func main() {
	startTime := time.Now()

	opts := Parse()
	if opts == nil {
		return
	}

	defer func() {
		duration := time.Since(startTime)
		log.Info().Msgf("✨ Total execution time: %s", duration.Round(time.Second))
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
	startTime := time.Now()

	// Get the parsed values from the options
	fileRegex := opts.GetFileRegex()
	selectors := opts.GetSelectors()
	filesChanged := opts.GetFilesChanged()
	redirectRevisions := opts.GetRedirectRevisions()
	clusterProvider := opts.GetClusterProvider()

	// Create unique ID only consisting of lowercase letters of 5 characters
	uniqueID := uuid.New().String()[:5]

	if !opts.CreateCluster {
		log.Info().Msgf("🔑 Unique ID for this run: %s", uniqueID)
	}

	// Check if users limited the Application Selection
	searchIsLimited := len(selectors) > 0 || len(filesChanged) > 0 || fileRegex != nil

	// Create branches
	baseBranch := git.NewBranch(opts.BaseBranch, git.Base)
	targetBranch := git.NewBranch(opts.TargetBranch, git.Target)

	filterOptions := argoapplication.FilterOptions{
		Selector:                  selectors,
		FilesChanged:              filesChanged,
		IgnoreInvalidWatchPattern: opts.IgnoreInvalidWatchPattern,
	}

	// Get applications for both branches
	baseApps, targetApps, err := argoapplication.GetApplicationsForBranches(
		opts.ArgocdNamespace,
		baseBranch,
		targetBranch,
		fileRegex,
		filterOptions,
		opts.Repo,
		redirectRevisions,
	)
	if err != nil {
		log.Error().Msgf("❌ Failed to get applications")
		return err
	}

	baseApps, targetApps = duplicates.RemoveDuplicates(baseApps, targetApps)

	// Return if no applications are found
	foundBaseApps := len(baseApps) > 0
	foundTargetApps := len(targetApps) > 0
	if !foundBaseApps && !foundTargetApps {
		log.Info().Msg("👀 Found no applications to process in either branch")

		// Write a message to the output file when no applications are found
		if err := utils.CreateFolder(opts.OutputFolder, true); err != nil {
			log.Error().Msgf("❌ Failed to create output folder: %s", opts.OutputFolder)
			return err
		}

		if err := diff.WriteNoAppsFoundMessage(opts.Title, opts.OutputFolder, selectors, filesChanged); err != nil {
			log.Error().Msgf("❌ Failed to write no apps found message")
			return err
		}

		return nil
	}

	var clusterCreationDuration time.Duration
	if opts.CreateCluster {
		// Create cluster and install Argo CD
		duration, err := clusterProvider.CreateCluster()
		if err != nil {
			log.Error().Msgf("❌ Failed to create cluster")
			return err
		}
		clusterCreationDuration = duration
	}

	defer func() {
		if opts.CreateCluster {
			if !opts.KeepClusterAlive {
				clusterProvider.DeleteCluster(true)
			} else {
				log.Info().Msg("🧟‍♂️ Cluster will be kept alive after the tool finishes")
			}
		}
	}()

	// create k8s client
	k8sClient, err := utils.NewK8sClient()
	if err != nil {
		log.Error().Msgf("❌ Failed to create k8s client")
		return err
	}

	// Delete old applications
	if !opts.CreateCluster {
		ageInMinutes := 20
		if err := k8sClient.DeleteAllApplicationsOlderThan(opts.ArgocdNamespace, ageInMinutes); err != nil {
			log.Error().Msgf("❌ Failed to delete old applications")
			return err
		}
	}

	argocd := argocd.New(k8sClient, opts.ArgocdNamespace, opts.ArgocdChartVersion, "")

	var argocdInstallationDuration time.Duration
	if opts.CreateCluster {
		// Install Argo CD
		duration, err := argocd.Install(opts.Debug, opts.SecretsFolder)
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
		log.Error().Msgf("❌ Failed to create temp folder: %s", tempFolder)
		return err
	}

	// Generate applications from ApplicationSets
	baseApps, targetApps, err = argoapplication.ConvertAppSetsToAppsInBothBranches(
		argocd,
		baseApps,
		targetApps,
		baseBranch,
		targetBranch,
		opts.Repo,
		tempFolder,
		redirectRevisions,
		opts.Debug,
		filterOptions,
	)
	if err != nil {
		log.Error().Msgf("❌ Failed to generate apps from ApplicationSets")
		return err
	}

	// Check for duplicates again
	baseApps, targetApps = duplicates.RemoveDuplicates(baseApps, targetApps)

	// Return if no applications are found
	foundBaseApps = len(baseApps) > 0
	foundTargetApps = len(targetApps) > 0
	if !foundBaseApps && !foundTargetApps {
		log.Info().Msg("👀 Found no applications to render")

		// Write a message to the output file when no applications are found
		if err := utils.CreateFolder(opts.OutputFolder, true); err != nil {
			log.Error().Msgf("❌ Failed to create output folder: %s", opts.OutputFolder)
			return err
		}

		if err := diff.WriteNoAppsFoundMessage(opts.Title, opts.OutputFolder, selectors, filesChanged); err != nil {
			log.Error().Msgf("❌ Failed to write no apps found message")
			return err
		}

		return nil
	}

	// enure unique ids
	baseApps = argoapplication.UniqueIds(baseApps, baseBranch)
	targetApps = argoapplication.UniqueIds(targetApps, targetBranch)

	if err := utils.CreateFolder(opts.OutputFolder, true); err != nil {
		log.Error().Msgf("❌ Failed to create output folder: %s", opts.OutputFolder)
		return err
	}

	// Advice the user to limit the Application Selection
	if !searchIsLimited && (len(baseApps) > 50 || len(targetApps) > 50) {
		log.Warn().Msgf("💡 You are rendering %d Applications. You might want to limit the Application rendered on each run.", len(baseApps)+len(targetApps))
		log.Warn().Msg("💡 Check out the documentation under section `Application Selection` for more information.")
	}

	// For debugging purposes, we can still write the manifests to files
	if opts.Debug {
		// Generate application manifests as strings
		baseManifest := argoapplication.ApplicationsToString(baseApps)
		targetManifest := argoapplication.ApplicationsToString(targetApps)
		if err := utils.WriteFile(fmt.Sprintf("%s/%s.yaml", tempFolder, baseBranch.FolderName()), baseManifest); err != nil {
			log.Error().Msg("❌ Failed to write base apps")
			return err
		}
		if err := utils.WriteFile(fmt.Sprintf("%s/%s.yaml", tempFolder, targetBranch.FolderName()), targetManifest); err != nil {
			log.Error().Msg("❌ Failed to write target apps")
			return err
		}
	}

	// Extract resources from the cluster based on each branch, passing the manifests directly
	deleteAfterProcessing := !opts.CreateCluster
	baseManifests, targetManifests, extractDuration, err := extract.GetResourcesFromBothBranches(
		argocd,
		opts.Timeout,
		baseApps,
		targetApps,
		uniqueID,
		deleteAfterProcessing,
	)
	if err != nil {
		log.Error().Msg("❌ Failed to extract resources")
		return err
	}

	baseAppInfos, err := convertExtractedAppsToAppInfos(baseManifests)
	if err != nil {
		log.Error().Msg("❌ Failed to convert extracted apps to yaml")
		return err
	}
	targetAppInfos, err := convertExtractedAppsToAppInfos(targetManifests)
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
		if err := utils.WriteFile(fmt.Sprintf("%s/%s.yaml", opts.OutputFolder, baseBranch.FolderName()), strings.Join(baseAppCombinedYaml, "\n---\n")); err != nil {
			log.Error().Msg("❌ Failed to write base manifests")
			return err
		}
		if err := utils.WriteFile(fmt.Sprintf("%s/%s.yaml", opts.OutputFolder, targetBranch.FolderName()), strings.Join(targetAppCombinedYaml, "\n---\n")); err != nil {
			log.Error().Msg("❌ Failed to write target manifests")
			return err
		}
	}

	// Create info box for storing run time information
	infoBox := diff.InfoBox{
		ExtractDuration:            extractDuration,
		ArgoCDInstallationDuration: argocdInstallationDuration,
		ClusterCreationDuration:    clusterCreationDuration,
		FullDuration:               time.Since(startTime),
		ApplicationCount:           len(baseApps) + len(targetApps),
	}

	// Generate diff between base and target branches
	if err := diff.GenerateDiff(
		opts.Title,
		opts.OutputFolder,
		baseBranch,
		targetBranch,
		baseAppInfos,
		targetAppInfos,
		&opts.DiffIgnore,
		opts.LineCount,
		opts.MaxDiffLength,
		infoBox,
	); err != nil {
		log.Error().Msg("❌ Failed to generate diff")
		return err
	}

	return nil
}

// convertExtractedAppsToAppInfos converts a list of ExtractedApp to a list of AppInfo
func convertExtractedAppsToAppInfos(extractedApps []extract.ExtractedApp) ([]diff.AppInfo, error) {
	appInfos := make([]diff.AppInfo, len(extractedApps))
	for i, extractedApp := range extractedApps {
		appInfo, err := convertExtractedAppToAppInfo(extractedApp)
		if err != nil {
			return nil, err
		}
		appInfos[i] = appInfo
	}
	return appInfos, nil
}

// convertExtractedAppToAppInfo converts an ExtractedApp to an AppInfo
func convertExtractedAppToAppInfo(extractedApp extract.ExtractedApp) (diff.AppInfo, error) {
	yamlString, err := convertToYamlString(&extractedApp)
	if err != nil {
		log.Error().Msgf("❌ Failed to convert extracted app to yaml string: %s", err)
		return diff.AppInfo{}, err
	}

	return diff.AppInfo{
		Id:          extractedApp.Id,
		Name:        extractedApp.Name,
		SourcePath:  extractedApp.SourcePath,
		FileContent: yamlString,
	}, nil
}

// convertToYamlString converts a list of ExtractedApp to a single YAML string
func convertToYamlString(apps *extract.ExtractedApp) (string, error) {
	var manifestStrings []string
	for _, manifest := range apps.Manifest {
		manifestString, err := yaml.Marshal(manifest.Object)
		if err != nil {
			return "", fmt.Errorf("failed to marshal unstructured object: %w", err)
		}
		manifestStrings = append(manifestStrings, string(manifestString))
	}
	return strings.Join(manifestStrings, "\n---\n"), nil
}
