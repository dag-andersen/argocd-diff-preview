use std::collections::HashSet;
use std::fs;
use std::{collections::BTreeMap, error::Error};

use log::{debug, error, info};

use crate::utils::run_command;
use crate::{apply_manifest, apps_file, Branch};

static ERROR_MESSAGES: [&str; 6] = [
    "helm template .",
    "authentication required",
    "authentication failed",
    "path does not exist",
    "error converting YAML to JSON",
    "Unknown desc = `helm template .",
];

static TIMEOUT_MESSAGES: [&str; 3] = [
    "Client.Timeout",
    "failed to get git client for repo",
    "rpc error: code = Unknown desc = Get \"https",
];

pub async fn get_resources(
    branch_type: &Branch,
    timeout: u64,
    output_folder: &str,
) -> Result<(), Box<dyn Error>> {
    info!("🌚 Getting resources for {}", branch_type);

    let app_file = apps_file(branch_type);

    delete_applications().await;

    if fs::metadata(app_file).unwrap().len() != 0 {
        match apply_manifest(app_file) {
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

        if items.len() == set_of_processed_apps.len() {
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
                    debug!("Getting manifests for application: {}", name);
                    match run_command(&format!("argocd app manifests {}", name), None).await {
                        Ok(o) => {
                            fs::write(
                                &format!("{}/{}/{}", output_folder, branch_type, name),
                                &o.stdout,
                            )?;
                            debug!("Got manifests for application: {}", name)
                        }
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

        // TIMEOUT
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

        // ERRORS
        if !set_of_failed_apps.is_empty() {
            for (name, msg) in &set_of_failed_apps {
                error!(
                    "❌ Failed to process application: {} with error: \n{}",
                    name, msg
                );
            }
            return Err("Failed to process applications".into());
        }

        // TIMED OUT APPS
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

        if apps_left > 0 {
            info!(
                "⏳ Waiting for {} out of {} applications to become 'OutOfSync'. Retrying in 5 seconds. Timeout in {} seconds...",
                apps_left,
                items.len(),
                timeout - time_elapsed
            );
        }

        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }

    info!(
        "🌚 Got all resources from {} applications for {}",
        set_of_processed_apps.len(),
        branch_type
    );

    Ok(())
}

async fn delete_applications() {
    match run_command(
        "kubectl delete applicationsets.argoproj.io,applications.argoproj.io --all -n argocd",
        None,
    )
    .await
    {
        Ok(_) => debug!("Deleted applications"),
        Err(e) => error!("error: {}", String::from_utf8_lossy(&e.stderr)),
    }
}
