use crate::{Operator, Selector};
use log::{debug, info, warn};
use regex::Regex;
use serde_yaml::Mapping;
use std::{error::Error, io::BufRead};

struct K8sResource {
    file_name: String,
    yaml: serde_yaml::Value,
}

pub struct Application {
    file_name: String,
    yaml: serde_yaml::Value,
    kind: ApplicationKind,
}

impl std::fmt::Display for Application {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", serde_yaml::to_string(&self.yaml).unwrap())
    }
}

enum ApplicationKind {
    Application,
    ApplicationSet,
}

const ANNOTATION_WATCH_PATTERN: &str = "argocd-diff-preview/watch-pattern";
const ANNOTATION_IGNORE: &str = "argocd-diff-preview/ignore";

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
) -> Result<(Vec<Application>, Vec<Application>), Box<dyn Error>> {
    let base_apps = get_applications(
        base_branch.directory,
        base_branch.branch,
        regex,
        selector,
        files_changed,
        repo,
    )
    .await?;
    let target_apps = get_applications(
        target_branch.directory,
        target_branch.branch,
        regex,
        selector,
        files_changed,
        repo,
    )
    .await?;

    Ok((base_apps, target_apps))
}

pub async fn get_applications(
    directory: &str,
    branch: &str,
    regex: &Option<Regex>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    repo: &str,
) -> Result<Vec<Application>, Box<dyn Error>> {
    let yaml_files = get_yaml_files(directory, regex).await;
    let k8s_resources = parse_yaml(directory, yaml_files).await;
    let applications = from_resource_to_application(k8s_resources, selector, files_changed);
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
                Some(url) if url.contains(repo) => {
                    spec["source"]["targetRevision"] = serde_yaml::Value::String(branch.to_string())
                }
                _ => debug!("Found no 'repoURL' under spec.sources[] in file: {}", file),
            }
        } else if spec.contains_key("sources") {
            if let Some(sources) = spec["sources"].as_sequence_mut() {
                for source in sources {
                    if source["chart"].as_str().is_some() {
                        continue;
                    }
                    match source["repoURL"].as_str() {
                        Some(url) if url.contains(repo) => {
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
                a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                a.file_name
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

            Some(Application {
                kind,
                file_name: r.file_name,
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
        (None, None) => {},
    }

    let number_of_apps_before_filtering = apps.len();

    let filtered_apps: Vec<Application> = apps.into_iter().filter_map(|a| {

        // check if the application should be ignored
        if a.yaml["metadata"]["annotations"][ANNOTATION_IGNORE].as_str()
            == Some("true")
        {
            debug!(
                "Ignoring application {:?} due to '{}=true' in file: {}",
                a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                ANNOTATION_IGNORE,
                a.file_name
            );
            return None;
        }

        // loop over labels and check if the selector matches
        if let Some(selector) = selector {
            let labels: Vec<(&str, &str)> = {
                match a.yaml["metadata"]["labels"].as_mapping() {
                    Some(m) => m.iter()
                        .flat_map(|(k, v)| Some((k.as_str()?, v.as_str()?)))
                        .collect(),
                    None => Vec::new(),
                }
            };
            let selected = selector.iter().all(|l| match l.operator {
                Operator::Eq => labels.iter().any(|(k, v)| k == &l.key && v == &l.value),
                Operator::Ne => labels.iter().all(|(k, v)| k != &l.key || v != &l.value),
            });
            if !selected {
                debug!(
                    "Ignoring application {:?} due to label selector mismatch in file: {}",
                    a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                    a.file_name
                );
                return None;
            } else {
                debug!(
                    "Selected application {:?} due to label selector match in file: {}",
                    a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                    a.file_name
                );
            }
        }

        // Check watch pattern annotation
        let pattern_annotation = a.yaml["metadata"]["annotations"][ANNOTATION_WATCH_PATTERN].as_str();
        let pattern: Option<Result<Regex, regex::Error>> = pattern_annotation.map(Regex::new);
        match (files_changed, pattern) {
            (None, _) => {}
            // Check if the application changed.
            (Some(files_changed), _) if files_changed.contains(&a.file_name) => {
                debug!(
                    "Selected application {:?} due to file change in file: {}",
                    a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                    a.file_name
                );
            }
            // Check if the application changed and the regex pattern matches.
            (Some(files_changed), Some(Ok(pattern))) if files_changed.iter().any(|f| pattern.is_match(f)) => {
                debug!(
                    "Selected application {:?} due to regex pattern '{}' matching changed files",
                    a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                    pattern
                );
            }
            (_, Some(Ok(pattern))) => {
                debug!(
                    "Ignoring application {:?} due to regex pattern '{}' not matching changed files",
                    a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                    pattern
                );
                return None;
            },
            (_, Some(Err(e))) => {
                warn!(
                    "🚨 Ignoring application {:?} due to invalid regex pattern '{}' ({})",
                    a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                    pattern_annotation.unwrap(),
                    a.file_name
                );
                debug!("Error: {}", e);
                return None;
            }
            (_, None) => {
                debug!(
                    "Ignoring application {:?} due to missing '{}' annotation ({})",
                    a.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                    &ANNOTATION_WATCH_PATTERN,
                    a.file_name
                );
                return None;
            }
        }

        Some(a)
    }).collect();

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
