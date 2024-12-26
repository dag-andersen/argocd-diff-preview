use crate::argocd::{get_manifests, refresh_app, ARGO_CD_NAMESPACE};
use crate::error::CommandError;
use crate::utils::{run_command, spawn_command};
use crate::{apply_manifest, Branch};
use log::{debug, error, info};
use serde_yaml::Value;
use std::collections::HashSet;
use std::fs;
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
    branch: &Branch,
    timeout: u64,
    output_folder: &str,
) -> Result<(), Box<dyn Error>> {
    info!("🌚 Getting resources from {}-branch", branch.branch_type);

    let app_file = branch.app_file();

    if fs::metadata(app_file)?.len() != 0 {
        apply_manifest(app_file).map_err(|e| {
            error!(
                "❌ Failed to apply applications for branch: {}",
                branch.name
            );
            CommandError::new(e)
        })?;
    }

    let mut processed_apps = HashSet::new();
    let mut failed_apps = BTreeMap::new();

    let start_time = std::time::Instant::now();

    loop {
        let command = format!("kubectl get applications -n {} -oyaml", ARGO_CD_NAMESPACE);
        let applications: Result<Value, serde_yaml::Error> = match run_command(&command) {
            Ok(o) => serde_yaml::from_str(&o.stdout),
            Err(e) => return Err(format!("❌ Failed to get applications: {}", e.stderr).into()),
        };

        let applications = match applications {
            Ok(applications) => applications,
            Err(_) => {
                return Err(format!("❌ Failed to parse yaml from command: {}", command).into());
            }
        };

        let applications = match applications["items"].as_sequence() {
            None => break,
            Some(apps) if apps.is_empty() => break,
            Some(apps) if apps.len() == processed_apps.len() => break,
            Some(apps) => apps,
        };

        let mut timed_out_apps = vec![];
        let mut other_errors = vec![];

        let mut apps_left = 0;

        for item in applications {
            let name = item["metadata"]["name"].as_str().unwrap();
            if processed_apps.contains(name) {
                continue;
            }
            match item["status"]["sync"]["status"].as_str() {
                Some("OutOfSync") | Some("Synced") => {
                    debug!("Getting manifests for application: {}", name);
                    match get_manifests(name) {
                        Ok(o) => {
                            fs::write(
                                format!("{}/{}/{}", output_folder, branch.branch_type, name),
                                &o.stdout,
                            )?;
                            debug!("Got manifests for application: {}", name)
                        }
                        Err(e) => error!("error: {}", e.stderr),
                    }
                    processed_apps.insert(name.to_string().clone());
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
                                            failed_apps
                                                .insert(name.to_string().clone(), msg.to_string());
                                            continue;
                                        }
                                        Some(msg)
                                            if TIMEOUT_MESSAGES.iter().any(|e| msg.contains(e)) =>
                                        {
                                            debug!(
                                                "Application: {} timed out with error: {}",
                                                name, msg
                                            );
                                            timed_out_apps.push(name.to_string().clone());
                                            other_errors.push((name.to_string(), msg.to_string()));
                                        }
                                        Some(msg) => {
                                            debug!(
                                                "Application: {} failed with error: {}",
                                                name, msg
                                            );
                                            other_errors.push((name.to_string(), msg.to_string()));
                                        }
                                        None => (),
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
        if !failed_apps.is_empty() {
            for (name, msg) in &failed_apps {
                error!(
                    "❌ Failed to process application: {} with error: \n{}",
                    name, msg
                );
            }
            return Err("Failed to process applications".into());
        }

        if applications.len() == processed_apps.len() {
            tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
            continue;
        }

        // TIMEOUT
        let time_elapsed = start_time.elapsed().as_secs();
        if time_elapsed > timeout {
            error!("❌ Timed out after {} seconds", timeout);
            error!(
                "❌ Processed {} applications, but {} applications still remain",
                processed_apps.len(),
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
        if !timed_out_apps.is_empty() {
            info!(
                "💤 {} Applications timed out.",
                timed_out_apps.len(),
            );
            for app in &timed_out_apps {
                match refresh_app(app) {
                    Ok(_) => info!("🔄 Refreshing application: {}", app),
                    Err(e) => error!(
                        "⚠️ Failed to refresh application: {} with {}",
                        app, &e.stderr
                    ),
                }
            }
        }

        if apps_left > 0 {
            info!(
                "⏳ Waiting for {} out of {} applications to become 'OutOfSync'. Retrying in 5 seconds. Timeout in {} seconds...",
                apps_left,
                applications.len(),
                timeout - time_elapsed
            );
        }

        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }

    info!(
        "🌚 Got all resources from {} applications for {}",
        processed_apps.len(),
        branch.name
    );

    Ok(())
}

pub async fn delete_applications() -> Result<(), Box<dyn Error>> {
    info!("🧼 Removing applications");
    loop {
        debug!("🗑 Deleting ApplicationSets");

        match run_command(&format!(
            "kubectl delete applicationsets.argoproj.io --all -n {}",
            ARGO_CD_NAMESPACE
        )) {
            Ok(_) => debug!("🗑 Deleted ApplicationSets"),
            Err(e) => {
                error!("❌ Failed to delete applicationsets: {}", &e.stderr)
            }
        };

        debug!("🗑 Deleting Applications");

        let mut child = spawn_command(
            &format!(
                "kubectl delete applications.argoproj.io --all -n {}",
                ARGO_CD_NAMESPACE
            ),
            None,
        );
        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
        if run_command("kubectl get applications -A --no-headers")
            .map(|e| e.stdout.trim().is_empty())
            .unwrap_or_default()
        {
            let _ = child.kill();
            break;
        }

        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
        if run_command("kubectl get applications -A --no-headers")
            .map(|e| e.stdout.trim().is_empty())
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
    info!("🧼 Removed applications successfully");
    Ok(())
}
