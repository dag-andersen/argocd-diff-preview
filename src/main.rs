use crate::utils::{check_if_folder_exists, create_folder_if_not_exists};
use branch::{Branch, BranchType};
use log::{debug, error, info};
use parsing::applications_to_string;
use regex::Regex;
use selector::Selector;
use std::fs;
use std::path::PathBuf;
use std::{error::Error, io::Write};
use structopt::StructOpt;
use utils::{run_command_from_list, CommandOutput};
mod argo_resource;
mod argocd;
mod branch;
mod diff;
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
    #[structopt(long, env)]
    local_cluster_tool: Option<String>,

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
}

#[derive(Debug)]
enum ClusterTool {
    Kind,
    Minikube,
}

#[tokio::main]
async fn main() -> Result<(), ()> {
    match run().await {
        Ok(_) => Ok(()),
        Err(e) => {
            let opt = Opt::from_args();
            error!("❌ {}", e);
            cleanup_cluster(
                get_cluster_tool(&opt.local_cluster_tool).unwrap(),
                &opt.cluster_name,
                true,
            );
            Err(())
        }
    }
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
    let argocd_version = opt
        .argocd_chart_version
        .as_deref()
        .filter(|f| !f.trim().is_empty());
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

    // select local cluster tool
    let cluster_tool = get_cluster_tool(&opt.local_cluster_tool)?;

    let repo_regex = Regex::new(r"^[a-zA-Z0-9-]+/[a-zA-Z0-9-]+$").unwrap();
    if !repo_regex.is_match(repo) {
        error!("❌ Invalid repository format. Please use OWNER/REPO");
        return Err("Invalid repository format".into());
    }

    info!("✨ Running with:");
    info!("✨ - local-cluster-tool: {:?}", cluster_tool);
    info!("✨ - base-branch: {}", base_branch_name);
    info!("✨ - target-branch: {}", target_branch_name);
    info!("✨ - secrets-folder: {}", secrets_folder);
    info!("✨ - output-folder: {}", output_folder);
    info!("✨ - repo: {}", repo);
    info!("✨ - timeout: {} seconds", timeout);
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

    let cluster_name = opt.cluster_name;

    // remove .git from repo
    let repo = repo.trim_end_matches(".git");
    let (base_apps, target_apps) = parsing::get_applications_for_both_branches(
        &base_branch,
        &target_branch,
        &file_regex,
        &selector,
        &files_changed,
        repo,
        opt.ignore_invalid_watch_pattern,
    )?;

    let found_base_apps = !base_apps.is_empty();
    let found_target_apps = !target_apps.is_empty();

    if !found_base_apps && !found_target_apps {
        info!("👀 Nothing to compare");
        info!("👀 If this doesn't seem right, try running the tool with '--debug' to get more details about what is happening");
        no_apps_found::write_message(output_folder, &selector, &files_changed)?;
        info!("🎉 Done in {} seconds", start.elapsed().as_secs());
        return Ok(());
    }

    fs::write(base_branch.app_file(), applications_to_string(base_apps))?;
    fs::write(
        target_branch.app_file(),
        applications_to_string(target_apps),
    )?;

    match cluster_tool {
        ClusterTool::Kind => kind::create_cluster(&cluster_name)?,
        ClusterTool::Minikube => minikube::create_cluster()?,
    }

    argocd::create_namespace()?;

    create_folder_if_not_exists(secrets_folder)?;
    match apply_folder(secrets_folder) {
        Ok(count) if count > 0 => info!("🤫 Applied {} secrets", count),
        Ok(_) => info!("🤷 No secrets found in {}", secrets_folder),
        Err(e) => {
            error!("❌ Failed to apply secrets");
            return Err(e);
        }
    }

    argocd::install_argo_cd(argocd::ArgoCDOptions {
        version: argocd_version,
        debug: opt.debug,
    })
    .await?;

    // Cleanup output folder
    clean_output_folder(output_folder)?;

    // Extract resources from Argo CD
    if found_base_apps {
        extract::get_resources(&base_branch, timeout, output_folder).await?;
        if found_target_apps {
            extract::delete_applications().await?;
            tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
        }
    }
    if found_target_apps {
        extract::get_resources(&target_branch, timeout, output_folder).await?;
    }

    // Delete cluster
    match cluster_tool {
        ClusterTool::Kind => kind::delete_cluster(&cluster_name, false),
        ClusterTool::Minikube => minikube::delete_cluster(false),
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

fn get_cluster_tool(tool: &Option<String>) -> Result<ClusterTool, Box<dyn Error>> {
    let tool = match tool {
        Some(t) if t == "kind" => ClusterTool::Kind,
        Some(t) if t == "minikube" => ClusterTool::Minikube,
        _ if kind::is_installed() => ClusterTool::Kind,
        _ if minikube::is_installed() => ClusterTool::Minikube,
        _ => {
            error!("❌ No local cluster tool found. Please install kind or minikube");
            return Err("No local cluster tool found".into());
        }
    };
    Ok(tool)
}

fn cleanup_cluster(tool: ClusterTool, cluster_name: &str, wait: bool) {
    info!("🧼 Cleaning up...");
    match tool {
        ClusterTool::Kind if kind::cluster_exists(cluster_name) => {
            kind::delete_cluster(cluster_name, wait)
        }
        ClusterTool::Minikube if minikube::cluster_exists() => minikube::delete_cluster(wait),
        _ => debug!("🧼 No cluster to clean up"),
    }
}

fn apply_manifest(file_name: &str) -> Result<CommandOutput, CommandOutput> {
    run_command_from_list(vec!["kubectl", "apply", "-f", file_name], None).map_err(|e| {
        error!("❌ Failed to apply manifest: {}", file_name);
        e
    })
}

fn apply_folder(folder_name: &str) -> Result<u64, Box<dyn Error>> {
    if !PathBuf::from(folder_name).is_dir() {
        return Err(format!("{} is not a directory", folder_name).into());
    }
    let mut count = 0;
    if let Ok(entries) = fs::read_dir(folder_name) {
        for entry in entries.flatten() {
            let path = entry.path();
            let file_name = path.to_str().unwrap();
            if file_name.ends_with(".yaml") || file_name.ends_with(".yml") {
                match apply_manifest(file_name) {
                    Ok(_) => count += 1,
                    Err(e) => return Err(e.stderr.into()),
                }
            }
        }
    }
    Ok(count)
}
