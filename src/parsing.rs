use regex::Regex;
use serde_yaml::Mapping;
use std::{error::Error, io::BufRead};

use log::{debug, info};

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
        spec["project"] = serde_yaml::Value::String("default".to_string());
        spec["destination"]["name"] = serde_yaml::Value::String("in-cluster".to_string());
        spec.remove("syncPolicy");
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
        .map(|mut r| {
            r["metadata"]["namespace"] = serde_yaml::Value::String("argocd".to_string());
            r
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
    for r in &applications {
        let r = serde_yaml::to_string(r).unwrap();
        output.push_str(&r);
        output.push_str("---\n");
    }

    Ok(output)
}
