use crate::utils::run_command;
use crate::{apply_manifest, apps_file, Branch};
use log::{debug, error, info};
use std::collections::HashSet;
use std::fs;
use std::process::{Command, Stdio};
use std::{collections::BTreeMap, error::Error};

static ERROR_MESSAGES: [&str; 10] = [
    "helm template .",
    "authentication required",
    "authentication failed",
    "path does not exist",
    "error converting YAML to JSON",
    "Unknown desc = `helm template .",
    "Unknown desc = `kustomize build",
    "Unknown desc = Unable to resolve",
    "is not a valid chart repository or cannot be reached",
    "Unknown desc = repository not found",
];

static TIMEOUT_MESSAGES: [&str; 7] = [
    "Client.Timeout",
    "failed to get git client for repo",
    "rpc error: code = Unknown desc = Get \"https",
    "i/o timeout",
    "Could not resolve host: github.com",
    ":8081: connect: connection refused",
    "Temporary failure in name resolution", // Attempt at fixing: https://github.com/dag-andersen/argocd-diff-preview/issues/44
];

pub async fn get_resources(
    branch_type: &Branch,
    timeout: u64,
    output_folder: &str,
) -> Result<(), Box<dyn Error>> {
    info!("🌚 Getting resources from {}-branch", branch_type);

    let app_file = apps_file(branch_type);

    if fs::metadata(app_file).unwrap().len() != 0 {
        if let Err(e) = apply_manifest(app_file) {
            error!(
                "❌ Failed to apply applications for branch: {}",
                branch_type
            );
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
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
        let mut other_errors = vec![];

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
                                format!("{}/{}/{}", output_folder, branch_type, name),
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
                            if let Some(t) = condition["type"].as_str() {
                                if t.to_lowercase().contains("error") {
                                    match condition["message"].as_str() {
                                        Some(msg)
                                            if ERROR_MESSAGES.iter().any(|e| msg.contains(e)) =>
                                        {
                                            set_of_failed_apps
                                                .insert(name.to_string().clone(), msg.to_string());
                                            continue;
                                        }
                                        Some(msg)
                                            if TIMEOUT_MESSAGES.iter().any(|e| msg.contains(e)) =>
                                        {
                                            debug!("Application: {} timed out with error: {}", name, msg);
                                            list_of_timed_out_apps.push(name.to_string().clone());
                                            other_errors.push((name.to_string(), msg.to_string()));
                                        }
                                        Some(msg) => {
                                            debug!("Application: {} failed with error: {}", name, msg);
                                            other_errors.push((name.to_string(), msg.to_string()));
                                        }
                                        _ => (),
                                    }
                                }
                            }
                        }
                    }
                }
                _ => (),
            }
            apps_left += 1
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

        if items.len() == set_of_processed_apps.len() {
            tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
            continue;
        }

        // TIMEOUT
        let time_elapsed = start_time.elapsed().as_secs();
        if time_elapsed > timeout {
            error!("❌ Timed out after {} seconds", timeout);
            error!(
                "❌ Processed {} applications, but {} applications still remain",
                set_of_processed_apps.len(),
                apps_left
            );
            if !other_errors.is_empty() {
                error!("❌ Applications with 'ComparisonError' errors:");
                for (name, msg) in &other_errors {
                    error!("❌ {}, {}", name, msg);
                }
            }
            return Err("Timed out".into());
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

pub async fn delete_applications() {
    info!("🧼 Removing applications");
    loop {
        debug!("🗑 Deleting ApplicationSets");

        match run_command(
            "kubectl delete applicationsets.argoproj.io --all -n argocd",
            None,
        )
        .await
        {
            Ok(_) => debug!("🗑 Deleted ApplicationSets"),
            Err(e) => {
                error!(
                    "❌ Failed to delete applicationsets: {}",
                    String::from_utf8_lossy(&e.stderr)
                )
            }
        };

        debug!("🗑 Deleting Applications");

        let args = "kubectl delete applications.argoproj.io --all -n argocd"
            .split_whitespace()
            .collect::<Vec<&str>>();
        let mut child = Command::new(args[0])
            .args(&args[1..])
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .expect("failed to execute process");

        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
        if run_command("kubectl get applications -A --no-headers", None)
            .await
            .map(|o| String::from_utf8_lossy(&o.stdout).to_string())
            .map(|e| e.trim().is_empty())
            .unwrap_or_default()
        {
            let _ = child.kill();
            break;
        }

        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
        if run_command("kubectl get applications -A --no-headers", None)
            .await
            .map(|o| String::from_utf8_lossy(&o.stdout).to_string())
            .map(|e| e.trim().is_empty())
            .unwrap_or_default()
        {
            let _ = child.kill();
            break;
        }

        match child.kill() {
            Ok(_) => debug!("Timed out. Retrying..."),
            Err(e) => error!("❌ Failed to delete applications: {}", e),
        };
    }
    info!("🧼 Removed applications successfully")
}
