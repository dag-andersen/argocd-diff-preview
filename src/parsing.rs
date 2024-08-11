use log::{debug, info};
use regex::Regex;
use serde_yaml::Mapping;
use std::{error::Error, io::BufRead};

struct K8sResource {
    file_name: String,
    yaml: serde_yaml::Value,
}

struct Application {
    file_name: String,
    yaml: serde_yaml::Value,
    kind: ApplicationKind,
}

enum ApplicationKind {
    Application,
    ApplicationSet,
}

pub async fn get_applications_as_string(
    directory: &str,
    branch: &str,
    regex: &Option<Regex>,
    repo: &str,
) -> Result<String, Box<dyn Error>> {
    let yaml_files = get_yaml_files(directory, regex).await;
    let k8s_resources = parse_yaml(yaml_files).await;
    let applications = get_applications(k8s_resources);
    let output = patch_applications(applications, branch, repo).await?;
    Ok(output)
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
        .map(|e| format!("{}", e.path().display()))
        .filter(|f| regex.is_none() || regex.as_ref().unwrap().is_match(f))
        .collect();

    match regex {
        Some(r) => debug!(
            "🤖 Found {} yaml files matching regex: {}",
            yaml_files.len(),
            r.as_str()
        ),
        None => debug!("🤖 Found {} yaml files", yaml_files.len()),
    }

    yaml_files
}

async fn parse_yaml(files: Vec<String>) -> Vec<K8sResource> {
    files.iter()
        .flat_map(|f| {
            debug!("Found file: {}", f);
            let file = std::fs::File::open(f).unwrap();
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
) -> Result<String, Box<dyn Error>> {
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

    // convert back to yaml string
    let mut output = String::new();
    for r in applications {
        output.push_str(&serde_yaml::to_string(&r.yaml)?);
        output.push_str("---\n");
    }

    Ok(output)
}

fn get_applications(k8s_resources: Vec<K8sResource>) -> Vec<Application> {
    k8s_resources
        .into_iter()
        .filter_map(|r| {
            debug!("Processing file: {}", r.file_name);
            let kind =
                r.yaml["kind"]
                    .as_str()
                    .map(|s| s.to_string())
                    .and_then(|kind| match kind.as_str() {
                        "Application" => Some(ApplicationKind::Application),
                        "ApplicationSet" => Some(ApplicationKind::ApplicationSet),
                        _ => None,
                    })?;

            if r.yaml["metadata"]["annotations"]["argocd-diff-preview/ignore"].as_str()
                == Some("true")
            {
                debug!(
                    "Ignoring application {:?} due to 'argocd-diff-preview/ignore=true' in file: {}",
                    r.yaml["metadata"]["name"].as_str().unwrap_or("unknown"),
                    r.file_name
                );
                return None;
            }

            Some(Application {
                kind,
                file_name: r.file_name,
                yaml: r.yaml,
            })
        })
        .collect()
}
