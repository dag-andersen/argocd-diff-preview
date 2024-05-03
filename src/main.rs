use regex::Regex;
use serde_yaml::Mapping;
use std::collections::HashSet;
use std::fs;
use std::{
    collections::BTreeMap,
    error::Error,
    io::{BufRead, Write},
    process::{Command, Output},
};

use log::{debug, error, info};
use std::path::PathBuf;
use structopt::StructOpt;

use crate::utils::{
    check_if_folder_exists, create_folder_if_not_exists, run_command, run_command_output_to_file,
    write_to_file,
};
mod argocd;
mod diff;
mod kind;
mod minikube;
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

    /// Base branch name
    #[structopt(short, long, default_value = "main", env)]
    base_branch: String,

    /// Base branch folder
    #[structopt(long, env, default_value = "base-branch")]
    base_branch_folder: String,

    /// Target branch name
    #[structopt(short, long, env)]
    target_branch: String,

    /// Target branch folder
    #[structopt(long, env, default_value = "target-branch")]
    target_branch_folder: String,

    /// Git repository URL
    #[structopt(short = "g", long = "git-repo", env = "GIT_REPO")]
    git_repository: String,

    /// Output folder where the diff will be saved
    #[structopt(short, long, default_value = "./output", env)]
    output_folder: String,

    /// Secrets folder where the secrets are read from
    #[structopt(short, long, default_value = "./secrets", env)]
    secrets_folder: String,

    /// Local cluster tool. Options: kind, minikube, auto. Default: Auto
    #[structopt(long, env)]
    local_cluster_tool: Option<String>,
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

fn apps_file(branch: &Branch) -> &'static str {
    match branch {
        Branch::Base => "apps_base_branch.yaml",
        Branch::Target => "apps_target_branch.yaml",
    }
}

static ERROR_MESSAGES: [&str; 6] = [
    "helm template .",
    "authentication required",
    "authentication failed",
    "path does not exist",
    "error converting YAML to JSON",
    "Unknown desc = `helm template .",
];

static TIMEOUT_MESSAGES: [&str; 2] = ["Client.Timeout", "failed to get git client for repo"];

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
    let base_branch_folder = opt.base_branch_folder;
    let target_branch_name = opt.target_branch;
    let target_branch_folder = opt.target_branch_folder;
    let repo = opt.git_repository;
    let diff_ignore = opt.diff_ignore;
    let timeout = opt.timeout;
    let output_folder = opt.output_folder.as_str();
    let secrets_folder = opt.secrets_folder.as_str();

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

    info!("✨ Running with:");
    info!("✨ - local-cluster-tool: {:?}", tool);
    info!("✨ - base-branch: {}", base_branch_name);
    info!("✨ - target-branch: {}", target_branch_name);
    info!("✨ - base-branch-folder: {}", base_branch_folder);
    info!("✨ - target-branch-folder: {}", target_branch_folder);
    info!("✨ - secrets-folder: {}", secrets_folder);
    info!("✨ - output-folder: {}", output_folder);
    info!("✨ - git-repo: {}", repo);
    info!("✨ - timeout: {}", timeout);
    if let Some(a) = file_regex.clone() {
        info!("✨ - file-regex: {}", a.as_str());
    }
    if let Some(a) = diff_ignore.clone() {
        info!("✨ - diff-ignore: {}", a);
    }

    if !check_if_folder_exists(&base_branch_folder) {
        error!(
            "❌ Base branch folder does not exist: {}",
            base_branch_folder
        );
        panic!("Base branch folder does not exist");
    }

    if !check_if_folder_exists(&target_branch_folder) {
        error!(
            "❌ Target branch folder does not exist: {}",
            target_branch_folder
        );
        panic!("Target branch folder does not exist");
    }

    let cluster_name = "argocd-diff-preview";

    match tool {
        ClusterTool::Kind => kind::create_cluster(&cluster_name).await?,
        ClusterTool::Minikube => minikube::create_cluster().await?,
    }
    argocd::install_argo_cd().await?;

    create_folder_if_not_exists(secrets_folder);
    match apply_folder(secrets_folder) {
        Ok(count) if count > 0 => info!("🤫 Applied {} secrets", count),
        Ok(_) => info!("🤷‍♂️ No secrets found in {}", secrets_folder),
        Err(e) => {
            error!("❌ Failed to apply secrets");
            panic!("error: {}", e)
        }
    }

    // remove .git from repo
    let repo = repo.trim_end_matches(".git");
    let base_apps =
        parse_argocd_application(&base_branch_folder, &base_branch_name, &file_regex, &repo)
            .await?;
    write_to_file(&base_apps, apps_file(&Branch::Base));
    let target_apps = parse_argocd_application(
        &target_branch_folder,
        &target_branch_name,
        &file_regex,
        &repo,
    )
    .await?;
    write_to_file(&target_apps, apps_file(&Branch::Target));

    // Cleanup
    clean_output_folder(output_folder);

    get_resources(&Branch::Base, timeout, output_folder).await?;
    tokio::time::sleep(tokio::time::Duration::from_secs(10)).await;
    get_resources(&Branch::Target, timeout, output_folder).await?;

    match tool {
        ClusterTool::Kind => kind::delete_cluster(&cluster_name),
        ClusterTool::Minikube => minikube::delete_cluster(),
    }

    diff::generate_diff(
        output_folder,
        &base_branch_name,
        &target_branch_name,
        diff_ignore,
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

async fn get_resources(
    branch_type: &Branch,
    timeout: u64,
    output_folder: &str,
) -> Result<(), Box<dyn Error>> {
    info!("🌚 Getting resources for {}", branch_type);

    if fs::metadata(apps_file(branch_type)).unwrap().len() != 0 {
        match apply_manifest(&apps_file(branch_type)) {
            Ok(_) => (),
            Err(e) => panic!("error: {}", String::from_utf8_lossy(&e.stderr)),
        }
    }

    let mut set_of_processed_apps = HashSet::new();
    let mut set_of_failed_apps = BTreeMap::new();

    let start_time = std::time::Instant::now();

    loop {
        let output = run_command("kubectl get applications -n argocd -oyaml", None)
            .await
            .expect("failed to get applications");
        let applications: serde_yaml::Value =
            serde_yaml::from_str(&String::from_utf8_lossy(&output.stdout)).unwrap();

        let items = applications["items"].as_sequence().unwrap();
        if items.is_empty() {
            break;
        }

        let mut list_of_timed_out_apps = vec![];

        let mut apps_left = 0;

        for item in items {
            let name = item["metadata"]["name"].as_str().unwrap();
            if set_of_processed_apps.contains(name) {
                continue;
            }
            match item["status"]["sync"]["status"].as_str() {
                Some("OutOfSync") | Some("Synced") => {
                    debug!("Processing application: {}", name);
                    match run_command_output_to_file(
                        &format!("argocd app manifests {}", name),
                        &format!("{}/{}/{}", output_folder, branch_type, name),
                        false,
                    )
                    .await
                    {
                        Ok(_) => debug!("Processed application: {}", name),
                        Err(e) => error!("error: {}", String::from_utf8_lossy(&e.stderr)),
                    }
                    set_of_processed_apps.insert(name.to_string().clone());
                    continue;
                }
                Some("Unknown") => {
                    if let Some(conditions) = item["status"]["conditions"].as_sequence() {
                        for condition in conditions {
                            if let Some("ComparisonError") = condition["type"].as_str() {
                                match condition["message"].as_str() {
                                    Some(msg) if ERROR_MESSAGES.iter().any(|e| msg.contains(e)) => {
                                        set_of_failed_apps
                                            .insert(name.to_string().clone(), msg.to_string());
                                        continue;
                                    }
                                    Some(msg)
                                        if TIMEOUT_MESSAGES.iter().any(|e| msg.contains(e)) =>
                                    {
                                        list_of_timed_out_apps.push(name.to_string().clone());
                                    }
                                    _ => (),
                                }
                            }
                        }
                    }
                }
                _ => (),
            }
            apps_left += 1
        }

        if apps_left == 0 {
            break;
        }

        let time_elapsed = start_time.elapsed().as_secs();
        if time_elapsed > timeout as u64 {
            error!("❌ Timed out after {} seconds", timeout);
            error!(
                "❌ Processed {} applications, but {} applications still remain",
                set_of_processed_apps.len(),
                apps_left
            );
            return Err("Timed out".into());
        }

        if !set_of_failed_apps.is_empty() {
            for (name, msg) in &set_of_failed_apps {
                error!(
                    "❌ Failed to process application: {} with error: \n{}",
                    name, msg
                );
            }
            return Err("Failed to process applications".into());
        }

        if !list_of_timed_out_apps.is_empty() {
            info!(
                "💤 {} Applications timed out.",
                list_of_timed_out_apps.len(),
            );
            for app in &list_of_timed_out_apps {
                match run_command(&format!("argocd app get {} --refresh", app), None).await {
                    Ok(_) => info!("🔄 Refreshing application: {}", app),
                    Err(e) => error!(
                        "⚠️ Failed to refresh application: {} with {}",
                        app,
                        String::from_utf8_lossy(&e.stderr)
                    ),
                }
            }
        }

        info!(
            "⏳ Waiting for {} out of {} applications to become 'OutOfSync'. Retrying in 5 seconds. Timeout in {} seconds...",
            apps_left,
            items.len(),
            timeout - time_elapsed
        );

        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }

    info!(
        "🌚 Got all resources from {} applications for {}",
        set_of_processed_apps.len(),
        branch_type
    );

    match run_command(
        "kubectl delete applications.argoproj.io -n argocd --all",
        None,
    )
    .await
    {
        Ok(_) => (),
        Err(e) => error!("error: {}", String::from_utf8_lossy(&e.stderr)),
    }
    Ok(())
}

async fn parse_argocd_application(
    directory: &str,
    branch: &str,
    regex: &Option<Regex>,
    repo: &str,
) -> Result<String, Box<dyn Error>> {
    let applications = parse_yaml(directory, regex).await?;
    let output = patch_argocd_applications(applications, branch, repo).await?;
    Ok(output)
}

async fn parse_yaml(directory: &str, regex: &Option<Regex>) -> Result<Vec<String>, Box<dyn Error>> {
    use walkdir::WalkDir;

    info!("🤖 Fetching all files in dir: {}", directory);

    let yaml_files = WalkDir::new(directory)
        .into_iter()
        .filter_map(|e| e.ok())
        .filter(|e| e.path().is_file())
        .filter(|e| {
            e.path()
                .extension()
                .and_then(|s| s.to_str())
                .map(|s| s == "yaml" || s == "yml")
                .unwrap_or(false)
        })
        .map(|e| format!("{}", e.path().display()))
        .filter(|f| regex.is_none() || regex.as_ref().unwrap().is_match(&f));

    let k8s_resources: Vec<String> = yaml_files
        .flat_map(|f| {
            debug!("Found file: {}", f);
            let file = std::fs::File::open(f).unwrap();
            let reader = std::io::BufReader::new(file);
            let lines = reader.lines().map(|l| l.unwrap());

            // split list of strings by "---"
            let string_lists: Vec<String> = lines.fold(vec!["".to_string()], |mut acc, s| {
                if s == "---" {
                    acc.push("".to_string());
                } else {
                    let last = acc.len() - 1;
                    acc[last].push_str("\n");
                    acc[last].push_str(&s);
                }
                acc
            });
            string_lists
        })
        .collect::<Vec<String>>();

    match regex {
        Some(r) => info!(
            "🤖 Found {} k8s resources in files matching regex: {}",
            k8s_resources.len(),
            r.as_str()
        ),
        None => info!("🤖 Found {} k8s resources", k8s_resources.len()),
    }

    Ok(k8s_resources)
}

async fn patch_argocd_applications(
    mut yaml_chunks: Vec<String>,
    branch: &str,
    repo: &str,
) -> Result<String, Box<dyn Error>> {
    info!("🤖 Patching applications for branch: {}", branch);

    let clean_spec = |spec: &mut Mapping| {
        spec.remove("syncPolicy");
        spec["project"] = serde_yaml::Value::String("default".to_string());
        spec["destination"]["name"] = serde_yaml::Value::String("in-cluster".to_string());
        spec["destination"]["namespace"] = serde_yaml::Value::String("default".to_string());
    };

    let redirect_sources = |spec: &mut Mapping| {
        if spec.contains_key("source") {
            if spec["source"]["repoURL"]
                .as_str()
                .unwrap()
                .trim_end_matches(".git")
                == repo
            {
                spec["source"]["targetRevision"] = serde_yaml::Value::String(branch.to_string());
            }
        } else if spec.contains_key("sources") {
            for source in spec["sources"].as_sequence_mut().unwrap() {
                if source["repoURL"].as_str().unwrap().trim_end_matches(".git") == repo {
                    source["targetRevision"] = serde_yaml::Value::String(branch.to_string());
                }
            }
        }
    };

    let applications = yaml_chunks
        .iter_mut()
        .map(|r| {
            let resource: serde_yaml::Value = match serde_yaml::from_str(r) {
                Ok(r) => r,
                Err(e) => {
                    debug!("⚠️ Failed to parse resource with error: {}", e);
                    serde_yaml::Value::Null
                }
            };
            resource
        })
        .filter_map(|r| {
            r["kind"].as_str().map(|s| s.to_string()).and_then(|kind| {
                if kind == "Application" || kind == "ApplicationSet" {
                    Some((kind, r))
                } else {
                    None
                }
            })
        })
        .map(|(kind, mut r)| {
            // Clean up the spec
            clean_spec({
                if kind == "Application" {
                    r["spec"].as_mapping_mut().unwrap()
                } else {
                    r["spec"]["template"]["spec"].as_mapping_mut().unwrap()
                }
            });
            (kind, r)
        })
        .map(|(kind, mut r)| {
            // Redirect all applications to the branch
            redirect_sources({
                if kind == "Application" {
                    r["spec"].as_mapping_mut().unwrap()
                } else {
                    r["spec"]["template"]["spec"].as_mapping_mut().unwrap()
                }
            });
            debug!(
                "Collected resources from application: {:?}",
                r["metadata"]["name"].as_str().unwrap()
            );
            r
        })
        .collect::<Vec<serde_yaml::Value>>();

    info!(
        "🤖 Patching {} Argo CD Application[Sets] for branch: {}",
        applications.len(),
        branch
    );

    // convert back to string
    let mut output = String::new();
    for r in applications {
        let r = serde_yaml::to_string(&r).unwrap();
        output.push_str(&r);
        output.push_str("---\n");
    }

    Ok(output)
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
        for entry in entries {
            if let Ok(entry) = entry {
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
    }
    Ok(count)
}
