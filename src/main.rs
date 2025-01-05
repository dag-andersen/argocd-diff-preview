use argo_resource::ArgoResource;
use branch::{Branch, BranchType};
use error::{CommandError, CommandOutput};
use log::{debug, error, info};
use regex::Regex;
use selector::Selector;
use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;
use std::str::FromStr;
use std::{error::Error, io::Write};
use structopt::StructOpt;
use utils::{check_if_folder_exists, create_folder_if_not_exists, run_command};
mod argo_resource;
mod argocd;
mod branch;
mod diff;
mod error;
mod extract;
mod kind;
mod minikube;
mod no_apps_found;
mod parsing;
mod selector;
mod utils;

#[derive(Debug, StructOpt)]
#[structopt(
    name = "argocd-diff-preview",
    about = "A tool that generates the diff between two branches"
)]
struct Opt {
    /// Activate debug mode
    // short and long flags (-d, --debug) will be deduced from the field's name
    #[structopt(short, long)]
    debug: bool,

    /// Set timeout
    #[structopt(long, default_value = "180", env)]
    timeout: u64,

    /// Regex to filter files. Example: "/apps_.*\.yaml"
    #[structopt(short = "r", long, env)]
    file_regex: Option<String>,

    /// Ignore lines in diff. Example: use 'v[1,9]+.[1,9]+.[1,9]+' for ignoring changes caused by version changes following semver
    #[structopt(short = "i", long, env)]
    diff_ignore: Option<String>,

    /// Generate diffs with <n> lines above and below the highlighted changes in the diff. Default: 10
    #[structopt(short = "c", long, env)]
    line_count: Option<usize>,

    /// Argo CD Helm Chart version.
    #[structopt(long, env)]
    argocd_chart_version: Option<String>,

    /// Base branch name
    #[structopt(short, long, default_value = "main", env)]
    base_branch: String,

    /// Target branch name
    #[structopt(short, long, env)]
    target_branch: String,

    /// Git Repository. Format: OWNER/REPO
    #[structopt(long = "repo", env)]
    repo: String,

    /// Output folder where the diff will be saved
    #[structopt(short, long, default_value = "./output", env)]
    output_folder: String,

    /// Secrets folder where the secrets are read from
    #[structopt(short, long, default_value = "./secrets", env)]
    secrets_folder: String,

    /// Local cluster tool. Options: kind, minikube, auto. Default: Auto
    #[structopt(long, env, default_value = "auto")]
    local_cluster_tool: ClusterTool,

    /// Max diff message character count. Default: 65536 (GitHub comment limit)
    #[structopt(long, env)]
    max_diff_length: Option<usize>,

    /// Label selector to filter on, supports '=', '==', and '!='. (e.g. -l key1=value1,key2=value2).
    #[structopt(long, short = "l", env)]
    selector: Option<String>,

    /// List of files changed between the two branches. Input must be a comma or space separated string. When provided, only Applications watching these files will be rendered.
    #[structopt(long, env)]
    files_changed: Option<String>,

    /// Ignore invalid watch pattern Regex on Applications. If flag is unset and an invalid Regex is found, the tool will exit with an error.
    #[structopt(long)]
    ignore_invalid_watch_pattern: bool,

    /// Cluster name (only applicable to kind)
    #[structopt(long, default_value = "argocd-diff-preview", env)]
    cluster_name: String,

    /// Namespace to use for Argo CD
    #[structopt(long, default_value = "argocd", env)]
    argocd_namespace: String,

    /// Keep cluster alive after the tool finishes
    #[structopt(long)]
    keep_cluster_alive: bool,
}

#[derive(Debug)]
enum ClusterTool {
    Kind,
    Minikube,
}

impl FromStr for ClusterTool {
    type Err = &'static str;
    fn from_str(day: &str) -> Result<Self, Self::Err> {
        match day.to_lowercase().as_str() {
            "kind" => Ok(ClusterTool::Kind),
            "minikube" => Ok(ClusterTool::Minikube),
            "auto" if kind::is_installed() => Ok(ClusterTool::Kind),
            "auto" if minikube::is_installed() => Ok(ClusterTool::Minikube),
            _ => Err("No local cluster tool found. Please install kind or minikube"),
        }
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    run().await.inspect_err(|e| {
        let opt = Opt::from_args();
        error!("❌ {}", e);
        if !opt.keep_cluster_alive {
            cleanup_cluster(opt.local_cluster_tool, &opt.cluster_name);
        }
    })
}

async fn run() -> Result<(), Box<dyn Error>> {
    let opt = Opt::from_args();

    // Start timer
    let start = std::time::Instant::now();

    if opt.debug {
        std::env::set_var("RUST_LOG", "debug");
        env_logger::init();
    } else {
        std::env::set_var("RUST_LOG", "info");
        env_logger::builder()
            .format(|buf, record| writeln!(buf, "{}", record.args()))
            .init();
    }

    debug!("Arguments provided: {:?}", opt);

    let file_regex = opt
        .file_regex
        .filter(|f| !f.trim().is_empty())
        .map(|f| Regex::new(&f))
        .transpose()?;

    let base_branch_name = opt.base_branch.trim();
    let target_branch_name = opt.target_branch.trim();
    let repo = opt.repo.trim();
    let diff_ignore = opt.diff_ignore.filter(|f| !f.trim().is_empty());
    let timeout = opt.timeout;
    let output_folder = opt.output_folder.as_str();
    let secrets_folder = opt.secrets_folder.as_str();
    let line_count = opt.line_count;
    let cluster_name = opt.cluster_name;
    let argocd_namespace = opt.argocd_namespace;
    let argocd_version = opt
        .argocd_chart_version
        .as_deref()
        .filter(|f| !f.trim().is_empty());
    let keep_cluster_alive = opt.keep_cluster_alive;
    let max_diff_length = opt.max_diff_length;
    let files_changed: Option<Vec<String>> = opt
        .files_changed
        .map(|f| f.trim().to_string())
        .filter(|f| !f.is_empty())
        .map(|f| {
            (if f.contains(',') {
                f.split(',')
            } else {
                f.split(' ')
            })
            .map(|s| s.trim().to_string())
            .collect()
        });

    let cluster_tool = &opt.local_cluster_tool;

    let repo_regex = Regex::new(r"^[a-zA-Z0-9-]+/[a-zA-Z0-9\._-]+$")?;
    if !repo_regex.is_match(repo) {
        error!("❌ Invalid repository format. Please use OWNER/REPO");
        return Err("Invalid repository format".into());
    }

    info!("✨ Running with:");
    info!("✨ - local-cluster-tool: {:?}", cluster_tool);
    info!("✨ - cluster-name: {}", cluster_name);
    info!("✨ - base-branch: {}", base_branch_name);
    info!("✨ - target-branch: {}", target_branch_name);
    info!("✨ - secrets-folder: {}", secrets_folder);
    info!("✨ - output-folder: {}", output_folder);
    info!("✨ - argocd-namespace: {}", argocd_namespace);
    info!("✨ - repo: {}", repo);
    info!("✨ - timeout: {} seconds", timeout);
    if keep_cluster_alive {
        info!("✨ - keep-cluster-alive: true");
    }
    if let Some(a) = file_regex.clone() {
        info!("✨ - file-regex: {}", a.as_str());
    }
    if let Some(a) = diff_ignore.clone() {
        info!("✨ - diff-ignore: {}", a);
    }
    if let Some(a) = line_count {
        info!("✨ - line-count: {}", a);
    }
    if let Some(a) = argocd_version {
        info!("✨ - argocd-version: {}", a);
    }
    if let Some(a) = max_diff_length {
        info!("✨ - max-diff-length: {}", a);
    }
    if let Some(a) = files_changed.clone() {
        info!("✨ - files-changed: {:?}", a);
    }
    if opt.ignore_invalid_watch_pattern {
        info!("✨ Ignoring invalid watch patterns Regex on Applications");
    }

    let base_branch = Branch {
        name: base_branch_name.to_string(),
        branch_type: BranchType::Base,
    };

    let target_branch = Branch {
        name: target_branch_name.to_string(),
        branch_type: BranchType::Target,
    };

    // label selectors can be fined in the following format: key1==value1,key2=value2,key3!=value3
    let selector = opt.selector.filter(|s| !s.trim().is_empty()).map(|s| {
        let labels: Vec<Selector> = s
            .split(',')
            .filter(|l| !l.trim().is_empty())
            .map(|l| Selector::from(l).expect("Invalid label selector format"))
            .collect();
        labels
    });

    if let Some(list) = &selector {
        info!(
            "✨ - selector: {}",
            list.iter()
                .map(|s| s.to_string())
                .collect::<Vec<String>>()
                .join(",")
        );
    }

    if !check_if_folder_exists(base_branch.folder_name()) {
        error!(
            "❌ Base branch folder does not exist: {}",
            base_branch.folder_name()
        );
        return Err("Base branch folder does not exist".into());
    }

    if !check_if_folder_exists(target_branch.folder_name()) {
        error!(
            "❌ Target branch folder does not exist: {}",
            target_branch.folder_name()
        );
        return Err("Target branch folder does not exist".into());
    }

    // remove .git from repo
    let repo = repo.trim_end_matches(".git");
    let (base_apps, target_apps) = parsing::get_applications_for_branches(
        &argocd_namespace,
        &base_branch,
        &target_branch,
        &file_regex,
        &selector,
        &files_changed,
        repo,
        opt.ignore_invalid_watch_pattern,
    )?;

    let base_apps = unique_names(base_apps, &base_branch);
    let target_apps = unique_names(target_apps, &target_branch);

    let found_base_apps = !base_apps.is_empty();
    let found_target_apps = !target_apps.is_empty();

    if !found_base_apps && !found_target_apps {
        info!("👀 Nothing to compare");
        info!("👀 If this doesn't seem right, try running the tool with '--debug' to get more details about what is happening");
        no_apps_found::write_message(output_folder, &selector, &files_changed)?;
        info!("🎉 Done in {} seconds", start.elapsed().as_secs());
        return Ok(());
    }

    {
        info!(
            "💾 Writing {} applications from '{}' to ./{}",
            base_apps.len(),
            base_branch.name,
            base_branch.app_file()
        );
        utils::write_to_file(base_branch.app_file(), &applications_to_string(base_apps)?)?;
        info!(
            "💾 Writing {} applications from '{}' to ./{}",
            target_apps.len(),
            target_branch.name,
            target_branch.app_file()
        );
        utils::write_to_file(
            target_branch.app_file(),
            &applications_to_string(target_apps)?,
        )?;
    }

    match cluster_tool {
        ClusterTool::Kind => kind::create_cluster(&cluster_name)?,
        ClusterTool::Minikube => minikube::create_cluster()?,
    }

    create_namespace(&argocd_namespace)?;

    create_folder_if_not_exists(secrets_folder)?;
    match apply_folder(secrets_folder) {
        Ok(count) if count > 0 => info!("🤫 Applied {} secrets", count),
        Ok(_) => info!("🤷 No secrets found in {}", secrets_folder),
        Err(e) => {
            error!("❌ Failed to apply secrets");
            return Err(e.into());
        }
    }

    let argocd = argocd::ArgoCDInstallation::new(
        &argocd_namespace,
        argocd_version.map(|v| v.to_string()),
        None,
    );

    argocd.install_argo_cd(opt.debug).await?;

    // Cleanup output folder
    clean_output_folder(output_folder)?;

    // Extract resources from Argo CD
    if found_base_apps {
        extract::get_resources(&argocd, &base_branch, timeout, output_folder).await?;
        if found_target_apps {
            extract::delete_applications().await?;
            tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
        }
    }
    if found_target_apps {
        extract::get_resources(&argocd, &target_branch, timeout, output_folder).await?;
    }

    // Delete cluster
    if !keep_cluster_alive {
        match cluster_tool {
            ClusterTool::Kind => kind::delete_cluster(&cluster_name, false),
            ClusterTool::Minikube => minikube::delete_cluster(false),
        }
    }

    diff::generate_diff(
        output_folder,
        &base_branch,
        &target_branch,
        diff_ignore,
        line_count,
        max_diff_length,
    )?;

    info!("🎉 Done in {} seconds", start.elapsed().as_secs());

    Ok(())
}

fn clean_output_folder(output_folder: &str) -> Result<(), Box<dyn Error>> {
    create_folder_if_not_exists(output_folder)?;
    fs::remove_dir_all(format!("{}/{}", output_folder, BranchType::Base)).unwrap_or_default();
    fs::remove_dir_all(format!("{}/{}", output_folder, BranchType::Target)).unwrap_or_default();
    {
        let dir = format!("{}/{}", output_folder, BranchType::Base);
        match fs::create_dir(&dir) {
            Ok(_) => (),
            Err(_) => return Err(format!("❌ Failed to create directory: {}", dir).into()),
        }
    }
    {
        let dir = format!("{}/{}", output_folder, BranchType::Target);
        match fs::create_dir(&dir) {
            Ok(_) => (),
            Err(_) => return Err(format!("❌ Failed to create directory: {}", dir).into()),
        }
    }
    Ok(())
}

// Give new name for duplicate names in Vec<ArgoResource>
fn unique_names(apps: Vec<ArgoResource>, branch: &Branch) -> Vec<ArgoResource> {
    let mut duplicate_names: HashMap<String, Vec<ArgoResource>> = HashMap::new();
    apps.into_iter().for_each(|a| {
        duplicate_names.entry(a.name.clone()).or_default().push(a);
    });
    let mut new_vec: Vec<ArgoResource> = vec![];
    let mut duplicate_counter = 0;
    for (name, apps) in duplicate_names {
        if apps.len() > 1 {
            duplicate_counter += 1;
            debug!(
                "Found {} duplicate applications with name: '{}'",
                apps.len(),
                name
            );
            let mut sorted_apps = apps.clone();
            sorted_apps.sort_by_key(|a| a.as_string().unwrap_or_default());
            for (i, app) in sorted_apps.into_iter().enumerate() {
                let new_name = format!("{}-{}", name, i + 1);
                let mut new_app = app.clone();
                new_app.name.clone_from(&new_name);
                new_app.yaml["metadata"]["name"] = serde_yaml::Value::String(new_name);
                new_vec.push(new_app);
            }
        } else {
            new_vec.push(apps[0].clone());
        }
    }
    if duplicate_counter > 0 {
        info!(
            "🔍 Found {} duplicate applications names for branch: {}. Suffixing with -1, -2, -3, etc.",
            duplicate_counter, branch.name
        );
    }
    new_vec
}

fn cleanup_cluster(tool: ClusterTool, cluster_name: &str) {
    match tool {
        ClusterTool::Kind if kind::cluster_exists(cluster_name) => {
            info!("🧼 Cleaning up...");
            kind::delete_cluster(cluster_name, true)
        }
        ClusterTool::Minikube if minikube::cluster_exists() => {
            info!("🧼 Cleaning up...");
            minikube::delete_cluster(true)
        }
        _ => debug!("🧼 No cluster to clean up"),
    }
}

pub fn create_namespace(namespace: &str) -> Result<(), Box<dyn Error>> {
    run_command(&format!("kubectl create ns {}", namespace)).map_err(|e| {
        error!("❌ Failed to create namespace '{}'", namespace);
        CommandError::new(e)
    })?;
    debug!("🤖 Namespace '{}' created successfully", namespace);
    Ok(())
}

fn apply_manifest(file_name: &str) -> Result<CommandOutput, CommandOutput> {
    run_command(&format!("kubectl apply -f {}", file_name)).inspect_err(|_e| {
        error!("❌ Failed to apply manifest: {}", file_name);
    })
}

fn apply_folder(folder_name: &str) -> Result<u64, String> {
    if !PathBuf::from(folder_name).is_dir() {
        return Err(format!("{} is not a directory", folder_name));
    }
    let mut count = 0;
    if let Ok(entries) = fs::read_dir(folder_name) {
        for entry in entries.flatten() {
            let path = entry.path();
            let file_name = path.to_str().unwrap();
            if file_name.ends_with(".yaml") || file_name.ends_with(".yml") {
                match apply_manifest(file_name) {
                    Ok(_) => count += 1,
                    Err(e) => return Err(e.stderr),
                }
            }
        }
    }
    Ok(count)
}

pub fn applications_to_string(applications: Vec<ArgoResource>) -> Result<String, Box<dyn Error>> {
    let output = applications
        .iter()
        .map(|a| {
            a.as_string().inspect_err(|e| {
                error!(
                    "❌ Failed to convert application '{}' (path: {}) to valid YAML: {}",
                    a.name, a.file_name, e
                );
            })
        })
        .collect::<Result<Vec<String>, Box<dyn Error>>>()?;
    Ok(output.join("---\n"))
}
