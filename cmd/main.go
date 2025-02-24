package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/argocd"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/cluster"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/diff"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/extract"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/kind"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/minikube"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/parsing"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/utils"
)

type Options struct {
	debug                     bool
	timeout                   uint64
	fileRegex                 string
	diffIgnore                string
	lineCount                 uint
	argocdChartVersion        string
	baseBranch                string
	targetBranch              string
	repo                      string
	outputFolder              string
	secretsFolder             string
	clusterType               string
	clusterName               string
	maxDiffLength             uint
	selector                  string
	filesChanged              string
	ignoreInvalidWatchPattern bool
	keepClusterAlive          bool
	argocdNamespace           string
	redirectTargetRevisions   string
}

const (
	dirMode  = os.ModePerm // 0755 - read/write/execute for owner, read/execute for group and others
	fileMode = 0644        // 0644 - read/write for owner, read-only for group and others
)

func main() {
	opts := parseFlags()

	if opts.debug {
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	}

	// Parse selectors
	var selectors []types.Selector
	if opts.selector != "" {
		for _, s := range strings.Split(opts.selector, ",") {
			selector, err := types.FromString(strings.TrimSpace(s))
			if err != nil {
				log.Fatalf("Invalid selector format: %v", err)
			}
			selectors = append(selectors, *selector)
		}
	}

	// Parse files changed
	var filesChanged []string
	if opts.filesChanged != "" {
		// Split by comma or space
		filesChanged = strings.FieldsFunc(opts.filesChanged, func(r rune) bool {
			return r == ',' || r == ' '
		})
	}

	// Parse regex
	var regex *string
	if opts.fileRegex != "" {
		regex = &opts.fileRegex
	}

	// Parse redirect revisions
	var redirectRevisions []string
	if opts.redirectTargetRevisions != "" {
		redirectRevisions = strings.Split(opts.redirectTargetRevisions, ",")
	}

	// Create branches
	baseBranch := types.NewBranch(opts.baseBranch, types.Base)
	targetBranch := types.NewBranch(opts.targetBranch, types.Target)

	// Get applications for both branches
	baseApps, targetApps, err := parsing.GetApplicationsForBranches(
		opts.argocdNamespace,
		baseBranch,
		targetBranch,
		regex,
		selectors,
		filesChanged,
		opts.repo,
		opts.ignoreInvalidWatchPattern,
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

	var provider cluster.Provider
	switch opts.clusterType {
	case "kind":
		provider = kind.New(opts.clusterName)
	case "minikube":
		provider = minikube.New()
	case "auto":
		if kind.IsInstalled() {
			provider = kind.New(opts.clusterName)
			opts.clusterType = "kind"
		} else if minikube.IsInstalled() {
			provider = minikube.New()
			opts.clusterType = "minikube"
		} else {
			log.Fatal("No local cluster tool found. Please install kind or minikube")
		}
	default:
		log.Fatalf("Unsupported cluster type: %s", opts.clusterType)
	}

	if !provider.IsInstalled() {
		log.Fatalf("%s is not installed", opts.clusterType)
	}

	logOptions(opts)

	// // Create cluster and install Argo CD
	// if err := provider.CreateCluster(); err != nil {
	// 	log.Fatalf("Failed to create cluster: %v", err)
	// }

	argocd := argocd.New(opts.argocdNamespace, opts.argocdChartVersion, "")
	if err := argocd.Install(opts.debug); err != nil {
		log.Fatalf("Failed to install Argo CD: %v", err)
	}

	tempFolder := "temp"

	// Write applications to files
	if err := os.MkdirAll(tempFolder, dirMode); err != nil {
		log.Fatalf("Failed to create temp folder: %v", err)
	}

	if err := extract.GetResources(argocd, baseBranch, opts.timeout, opts.outputFolder, tempFolder); err != nil {
		log.Fatalf("Failed to get resources: %v", err)
	}

	// Generate applications from ApplicationSets
	baseApps, err = parsing.GenerateAppsFromAppSet(
		argocd,
		baseApps,
		baseBranch,
		opts.repo,
		tempFolder,
		redirectRevisions,
	)
	if err != nil {
		log.Fatalf("Failed to generate base apps: %v", err)
	}

	targetApps, err = parsing.GenerateAppsFromAppSet(
		argocd,
		targetApps,
		targetBranch,
		opts.repo,
		tempFolder,
		redirectRevisions,
	)
	if err != nil {
		log.Fatalf("Failed to generate target apps: %v", err)
	}

	// Write base apps
	baseAppsPath := fmt.Sprintf("%s/%s.yaml", tempFolder, baseBranch.FolderName())
	log.Printf("💾 Writing %d Applications from '%s' to ./%s",
		len(baseApps), baseBranch.Name, baseAppsPath)

	baseYaml := applicationsToString(baseApps)
	if err := os.WriteFile(baseAppsPath, []byte(baseYaml), fileMode); err != nil {
		log.Fatalf("Failed to write base apps: %v", err)
	}

	// Write target apps
	targetAppsPath := fmt.Sprintf("%s/%s.yaml", tempFolder, targetBranch.FolderName())
	log.Printf("💾 Writing %d Applications from '%s' to ./%s",
		len(targetApps), targetBranch.Name, targetAppsPath)

	targetYaml := applicationsToString(targetApps)
	if err := os.WriteFile(targetAppsPath, []byte(targetYaml), fileMode); err != nil {
		log.Fatalf("Failed to write target apps: %v", err)
	}

	// Apply files to cluster with kubectl
	if err := utils.ApplyManifest(baseAppsPath); err != nil {
		log.Fatalf("Failed to apply base apps: %v", err)
	}

	if err := extract.GetResources(argocd, baseBranch, opts.timeout, opts.outputFolder, tempFolder); err != nil {
		log.Fatalf("Failed to get resources: %v", err)
	}

	// delete applications
	if err := utils.DeleteManifest(baseAppsPath); err != nil {
		log.Fatalf("Failed to delete base apps: %v", err)
	}

	// apply target apps
	if err := utils.ApplyManifest(targetAppsPath); err != nil {
		log.Fatalf("Failed to apply target apps: %v", err)
	}

	if err := extract.GetResources(argocd, targetBranch, opts.timeout, opts.outputFolder, tempFolder); err != nil {
		log.Fatalf("Failed to get resources: %v", err)
	}

	// Generate diff between base and target branches
	var diffIgnore *string
	if opts.diffIgnore != "" {
		diffIgnore = &opts.diffIgnore
	}

	if err := diff.GenerateDiff(
		opts.outputFolder,
		baseBranch,
		targetBranch,
		diffIgnore,
		opts.lineCount,
		opts.maxDiffLength,
	); err != nil {
		log.Fatalf("Failed to generate diff: %v", err)
	}

	if !opts.keepClusterAlive {
		provider.DeleteCluster(true)
	}
}

func parseFlags() Options {
	opts := Options{}

	// Basic flags
	flag.BoolVar(&opts.debug, "debug", false, "Activate debug mode")
	flag.BoolVar(&opts.debug, "d", false, "Activate debug mode (shorthand)")
	flag.Uint64Var(&opts.timeout, "timeout", 180, "Set timeout in seconds")

	// File and diff related
	flag.StringVar(&opts.fileRegex, "file-regex", "", "Regex to filter files. Example: /apps_.*\\.yaml")
	flag.StringVar(&opts.fileRegex, "r", "", "Regex to filter files (shorthand)")
	flag.StringVar(&opts.diffIgnore, "diff-ignore", "", "Ignore lines in diff. Example: v[1,9]+.[1,9]+.[1,9]+ for ignoring version changes")
	flag.StringVar(&opts.diffIgnore, "i", "", "Ignore lines in diff (shorthand)")
	flag.UintVar(&opts.lineCount, "line-count", 10, "Generate diffs with <n> lines of context")
	flag.UintVar(&opts.lineCount, "c", 10, "Generate diffs with <n> lines of context (shorthand)")

	// Argo CD related
	flag.StringVar(&opts.argocdChartVersion, "argocd-chart-version", "", "Argo CD Helm Chart version")
	flag.StringVar(&opts.argocdNamespace, "argocd-namespace", "argocd", "Namespace to use for Argo CD")

	// Git related
	flag.StringVar(&opts.baseBranch, "base-branch", "main", "Base branch name")
	flag.StringVar(&opts.baseBranch, "b", "main", "Base branch name (shorthand)")
	flag.StringVar(&opts.targetBranch, "target-branch", "", "Target branch name")
	flag.StringVar(&opts.targetBranch, "t", "", "Target branch name (shorthand)")
	flag.StringVar(&opts.repo, "repo", "", "Git Repository. Format: OWNER/REPO")

	// Folders
	flag.StringVar(&opts.outputFolder, "output-folder", "./output", "Output folder where the diff will be saved")
	flag.StringVar(&opts.outputFolder, "o", "./output", "Output folder (shorthand)")
	flag.StringVar(&opts.secretsFolder, "secrets-folder", "./secrets", "Secrets folder where the secrets are read from")
	flag.StringVar(&opts.secretsFolder, "s", "./secrets", "Secrets folder (shorthand)")

	// Cluster related
	flag.StringVar(&opts.clusterType, "cluster", "auto", "Local cluster tool. Options: kind, minikube, auto")
	flag.StringVar(&opts.clusterName, "name", "argocd-diff-preview", "Cluster name (only for kind)")
	flag.BoolVar(&opts.keepClusterAlive, "keep-cluster-alive", false, "Keep cluster alive after the tool finishes")

	// Other options
	flag.UintVar(&opts.maxDiffLength, "max-diff-length", 65536, "Max diff message character count")
	flag.StringVar(&opts.selector, "selector", "", "Label selector to filter on (e.g. key1=value1,key2=value2)")
	flag.StringVar(&opts.selector, "l", "", "Label selector (shorthand)")
	flag.StringVar(&opts.filesChanged, "files-changed", "", "List of files changed between branches (comma or space separated)")
	flag.BoolVar(&opts.ignoreInvalidWatchPattern, "ignore-invalid-watch-pattern", false, "Ignore invalid watch pattern Regex on Applications")
	flag.StringVar(&opts.redirectTargetRevisions, "redirect-target-revisions", "", "List of target revisions to redirect")

	// Parse environment variables
	if envVal := os.Getenv("TIMEOUT"); envVal != "" {
		if val := parseUint64(envVal); val != 0 {
			opts.timeout = val
		}
	}
	// Add similar environment variable parsing for other flags...

	flag.Parse()
	return opts
}

func parseUint64(s string) uint64 {
	var val uint64
	if _, err := fmt.Sscanf(s, "%d", &val); err == nil {
		return val
	}
	return 0
}

func logOptions(opts Options) {
	log.Println("✨ Running with:")
	log.Printf("✨ - local-cluster-tool: %s", opts.clusterType)
	log.Printf("✨ - cluster-name: %s", opts.clusterName)
	log.Printf("✨ - base-branch: %s", opts.baseBranch)
	log.Printf("✨ - target-branch: %s", opts.targetBranch)
	log.Printf("✨ - secrets-folder: %s", opts.secretsFolder)
	log.Printf("✨ - output-folder: %s", opts.outputFolder)
	log.Printf("✨ - argocd-namespace: %s", opts.argocdNamespace)
	log.Printf("✨ - repo: %s", opts.repo)
	log.Printf("✨ - timeout: %d seconds", opts.timeout)

	if opts.keepClusterAlive {
		log.Println("✨ - keep-cluster-alive: true")
	}
	if opts.debug {
		log.Println("✨ - debug: true")
	}
	if opts.fileRegex != "" {
		log.Printf("✨ - file-regex: %s", opts.fileRegex)
	}
	if opts.diffIgnore != "" {
		log.Printf("✨ - diff-ignore: %s", opts.diffIgnore)
	}
	if opts.lineCount > 0 {
		log.Printf("✨ - line-count: %d", opts.lineCount)
	}
	if opts.argocdChartVersion != "" {
		log.Printf("✨ - argocd-version: %s", opts.argocdChartVersion)
	}
	if opts.maxDiffLength > 0 {
		log.Printf("✨ - max-diff-length: %d", opts.maxDiffLength)
	}
	if opts.filesChanged != "" {
		log.Printf("✨ - files-changed: %s", opts.filesChanged)
	}
	if opts.ignoreInvalidWatchPattern {
		log.Println("✨ Ignoring invalid watch patterns Regex on Applications")
	}
}

func applicationsToString(apps []types.ArgoResource) string {
	var yamlStrings []string
	for _, app := range apps {
		yamlStr, err := app.AsString()
		if err != nil {
			log.Printf("Failed to convert app %s to YAML: %v", app.Name, err)
			continue
		}
		// add a comment with the name of the file
		yamlStr = fmt.Sprintf("# File: %s\n%s", app.FileName, yamlStr)

		yamlStrings = append(yamlStrings, yamlStr)
	}
	return strings.Join(yamlStrings, "---\n")
}
