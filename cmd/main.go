package main

import (
	"log"
	"os"

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
			log.Fatal("No local cluster tool found. Please install kind or minikube")
		}
	default:
		log.Fatalf("Unsupported cluster type: %s", clusterType)
	}

	if !provider.IsInstalled() {
		log.Fatalf("%s is not installed", clusterType)
	}

	return provider
}

func main() {

	// Parse input options
	opts := options.Parse()
	opts.LogOptions()

	if opts.Debug {
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	}

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
		log.Fatalf("Failed to get applications: %v", err)
	}

	foundBaseApps := len(baseApps) > 0
	foundTargetApps := len(targetApps) > 0

	if !foundBaseApps && !foundTargetApps {
		log.Println("No applications found in either branch")
		os.Exit(0)
	}

	// Create cluster and install Argo CD
	provider := getClusterProvider(opts.ClusterType, opts.ClusterName)
	if err := provider.CreateCluster(); err != nil {
		log.Fatalf("Failed to create cluster: %v", err)
	}

	// Install Argo CD
	argocd := argocd.New(opts.ArgocdNamespace, opts.ArgocdChartVersion, "")
	if err := argocd.Install(opts.Debug); err != nil {
		log.Fatalf("Failed to install Argo CD: %v", err)
	}

	// Generate applications from ApplicationSets

	tempFolder := "temp"
	if err := os.MkdirAll(tempFolder, dirMode); err != nil {
		log.Fatalf("Failed to create temp folder: %v", err)
	}

	// Base branch
	baseApps, err = parsing.ConvertAppSetsToApps(
		argocd,
		baseApps,
		baseBranch,
		opts.Repo,
		tempFolder,
		redirectRevisions,
	)
	if err != nil {
		log.Fatalf("Failed to generate base apps: %v", err)
	}

	// Target branch
	targetApps, err = parsing.ConvertAppSetsToApps(
		argocd,
		targetApps,
		targetBranch,
		opts.Repo,
		tempFolder,
		redirectRevisions,
	)
	if err != nil {
		log.Fatalf("Failed to generate target apps: %v", err)
	}

	// Write applications to files
	if err := utils.WriteApplications(baseApps, baseBranch, tempFolder); err != nil {
		log.Fatalf("Failed to write base apps: %v", err)
	}
	if err := utils.WriteApplications(targetApps, targetBranch, tempFolder); err != nil {
		log.Fatalf("Failed to write target apps: %v", err)
	}

	// Extract resources from the cluster based on each branch
	if err := extract.GetResourcesFromBothBranches(argocd, baseBranch, targetBranch, opts.Timeout, tempFolder, opts.OutputFolder); err != nil {
		log.Fatalf("Failed to get resources: %v", err)
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
		log.Fatalf("Failed to generate diff: %v", err)
	}

	if !opts.KeepClusterAlive {
		provider.DeleteCluster(true)
	}
}
