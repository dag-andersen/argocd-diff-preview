use crate::{
    argo_resource::{ApplicationKind, ArgoResource},
    argocd::ArgoCDInstallation,
    error::CommandError,
    utils, Branch, Selector,
};
use log::{debug, error, info, warn};
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

pub fn get_applications_for_branches(
    argo_cd_namespace: &str,
    base_branch: &Branch,
    target_branch: &Branch,
    regex: &Option<Regex>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    repo: &str,
    ignore_invalid_watch_pattern: bool,
    redirect_revisions: &Option<Vec<String>>,
) -> Result<(Vec<ArgoResource>, Vec<ArgoResource>), Box<dyn Error>> {
    let base_apps = get_applications(
        argo_cd_namespace,
        base_branch,
        regex,
        selector,
        files_changed,
        repo,
        ignore_invalid_watch_pattern,
        redirect_revisions,
    )?;
    let target_apps = get_applications(
        argo_cd_namespace,
        target_branch,
        regex,
        selector,
        files_changed,
        repo,
        ignore_invalid_watch_pattern,
        redirect_revisions,
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
        let base_apps_before = base_apps.len();
        let target_apps_before = target_apps.len();

        // remove duplicates
        let base_apps: Vec<ArgoResource> = base_apps
            .into_iter()
            .filter(|a| !duplicate_yaml.contains(&a.yaml))
            .collect();
        let target_apps: Vec<ArgoResource> = target_apps
            .into_iter()
            .filter(|a| !duplicate_yaml.contains(&a.yaml))
            .collect();

        info!(
            "🤖 Skipped {} Application[Sets] for branch: '{}' because they have not changed after patching",
            base_apps_before - base_apps.len(),
            base_branch.name
        );

        info!(
            "🤖 Skipped {} Application[Sets] for branch: '{}' because they have not changed after patching",
            target_apps_before - target_apps.len(),
            target_branch.name
        );

        info!(
            "🤖 Using the remaining {} Application[Sets] for branch: '{}'",
            base_apps.len(),
            base_branch.name
        );

        info!(
            "🤖 Using the remaining {} Application[Sets] for branch: '{}'",
            target_apps.len(),
            target_branch.name
        );

        Ok((base_apps, target_apps))
    }
}

fn get_applications(
    argo_cd_namespace: &str,
    branch: &Branch,
    regex: &Option<Regex>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>,
    repo: &str,
    ignore_invalid_watch_pattern: bool,
    redirect_revisions: &Option<Vec<String>>,
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
        info!("🤖 Patching Application[Sets] for branch: {}", branch.name);
        let apps = patch_applications(
            argo_cd_namespace,
            applications,
            branch,
            repo,
            redirect_revisions,
        )?;
        info!(
            "🤖 Patching {} Argo CD Application[Sets] for branch: {}",
            apps.len(),
            branch.name
        );
        return Ok(apps);
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

fn patch_application(
    argo_cd_namespace: &str,
    application: ArgoResource,
    branch: &Branch,
    repo: &str,
    redirect_revisions: &Option<Vec<String>>,
) -> Result<ArgoResource, Box<dyn Error>> {
    let app_name = application.name.clone();
    let app = application
        .set_namespace(argo_cd_namespace)
        .remove_sync_policy()
        .set_project_to_default()
        .and_then(|a| a.point_destination_to_in_cluster())
        .and_then(|a| a.redirect_sources(repo, &branch.name, redirect_revisions))
        .and_then(|a| a.redirect_generators(repo, &branch.name, redirect_revisions));

    if app.is_err() {
        error!("❌ Failed to patch application: {}", app_name);
    }

    app
}

fn patch_applications(
    argo_cd_namespace: &str,
    applications: Vec<ArgoResource>,
    branch: &Branch,
    repo: &str,
    redirect_revisions: &Option<Vec<String>>,
) -> Result<Vec<ArgoResource>, Box<dyn Error>> {
    applications
        .into_iter()
        .map(|a| patch_application(argo_cd_namespace, a, branch, repo, redirect_revisions))
        .collect()
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
            f.join("', '")
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
            "🤖 Found {} Application[Sets] before filtering",
            number_of_apps_before_filtering
        );
        info!(
            "🤖 Found {} Application[Sets] after filtering",
            filtered_apps.len()
        );
    } else {
        info!(
            "🤖 Found {} Application[Sets]",
            number_of_apps_before_filtering
        );
    }

    filtered_apps
}

pub fn generate_apps_from_app_set(
    argocd: &ArgoCDInstallation,
    app_sets: Vec<ArgoResource>,
    branch: &Branch,
    repo: &str,
    temp_folder: &str,
    redirect_target_revisions: &Option<Vec<String>>,
) -> Result<Vec<ArgoResource>, Box<dyn Error>> {
    let mut apps_new: Vec<ArgoResource> = vec![];

    let mut app_set_counter = 0;
    let mut generated_apps_counter = 0;

    for app_set in app_sets {
        if app_set.kind != ApplicationKind::ApplicationSet {
            apps_new.push(app_set);
            continue;
        }

        app_set_counter += 1;

        // generate random name for ApplicationSet
        let random_file_name = format!(
            "{}/{}-{}.yaml",
            temp_folder,
            app_set.name,
            rand::random::<u32>()
        );
        utils::write_to_file(&random_file_name, &app_set.as_string()?)?;

        debug!(
            "Generating applications from ApplicationSet in file: {}",
            random_file_name
        );

        let apps_string = argocd
            .appset_generate(&random_file_name)
            .map_err(|e| {
                error!(
                    "❌ Failed to generate applications from ApplicationSet in file: {}",
                    app_set.file_name
                );
                CommandError::new(e)
            })?
            .stdout;

        let yaml: Value = match serde_yaml::from_str(&apps_string) {
            Ok(y) => y,
            Err(e) => {
                warn!(
                    "⚠️ Failed to parse yaml from generated ApplicationSet output (in file: {}) with error: '{}'",
                    app_set.file_name,
                    e
                );
                continue;
            }
        };

        let apps = match (yaml.as_sequence(), yaml.is_mapping()) {
            (Some(s), _) => {
                debug!(
                    "Got a list of {} Applications from ApplicationSet in file: {}",
                    app_set.file_name,
                    s.len()
                );
                let apps = s
                    .iter()
                    .filter_map(|a| {
                        ArgoResource::from_k8s_resource(K8sResource {
                            file_name: app_set.file_name.clone(),
                            yaml: a.clone(),
                        })
                    })
                    .collect::<Vec<ArgoResource>>();
                debug!(
                    "Generated {} Applications from ApplicationSet in file: {}",
                    apps.len(),
                    app_set.file_name
                );
                patch_applications(
                    &argocd.namespace,
                    apps,
                    branch,
                    repo,
                    redirect_target_revisions,
                )
            }
            (_, true) => {
                debug!(
                    "Got a single Application from ApplicationSet in file: {}",
                    app_set.file_name
                );
                let resource = ArgoResource::from_k8s_resource(K8sResource {
                    file_name: app_set.file_name.clone(),
                    yaml,
                });
                match resource {
                    None => {
                        warn!(
                        "⚠️ Failed to parse yaml from generated applications from ApplicationSet (in file: {})",
                        app_set.file_name
                    );
                        continue;
                    }
                    Some(r) => patch_application(
                        &argocd.namespace,
                        r,
                        branch,
                        repo,
                        redirect_target_revisions,
                    )
                    .map(|a| vec![a]),
                }
            }
            (None, false) => continue,
        };

        match apps {
            Ok(apps) => {
                debug!(
                    "Generated {} Applications from ApplicationSet in file: {}",
                    apps.len(),
                    app_set.file_name
                );
                generated_apps_counter += apps.len();
                apps_new.extend(apps);
            }
            Err(e) => {
                error!(
                    "❌ Failed to generate Applications from ApplicationSet in file: {}",
                    app_set.file_name
                );
                return Err(e);
            }
        }
    }

    if app_set_counter > 0 {
        info!(
            "🤖 Generated {} applications from {} ApplicationSets for branch: {}",
            generated_apps_counter, app_set_counter, branch.name
        );
    } else {
        info!("🤖 No ApplicationSets found for branch: {}", branch.name);
    }

    debug_assert!(
        apps_new
            .iter()
            .all(|a| a.kind == ApplicationKind::Application),
        "All applications should be of kind Application"
    );

    Ok(apps_new)
}
