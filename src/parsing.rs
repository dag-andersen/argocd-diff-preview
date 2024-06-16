use regex::Regex;
use serde_yaml::Mapping;
use std::{error::Error, io::BufRead};

use log::{debug, info};

type K8sResource = serde_yaml::Value;
type K8sFiles = Vec<(String, K8sResource)>;

pub async fn get_applications(
    directory: &str,
    branch: &str,
    regex: &Option<Regex>,
    repo: &str,
) -> Result<String, Box<dyn Error>> {
    let applications = parse_yaml(directory, regex).await?;
    let output = patch_argocd_applications(applications, branch, repo).await?;
    Ok(output)
}

async fn parse_yaml(directory: &str, regex: &Option<Regex>) -> Result<K8sFiles, Box<dyn Error>> {
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

    let k8s_resources: K8sFiles = yaml_files
        .flat_map(|f| {
            debug!("Found file: {}", f);
            let file = std::fs::File::open(&f).unwrap();
            let reader = std::io::BufReader::new(file);
            let lines = reader.lines().map(|l| l.unwrap());

            let mut raw_yaml_chunks: Vec<String> = lines.fold(vec!["".to_string()], |mut acc, s| {
                if s == "---" {
                    acc.push("".to_string());
                } else {
                    let last = acc.len() - 1;
                    acc[last].push_str("\n");
                    acc[last].push_str(&s);
                }
                acc
            });
            let yaml_vec: K8sFiles = raw_yaml_chunks.iter_mut().enumerate().map(|(i,r)| {
                let yaml = match serde_yaml::from_str(r) {
                    Ok(r) => r,
                    Err(e) => {
                        debug!("⚠️ Failed to parse element number {}, in file '{}', with error: '{}'", i+1, f, e);
                        serde_yaml::Value::Null
                    }
                };
                (f.clone(),yaml)
            }).collect();
            yaml_vec
        })
        .collect();

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
    yaml_chunks: K8sFiles,
    branch: &str,
    repo: &str,
) -> Result<String, Box<dyn Error>> {
    info!("🤖 Patching applications for branch: {}", branch);

    let clean_spec = |spec: &mut Mapping| {
        spec["project"] = serde_yaml::Value::String("default".to_string());
        if spec.contains_key("destination") {
            spec["destination"]["name"] = serde_yaml::Value::String("in-cluster".to_string());
            spec["destination"]
                .as_mapping_mut()
                .map(|a| a.remove("server"));
        }
        spec.remove("syncPolicy");
    };

    let redirect_sources = |file: &str, spec: &mut Mapping| {
        if spec.contains_key("source") {
            match spec["source"]["repoURL"].as_str() {
                Some(url) if url.contains(repo) => {
                    spec["source"]["targetRevision"] = serde_yaml::Value::String(branch.to_string())
                }
                _ => debug!("Found no 'repoURL' under spec.sources[] in file: {}", file),
            }
        } else if spec.contains_key("sources") {
            if let Some(sources) = spec["sources"].as_sequence_mut() {
                for source in sources {
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

    let applications: Vec<K8sResource> = yaml_chunks
        .into_iter()
        .map(|(f, mut r)| {
            r["metadata"]["namespace"] = serde_yaml::Value::String("argocd".to_string());
            (f, r)
        })
        .filter_map(|(f, r)| {
            r["kind"].as_str().map(|s| s.to_string()).and_then(|kind| {
                (kind == "Application" || kind == "ApplicationSet").then(|| (f, kind, r))
            })
        })
        .filter_map(|(f, kind, mut r)| {
            // Clean up the spec
            clean_spec({
                if kind == "Application" {
                    r["spec"].as_mapping_mut()?
                } else {
                    r["spec"]["template"]["spec"].as_mapping_mut()?
                }
            });
            Some((f, kind, r))
        })
        .filter_map(|(f, kind, mut r)| {
            // Redirect all applications to the branch
            redirect_sources(&f, {
                if kind == "Application" {
                    r["spec"].as_mapping_mut()?
                } else {
                    r["spec"]["template"]["spec"].as_mapping_mut()?
                }
            });
            debug!(
                "Collected resources from application: {:?} in file: {}",
                r["metadata"]["name"].as_str().unwrap_or("unknown"),
                f
            );
            Some(r)
        })
        .collect();

    info!(
        "🤖 Patching {} Argo CD Application[Sets] for branch: {}",
        applications.len(),
        branch
    );

    // convert back to string
    let mut output = String::new();
    for r in &applications {
        let r = serde_yaml::to_string(r)?;
        output.push_str(&r);
        output.push_str("---\n");
    }

    Ok(output)
}
