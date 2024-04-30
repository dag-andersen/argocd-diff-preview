#![allow(unused_imports)]
#![allow(dead_code)]
#![allow(unused_variables)]
use k8s_openapi::api::core::v1::{Namespace, Secret};
use kube::{Api, Client, Resource};
use regex::Regex;
use serde_yaml::Mapping;
use std::collections::HashSet;
use std::{
    collections::BTreeMap,
    error::Error,
    fs::{self, File},
    io::{BufRead, Write},
    process::{Command, Output, Stdio},
};

use log::{debug, error, info, log_enabled, warn, Level, LevelFilter};
use std::path::PathBuf;
use structopt::StructOpt;

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

static ERROR_MESSAGES: [&str; 6] = [
    "helm template .",
    "authentication required",
    "authentication failed",
    "path does not exist",
    "error converting YAML to JSON",
    "Unknown desc = `helm template .",
];

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

    info!("✨ Running with:");
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
        error!("❌ Base branch folder does not exist: {}", base_branch_folder);
        panic!("Base branch folder does not exist");
    }

    if !check_if_folder_exists(&target_branch_folder) {
        error!("❌ Target branch folder does not exist: {}", target_branch_folder);
        panic!("Target branch folder does not exist");
    }

    let cluster_name = "argocd-diff-preview";

    create_cluster(&cluster_name).await?;
    install_argo_cd().await?;

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
    let base_apps = parse_argocd_application(&base_branch_folder, &base_branch_name, &file_regex, &repo).await?;
    write_to_file(&base_apps, apps_file(&Branch::Base));
    let target_apps = parse_argocd_application(&target_branch_folder, &target_branch_name, &file_regex, &repo).await?;
    write_to_file(&target_apps, apps_file(&Branch::Target));

    // Cleanup
    clean_output_folder(output_folder);

    get_resources(&Branch::Base, timeout, output_folder).await?;
    tokio::time::sleep(tokio::time::Duration::from_secs(10)).await;
    get_resources(&Branch::Target, timeout, output_folder).await?;

    delete_cluster(&cluster_name);

    generate_diff(
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

fn apps_file(branch: &Branch) -> &'static str {
    match branch {
        Branch::Base => "apps_base_branch.yaml",
        Branch::Target => "apps_target_branch.yaml",
    }
}

async fn generate_diff(
    output_folder: &str,
    base_branch_name: &str,
    target_branch_name: &str,
    diff_ignore: Option<String>,
) -> Result<(), Box<dyn Error>> {
    info!("🔮 Generating diff between {} and {}", base_branch_name, target_branch_name);

    let list_of_patterns_to_ignore = match diff_ignore {
        Some(s) => s
            .split(",")
            .map(|s| format!("--ignore-matching-lines={}", s))
            .collect::<Vec<String>>()
            .join(" "),
        None => "".to_string(),
    };

    let parse_diff_output = |output: Result<Output, Output>| -> String {
        match output {
            Ok(o) => "No changes found".to_string(),
            Err(e) => String::from_utf8_lossy(&e.stdout).to_string(),
        }
    };

    let summary_as_string = parse_diff_output(
        run_command(
            &format!(
                "git --no-pager diff --compact-summary --no-index {} {} {}",
                list_of_patterns_to_ignore, Branch::Base, Branch::Target
            ),
            Some(output_folder),
        )
        .await,
    );

    let diff_as_string = parse_diff_output(
        run_command(
            &format!(
                "git --no-pager diff -U10 --no-index {} {} {}",
                list_of_patterns_to_ignore, Branch::Base, Branch::Target
            ),
            Some(output_folder),
        )
        .await,
    );

    let markdown = print_diff(&summary_as_string, &diff_as_string);

    let markdown_path = format!("{}/diff.md", output_folder);
    write_to_file(&markdown, &markdown_path);

    info!("🙏 Please check the {} file for differences", markdown_path);

    Ok(())
}

async fn get_resources(
    branch_type: &Branch,
    timeout: u64,
    output_folder: &str,
) -> Result<(), Box<dyn Error>> {
    info!("🌚 Getting resources for {}", branch_type);

    match apply_manifest(&apps_file(branch_type)) {
        Ok(_) => (),
        Err(e) => panic!("error: {}", String::from_utf8_lossy(&e.stderr)),
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
                                        if msg.contains("Client.Timeout")
                                            || msg
                                                .contains("failed to get git client for repo") =>
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
                    Ok(e) => info!("🔄 Refreshing application: {}", app),
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

    match run_command("kubectl delete applications.argoproj.io -n argocd --all", None).await {
        Ok(_) => (),
        Err(e) => error!("error: {}", String::from_utf8_lossy(&e.stderr)),
    }
    Ok(())
}

async fn install_argo_cd() -> Result<(), Box<dyn Error>> {
    info!("🦑 Installing Argo CD...");


    match run_command("kubectl create ns argocd", None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Failed to create namespace argocd");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    // Install Argo CD
    match run_command("kubectl -n argocd apply -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml", None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Failed to install Argo CD");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }
    info!("🦑 Waiting for Argo CD to start...");

    static ARGOCD_CMD_PARAMS_CM : &str = r#"
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/name: argocd-cmd-params-cm
    app.kubernetes.io/part-of: argocd
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  reposerver.git.request.timeout: "150s"
  reposerver.parallelism.limit: "300"
"#;
    // apply argocd-cmd-params-cm
    let mut child = Command::new("kubectl")
        .arg("apply")
        .arg("-f")
        .arg("-")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("failed to execute process");

    let child_stdin = child.stdin.as_mut().expect("Failed to open stdin");
    child_stdin.write_all(ARGOCD_CMD_PARAMS_CM.as_bytes()).expect("Failed to write to stdin");

    let output = child.wait_with_output().expect("Failed to read stdout");

    run_command(
        "kubectl -n argocd rollout restart deploy argocd-repo-server",
        None,
    )
    .await
    .expect("failed to restart argocd-repo-server");
    run_command(
        "kubectl -n argocd rollout status deployment/argocd-repo-server --timeout=60s",
        None,
    )
    .await
    .expect("failed to wait for argocd-repo-server");

    info!("🦑 Logging in to Argo CD through CLI...");
    debug!("Port-forwarding Argo CD server...");

    // port-forward Argo CD server
    let child = Command::new("kubectl")
        .arg("-n")
        .arg("argocd")
        .arg("port-forward")
        .arg("service/argocd-server")
        .arg("8080:443")
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect("failed to execute process");

    let client = Client::try_default().await?;
    debug!("password:");
    let secret: Api<Secret> = Api::namespaced(client, "argocd");
    let secret_name = "argocd-initial-admin-secret";
    let secret = secret.get(secret_name).await?.clone();
    let data = secret.data.unwrap();
    let password_encoded = data.get("password").unwrap();
    let password_decoded = String::from_utf8_lossy(&password_encoded.0);
    debug!("- {}", password_decoded);

    // sleep for 5 seconds
    tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;

    // log into Argo CD

    run_command(
        &format!(
            "argocd login localhost:8080 --insecure --username admin --password {}",
            password_decoded
        ),
        None,
    )
    .await
    .expect("failed to login to argocd");

    let output = run_command("argocd app list", None)
        .await
        .expect("failed to list argocd apps");
    debug!(
        "argocd app list output: \n{}",
        String::from_utf8_lossy(&output.stdout)
    );

    info!("🦑 Argo CD installed successfully");
    Ok(())
}

async fn create_cluster(cluster_name: &str) -> Result<(), Box<dyn Error>> {
    // check if docker is running
    match run_command("docker ps", None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Docker is not running");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    info!("🚀 Creating cluster...");
    let output = match Command::new("kind")
        .arg("delete")
        .arg("cluster")
        .arg("--name")
        .arg(cluster_name)
        .output()
    {
        Ok(o) => o,
        Err(e) => {
            panic!("error: {}", e)
        }
    };

    match run_command(
        &format!("kind create cluster --name {}", cluster_name),
        None,
    )
    .await
    {
        Ok(_) => {
            info!("🚀 Cluster created successfully");
            Ok(())
        }
        Err(e) => {
            error!("❌ Failed to Create cluster");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }
}

fn delete_cluster(cluster_name: &str) {
    info!("💥 Deleting cluster...");
    spawn_command(
        &format!("kind delete cluster --name {}", cluster_name),
        None,
    );
}

async fn run_command_output_to_file(
    command: &str,
    file_name: &str,
    create_folder: bool,
) -> Result<Output, Output> {
    let output = run_command(command, None).await?;
    save_to_file(&output.stdout, file_name, create_folder)
        .await
        .expect("failed to save output to file");
    Ok(output)
}

async fn save_to_file(
    s: &Vec<u8>,
    file_name: &str,
    create_folder: bool,
) -> Result<(), Box<dyn Error>> {
    if create_folder {
        let path = PathBuf::from(file_name);
        let parent = path.parent().unwrap();
        if !parent.is_dir() {
            fs::create_dir_all(parent).expect("Unable to create directory");
        }
    }
    fs::remove_file(file_name).unwrap_or_default();
    let mut f = File::create_new(file_name).expect("Unable to create file");
    f.write_all(s).expect("Unable to write file");

    Ok(())
}

async fn run_command(command: &str, current_dir: Option<&str>) -> Result<Output, Output> {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    let output = Command::new(args[0])
        .args(&args[1..])
        .current_dir(current_dir.unwrap_or("."))
        .output()
        .expect(format!("Failed to execute command: {}", command).as_str());

    if !output.status.success() {
        return Err(output);
    }

    Ok(output)
}

fn spawn_command(command: &str, current_dir: Option<&str>) {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    Command::new(args[0])
        .args(&args[1..])
        .current_dir(current_dir.unwrap_or("."))
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect(format!("Failed to execute command: {}", command).as_str());
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
    mut applications: Vec<String>,
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

    let filtered_resources = applications
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
        filtered_resources.len(),
        branch
    );

    // convert back to string
    let mut output = String::new();
    for r in filtered_resources {
        let r = serde_yaml::to_string(&r).unwrap();
        output.push_str(&r);
        output.push_str("---\n");
    }

    Ok(output)
}

fn write_to_file(s: &str, file_name: &str) {
    fs::remove_file(file_name).unwrap_or_default();
    let mut f = File::create_new(file_name).expect("Unable to create file");
    f.write_all(s.as_bytes()).expect("Unable to write file");
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

// TODO: delete using "delete applications --all"
fn delete_manifest(file_name: &str) -> Result<Output, Output> {
    let output = Command::new("kubectl")
        .arg("delete")
        .arg("-f")
        .arg(file_name)
        .output()
        .expect(format!("failed to delete manifest: {}", file_name).as_str());
    match output.status.success() {
        true => Ok(output),
        false => Err(output),
    }
}

fn create_folder_if_not_exists(folder_name: &str) {
    if !PathBuf::from(folder_name).is_dir() {
        fs::create_dir(folder_name).expect("Unable to create directory");
    }
}

fn check_if_folder_exists(folder_name: &str) -> bool {
    PathBuf::from(folder_name).is_dir()
}

fn print_diff(summary: &str, diff: &str) -> String {
    let markdown = r#"
## Argo CD Diff Preview

Summary:
```bash
%summary%
```

<details>
<summary>Diff:</summary>
<br>

```diff
%diff%
```

</details>
"#;

    markdown
        .replace("%summary%", summary)
        .replace("%diff%", diff)
}
