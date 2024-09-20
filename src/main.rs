use crate::utils::{check_if_folder_exists, create_folder_if_not_exists, run_command};
use log::{debug, error, info};
use regex::Regex;
use std::fs;
use std::path::PathBuf;
use std::{
    error::Error,
    io::Write,
    process::{Command, Output},
};
use structopt::StructOpt;
mod argocd;
mod diff;
mod extract;
mod kind;
mod minikube;
mod parsing;
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

    /// Argo CD version. Default: stable
    #[structopt(long, env)]
    argocd_version: Option<String>,

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

    /// kustomize.buildOptions for argocd-cm ConfigMap
    #[structopt(long, env)]
    kustomize_build_options: Option<String>,

    /// Label selector to filter on, supports '=', '==', and '!='. (e.g. -l key1=value1,key2=value2).
    #[structopt(long, short = "l", env)]
    selector: Option<String>,
}

#[derive(Debug)]
enum ClusterTool {
    Kind,
    Minikube,
}

enum Branch {
    Base,
    Target,
}

impl std::fmt::Display for Branch {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            Branch::Base => write!(f, "base"),
            Branch::Target => write!(f, "target"),
        }
    }
}

enum Operator {
    Eq,
    Ne,
}

struct Selector {
    key: String,
    value: String,
    operator: Operator,
}

impl std::fmt::Display for Selector {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            Selector {
                key,
                value,
                operator,
            } => match operator {
                Operator::Eq => write!(f, "{}={}", key, value),
                Operator::Ne => write!(f, "{}!={}", key, value),
            },
        }
    }
}

fn apps_file(branch: &Branch) -> &'static str {
    match branch {
        Branch::Base => "apps_base_branch.yaml",
        Branch::Target => "apps_target_branch.yaml",
    }
}

const BASE_BRANCH_FOLDER: &str = "base-branch";
const TARGET_BRANCH_FOLDER: &str = "target-branch";
const CLUSTER_NAME: &str = "argocd-diff-preview";

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
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
        .map(|f| Regex::new(&f).unwrap());

    let base_branch_name = opt.base_branch;
    let target_branch_name = opt.target_branch;
    let repo = opt.repo;
    let diff_ignore = opt.diff_ignore.filter(|f| !f.trim().is_empty());
    let timeout = opt.timeout;
    let output_folder = opt.output_folder.as_str();
    let secrets_folder = opt.secrets_folder.as_str();
    let line_count = opt.line_count;
    let argocd_version = opt
        .argocd_version
        .as_deref()
        .filter(|f| !f.trim().is_empty());
    let kube_build_options = opt
        .kustomize_build_options
        .as_deref()
        .filter(|f| !f.trim().is_empty());
    let max_diff_length = opt.max_diff_length;

    // select local cluster tool
    let tool = match opt.local_cluster_tool {
        Some(t) if t == "kind" => ClusterTool::Kind,
        Some(t) if t == "minikube" => ClusterTool::Minikube,
        _ if kind::is_installed().await => ClusterTool::Kind,
        _ if minikube::is_installed().await => ClusterTool::Minikube,
        _ => {
            error!("❌ No local cluster tool found. Please install kind or minikube");
            panic!("No local cluster tool found")
        }
    };

    let repo_regex = Regex::new(r"^[a-zA-Z0-9-]+/[a-zA-Z0-9-]+$").unwrap();
    if !repo_regex.is_match(&repo) {
        error!("❌ Invalid repository format. Please use OWNER/REPO");
        panic!("Invalid repository format");
    }

    info!("✨ Running with:");
    info!("✨ - local-cluster-tool: {:?}", tool);
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
    if let Some(a) = kube_build_options {
        info!("✨ - kube-build-options: {}", a);
    }

    // label selectors can be fined in the following format: key1==value1,key2=value2,key3!=value3
    let selector = opt.selector.map(|s| {
        let labels: Vec<Selector> = s
            .split(",")
            .filter(|l| !l.trim().is_empty())
            .map(|l| {
                let not_equal = l.split("!=").collect::<Vec<&str>>();
                let equal_double = l.split("==").collect::<Vec<&str>>();
                let equal_single = l.split("=").collect::<Vec<&str>>();
                let selector = match (not_equal.len(), equal_double.len(), equal_single.len()) {
                    (2, _, _) => Selector {
                        key: not_equal[0].trim().to_string(),
                        value: not_equal[1].trim().to_string(),
                        operator: Operator::Ne,
                    },
                    (_, 2, _) => Selector {
                        key: equal_double[0].trim().to_string(),
                        value: equal_double[1].trim().to_string(),
                        operator: Operator::Eq,
                    },
                    (_, _, 2) => Selector {
                        key: equal_single[0].trim().to_string(),
                        value: equal_single[1].trim().to_string(),
                        operator: Operator::Eq,
                    },
                    _ => {
                        error!("❌ Invalid label selector format: {}", l);
                        panic!("Invalid label selector format");
                    }
                };
                if selector.key.is_empty()
                    || selector.key.contains("!")
                    || selector.key.contains("=")
                    || selector.value.is_empty()
                    || selector.value.contains("!")
                    || selector.value.contains("=")
                {
                    error!("❌ Invalid label selector format: {}", l);
                    panic!("Invalid label selector format");
                }
                selector
            })
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

    if !check_if_folder_exists(&BASE_BRANCH_FOLDER) {
        error!(
            "❌ Base branch folder does not exist: {}",
            BASE_BRANCH_FOLDER
        );
        panic!("Base branch folder does not exist");
    }

    if !check_if_folder_exists(&TARGET_BRANCH_FOLDER) {
        error!(
            "❌ Target branch folder does not exist: {}",
            TARGET_BRANCH_FOLDER
        );
        panic!("Target branch folder does not exist");
    }

    let cluster_name = CLUSTER_NAME;

    match tool {
        ClusterTool::Kind => kind::create_cluster(&cluster_name).await?,
        ClusterTool::Minikube => minikube::create_cluster().await?,
    }
    argocd::install_argo_cd(argocd::ArgoCDOptions {
        version: argocd_version,
        kube_build_options,
    })
    .await?;

    create_folder_if_not_exists(secrets_folder);
    match apply_folder(secrets_folder) {
        Ok(count) if count > 0 => info!("🤫 Applied {} secrets", count),
        Ok(_) => info!("🤷 No secrets found in {}", secrets_folder),
        Err(e) => {
            error!("❌ Failed to apply secrets");
            panic!("error: {}", e)
        }
    }

    // remove .git from repo
    let repo = repo.trim_end_matches(".git");
    let base_apps = parsing::get_applications_as_string(
        BASE_BRANCH_FOLDER,
        &base_branch_name,
        &file_regex,
        &selector,
        repo,
    )
    .await?;
    let target_apps = parsing::get_applications_as_string(
        TARGET_BRANCH_FOLDER,
        &target_branch_name,
        &file_regex,
        &selector,
        repo,
    )
    .await?;

    fs::write(apps_file(&Branch::Base), base_apps)?;
    fs::write(apps_file(&Branch::Target), &target_apps)?;

    // Cleanup
    clean_output_folder(output_folder);

    extract::get_resources(&Branch::Base, timeout, output_folder).await?;
    extract::delete_applications().await;
    tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    extract::get_resources(&Branch::Target, timeout, output_folder).await?;

    match tool {
        ClusterTool::Kind => kind::delete_cluster(&cluster_name),
        ClusterTool::Minikube => minikube::delete_cluster(),
    }

    diff::generate_diff(
        output_folder,
        &base_branch_name,
        &target_branch_name,
        diff_ignore,
        line_count,
        max_diff_length,
    )
    .await?;

    info!("🎉 Done in {} seconds", start.elapsed().as_secs());

    Ok(())
}

fn clean_output_folder(output_folder: &str) {
    create_folder_if_not_exists(output_folder);
    fs::remove_dir_all(format!("{}/{}", output_folder, Branch::Base)).unwrap_or_default();
    fs::remove_dir_all(format!("{}/{}", output_folder, Branch::Target)).unwrap_or_default();
    fs::create_dir(format!("{}/{}", output_folder, Branch::Base))
        .expect("Unable to create directory");
    fs::create_dir(format!("{}/{}", output_folder, Branch::Target))
        .expect("Unable to create directory");
}

fn apply_manifest(file_name: &str) -> Result<Output, Output> {
    let output = Command::new("kubectl")
        .arg("apply")
        .arg("-f")
        .arg(file_name)
        .output()
        .expect(format!("failed to apply manifest: {}", file_name).as_str());
    match output.status.success() {
        true => Ok(output),
        false => Err(output),
    }
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
                    Err(e) => return Err(String::from_utf8_lossy(&e.stderr).to_string()),
                }
            }
        }
    }
    Ok(count)
}
