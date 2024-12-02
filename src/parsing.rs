use crate::{
    argo_resource::{ApplicationKind, ArgoResource},
    error::CommandError,
    utils::run_command,
    Branch, Selector,
};
use log::{debug, info};
use regex::Regex;
use serde_yaml::Value;
use std::{error::Error, io::BufRead};

pub struct K8sResource {
    pub file_name: String,
    pub yaml: serde_yaml::Value,
}

impl Clone for K8sResource {
    fn clone(&self) -> Self {
        K8sResource {
            file_name: self.file_name.clone(),
            yaml: self.yaml.clone(),
        }
    }
}

pub fn get_applications_for_both_branches<'a>(
    base_branch: &Branch,
    target_branch: &Branch,
    regex: &Option<Regex>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    repo: &str,
    ignore_invalid_watch_pattern: bool,
) -> Result<(Vec<ArgoResource>, Vec<ArgoResource>), Box<dyn Error>> {
    let base_apps = get_applications(
        base_branch,
        regex,
        selector,
        files_changed,
        repo,
        ignore_invalid_watch_pattern,
    )?;
    let target_apps = get_applications(
        target_branch,
        regex,
        selector,
        files_changed,
        repo,
        ignore_invalid_watch_pattern,
    )?;

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

pub fn get_applications(
    branch: &Branch,
    regex: &Option<Regex>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    repo: &str,
    ignore_invalid_watch_pattern: bool,
) -> Result<Vec<ArgoResource>, Box<dyn Error>> {
    let yaml_files = get_yaml_files(branch.folder_name(), regex);
    let k8s_resources = parse_yaml(branch.folder_name(), yaml_files);
    let applications = from_resource_to_application(
        k8s_resources,
        selector,
        files_changed,
        ignore_invalid_watch_pattern,
    );
    if !applications.is_empty() {
        return patch_applications(applications, branch, repo);
    }
    Ok(applications)
}

fn get_yaml_files(directory: &str, regex: &Option<Regex>) -> Vec<String> {
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

fn parse_yaml(directory: &str, files: Vec<String>) -> Vec<K8sResource> {
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

fn patch_applications(
    applications: Vec<ArgoResource>,
    branch: &Branch,
    repo: &str,
) -> Result<Vec<ArgoResource>, Box<dyn Error>> {
    info!("🤖 Patching applications for branch: {}", branch.name);

    let applications: Vec<Result<ArgoResource, Box<dyn Error>>> = applications
        .into_iter()
        .map(|a| {
            let app_name = a.name.clone();
            let app: Result<ArgoResource, Box<dyn Error>> = a
                .set_namespace("argocd")
                .remove_sync_policy()
                .set_project_to_default()
                .and_then(|a| a.point_destination_to_in_cluster())
                .and_then(|a| a.redirect_sources(repo, &branch.name));

            if app.is_err() {
                info!("❌ Failed to patch application: {}", app_name);
                return app;
            }
            app
        })
        .collect();

    info!(
        "🤖 Patching {} Argo CD Application[Sets] for branch: {}",
        applications.len(),
        branch.name
    );

    let errors: Vec<String> = applications
        .iter()
        .filter_map(|a| match a {
            Ok(_) => None,
            Err(e) => Some(e.to_string()),
        })
        .collect();

    if !errors.is_empty() {
        return Err(errors.join("\n").into());
    }

    let apps = applications.into_iter().filter_map(|a| a.ok()).collect();
    Ok(apps)
}

fn from_resource_to_application(
    k8s_resources: Vec<K8sResource>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    ignore_invalid_watch_pattern: bool,
) -> Vec<ArgoResource> {
    let apps: Vec<ArgoResource> = k8s_resources
        .iter()
        .filter_map(|r| ArgoResource::from_k8s_resource(r.clone()))
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

    let filtered_apps: Vec<ArgoResource> = apps
        .into_iter()
        .filter_map(|a| a.filter(selector, files_changed, ignore_invalid_watch_pattern))
        .collect();

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
