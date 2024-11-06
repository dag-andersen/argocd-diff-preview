use crate::{filter_apps::filter, Operator, Selector};
use log::{debug, error, info, warn};
use regex::Regex;
use serde_yaml::{Mapping, Value};
use std::{error::Error, io::BufRead};

struct K8sResource {
    file_name: String,
    yaml: serde_yaml::Value,
}

pub struct Application {
    pub file_name: String,
    pub yaml: serde_yaml::Value,
    pub kind: ApplicationKind,
    pub name: String,
}

impl std::fmt::Display for Application {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", serde_yaml::to_string(&self.yaml).unwrap())
    }
}

impl PartialEq for Application {
    fn eq(&self, other: &Self) -> bool {
        self.yaml == other.yaml
    }
}

enum ApplicationKind {
    Application,
    ApplicationSet,
}

pub struct GetApplicationOptions<'a> {
    pub directory: &'a str,
    pub branch: &'a str,
}

pub async fn get_applications_for_both_branches<'a>(
    base_branch: GetApplicationOptions<'a>,
    target_branch: GetApplicationOptions<'a>,
    regex: &Option<Regex>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    repo: &str,
    ignore_invalid_watch_pattern: bool,
) -> Result<(Vec<Application>, Vec<Application>), Box<dyn Error>> {
    let base_apps = get_applications(
        base_branch.directory,
        base_branch.branch,
        regex,
        selector,
        files_changed,
        repo,
        ignore_invalid_watch_pattern,
    )
    .await?;
    let target_apps = get_applications(
        target_branch.directory,
        target_branch.branch,
        regex,
        selector,
        files_changed,
        repo,
        ignore_invalid_watch_pattern,
    )
    .await?;

    let duplicate_yaml = base_apps
        .iter()
        .filter(|a| target_apps.iter().any(|b| a.name == b.name))
        .filter(|a| {
            target_apps.iter().any(|b| {
                let equal = a.yaml == b.yaml;
                if equal {
                    debug!(
                        "Skipping application '{}' because it has not changed",
                        a.name
                    )
                }
                equal
            })
        })
        .map(|a| a.yaml.clone())
        .collect::<Vec<Value>>();

    if duplicate_yaml.is_empty() {
        Ok((base_apps, target_apps))
    } else {
        // remove duplicates
        let base_apps = base_apps
            .into_iter()
            .filter(|a| !duplicate_yaml.contains(&a.yaml))
            .collect();
        let target_apps = target_apps
            .into_iter()
            .filter(|a| !duplicate_yaml.contains(&a.yaml))
            .collect();

        Ok((base_apps, target_apps))
    }
}

pub async fn get_applications(
    directory: &str,
    branch: &str,
    regex: &Option<Regex>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    repo: &str,
    ignore_invalid_watch_pattern: bool,
) -> Result<Vec<Application>, Box<dyn Error>> {
    let yaml_files = get_yaml_files(directory, regex).await;
    let k8s_resources = parse_yaml(directory, yaml_files).await;
    let applications = from_resource_to_application(
        k8s_resources,
        selector,
        files_changed,
        ignore_invalid_watch_pattern,
    );
    if !applications.is_empty() {
        return patch_applications(applications, branch, repo).await;
    }
    Ok(applications)
}

async fn get_yaml_files(directory: &str, regex: &Option<Regex>) -> Vec<String> {
    use walkdir::WalkDir;

    info!("🤖 Fetching all files in dir: {}", directory);

    let yaml_files: Vec<String> = WalkDir::new(directory)
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
        .map(|e| {
            format!(
                "{}",
                e.path()
                    .iter()
                    .skip(1)
                    .collect::<std::path::PathBuf>()
                    .display()
            )
        })
        .filter(|f| regex.is_none() || regex.as_ref().unwrap().is_match(f))
        .collect();

    match regex {
        Some(r) => debug!(
            "🤖 Found {} yaml files in dir '{}' matching regex: {}",
            yaml_files.len(),
            directory,
            r.as_str()
        ),
        None => debug!(
            "🤖 Found {} yaml files in dir '{}'",
            yaml_files.len(),
            directory
        ),
    }

    yaml_files
}

async fn parse_yaml(directory: &str, files: Vec<String>) -> Vec<K8sResource> {
    files.iter()
        .flat_map(|f| {
            debug!("In dir '{}' found yaml file: {}", directory, f);
            let file = std::fs::File::open(format!("{}/{}",directory,f)).unwrap();
            let reader = std::io::BufReader::new(file);
            let lines = reader.lines().map(|l| l.unwrap());

            let mut raw_yaml_chunks: Vec<String> = lines.fold(vec!["".to_string()], |mut acc, s| {
                if s == "---" {
                    acc.push("".to_string());
                } else {
                    let last = acc.len() - 1;
                    acc[last].push('\n');
                    acc[last].push_str(&s);
                }
                acc
            });
            let yaml_vec: Vec<K8sResource> = raw_yaml_chunks.iter_mut().enumerate().map(|(i,r)| {
                let yaml = match serde_yaml::from_str(r) {
                    Ok(r) => r,
                    Err(e) => {
                        debug!("⚠️ Failed to parse element number {}, in file '{}', with error: '{}'", i+1, f, e);
                        serde_yaml::Value::Null
                    }
                };
                K8sResource {
                    file_name: f.clone(),
                    yaml,
                }
            }).collect();
            yaml_vec
        })
        .collect()
}

async fn patch_applications(
    applications: Vec<Application>,
    branch: &str,
    repo: &str,
) -> Result<Vec<Application>, Box<dyn Error>> {
    info!("🤖 Patching applications for branch: {}", branch);

    let point_destination_to_in_cluster = |spec: &mut Mapping| {
        if spec.contains_key("destination") {
            spec["destination"]["name"] = serde_yaml::Value::String("in-cluster".to_string());
            spec["destination"]
                .as_mapping_mut()
                .map(|a| a.remove("server"));
        }
    };

    let set_project_to_default =
        |spec: &mut Mapping| spec["project"] = serde_yaml::Value::String("default".to_string());

    let remove_sync_policy = |spec: &mut Mapping| spec.remove("syncPolicy");

    let redirect_sources = |spec: &mut Mapping, file: &str| {
        if spec.contains_key("source") {
            if spec["source"]["chart"].as_str().is_some() {
                return;
            }
            match spec["source"]["repoURL"].as_str() {
                Some(url) if url.to_lowercase().contains(repo.to_lowercase()) => {
                    spec["source"]["targetRevision"] = serde_yaml::Value::String(branch.to_string())
                }
                _ => debug!("Found no 'repoURL' under spec.source in file: {}", file),
            }
        } else if spec.contains_key("sources") {
            if let Some(sources) = spec["sources"].as_sequence_mut() {
                for source in sources {
                    if source["chart"].as_str().is_some() {
                        continue;
                    }
                    match source["repoURL"].as_str() {
                        Some(url) if url.to_lowercase().contains(repo.to_lowercase()) => {
                            source["targetRevision"] =
                                serde_yaml::Value::String(branch.to_string());
                        }
                        _ => debug!("Found no 'repoURL' under spec.sources[] in file: {}", file),
                    }
                }
            }
        }
    };

    let applications: Vec<Application> = applications
        .into_iter()
        .map(|mut a| {
            // Update namesapce
            a.yaml["metadata"]["namespace"] = serde_yaml::Value::String("argocd".to_string());
            a
        })
        .filter_map(|mut a| {
            // Clean up the spec
            let spec = match a.kind {
                ApplicationKind::Application => a.yaml["spec"].as_mapping_mut()?,
                ApplicationKind::ApplicationSet => {
                    a.yaml["spec"]["template"]["spec"].as_mapping_mut()?
                }
            };
            remove_sync_policy(spec);
            set_project_to_default(spec);
            point_destination_to_in_cluster(spec);
            redirect_sources(spec, &a.file_name);
            debug!(
                "Collected resources from application: {:?} in file: {}",
                a.name, a.file_name
            );
            Some(a)
        })
        .collect();

    info!(
        "🤖 Patching {} Argo CD Application[Sets] for branch: {}",
        applications.len(),
        branch
    );

    Ok(applications)
}

fn from_resource_to_application(
    k8s_resources: Vec<K8sResource>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    ignore_invalid_watch_pattern: bool,
) -> Vec<Application> {
    let apps: Vec<Application> = k8s_resources
        .into_iter()
        .filter_map(|r| {
            let kind =
                r.yaml["kind"]
                    .as_str()
                    .map(|s| s.to_string())
                    .and_then(|kind| match kind.as_str() {
                        "Application" => Some(ApplicationKind::Application),
                        "ApplicationSet" => Some(ApplicationKind::ApplicationSet),
                        _ => None,
                    })?;

            let name = r.yaml["metadata"]["name"]
                .as_str()
                .unwrap_or("unknown")
                .to_string();

            Some(Application {
                kind,
                file_name: r.file_name,
                name,
                yaml: r.yaml,
            })
        })
        .collect();

    match (selector, files_changed) {
        (Some(s), Some(f)) => info!(
            "🤖 Will only run on Applications that match '{}' and watch these files: '{}'",
            s.iter()
                .map(|s| s.to_string())
                .collect::<Vec<String>>()
                .join(","),
            f.join("`, `")
        ),
        (Some(s), None) => info!(
            "🤖 Will only run on Applications that match '{}'",
            s.iter()
                .map(|s| s.to_string())
                .collect::<Vec<String>>()
                .join(",")
        ),
        (None, Some(f)) => info!(
            "🤖 Will only run on Applications that watch these files: '{}'",
            f.join("`, `")
        ),
        (None, None) => {}
    }

    let number_of_apps_before_filtering = apps.len();

    let filtered_apps: Vec<Application> = filter(apps, selector, files_changed, ignore_invalid_watch_pattern);

    if number_of_apps_before_filtering != filtered_apps.len() {
        info!(
            "🤖 Found {} applications before filtering",
            number_of_apps_before_filtering
        );
        info!(
            "🤖 Found {} applications after filtering",
            filtered_apps.len()
        );
    } else {
        info!("🤖 Found {} applications", number_of_apps_before_filtering);
    }

    filtered_apps
}

pub fn applications_to_string(applications: Vec<Application>) -> String {
    applications
        .iter()
        .map(|a| a.to_string())
        .collect::<Vec<String>>()
        .join("---\n")
}
