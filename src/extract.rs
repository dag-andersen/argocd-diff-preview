use crate::argocd::ArgoCDInstallation;
use crate::error::CommandError;
use crate::utils::{self, run_simple_command, write_to_file};
use crate::{apply_manifest, Branch};
use log::{debug, error, info};
use serde_yaml::Value;
use std::collections::HashSet;
use std::fs;
use std::{collections::BTreeMap, error::Error};

static ERROR_MESSAGES: [&str; 11] = [
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
    "InvalidSpecError: Namespace for"
];

static TIMEOUT_MESSAGES: [&str; 8] = [
    "Client.Timeout",
    "failed to get git client for repo",
    "rpc error: code = Unknown desc = Get \"https",
    "i/o timeout",
    "Could not resolve host: github.com",
    ":8081: connect: connection refused",
    "Temporary failure in name resolution", // Attempt at fixing: https://github.com/dag-andersen/argocd-diff-preview/issues/44
    "info/refs?service=git-upload-pack"
];

pub async fn get_resources(
    argocd: &ArgoCDInstallation,
    branch: &Branch,
    timeout: u64,
    output_folder: &str,
    input_folder: &str,
) -> Result<(), Box<dyn Error>> {
    info!("🌚 Getting resources from {}-branch", branch.branch_type);

    let app_file = &format!("{}/{}", input_folder, branch.app_file());

    if fs::metadata(app_file)
        .inspect_err(|_| error!("❌ File does not exist: {}", app_file))?
        .len()
        != 0
    {
        apply_manifest(app_file).map_err(|e| {
            error!(
                "❌ Failed to apply applications for branch: {}",
                branch.name
            );
            CommandError::new(e)
        })?;
    }

    let destination_folder = format!("{}/{}", output_folder, branch.branch_type);

    let mut processed_apps = HashSet::new();
    let mut failed_apps = BTreeMap::new();

    let start_time = std::time::Instant::now();

    loop {
        let command = "kubectl get applications -A -oyaml".to_string();
        let yaml_output: Value = match run_simple_command(&command) {
            Err(e) => return Err(format!("❌ Failed to get applications: {}", e.stderr).into()),
            Ok(o) => serde_yaml::from_str(&o.stdout).inspect_err(|_e| {
                error!("❌ Failed to parse yaml from command: {}", command);
            }),
        }?;

        let applications = match yaml_output["items"].as_sequence() {
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
                    match argocd.get_manifests(name) {
                        Ok(o) => {
                            write_to_file(&format!("{}/{}", destination_folder, name), &o.stdout)?;
                            debug!("Got manifests for application: {}", name);
                            processed_apps.insert(name.to_string().clone());
                        }
                        Err(e) => {
                            error!("❌ Failed to get manifests for application: {}", name);
                            failed_apps.insert(name.to_string().clone(), e.stderr);
                        }
                    }
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
            utils::sleep(5).await;
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
            info!("💤 {} Applications timed out.", timed_out_apps.len(),);
            for app in &timed_out_apps {
                match &argocd.refresh_app(app) {
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

        utils::sleep(5).await;
    }

    info!(
        "🌚 Got all resources from {} applications for {}",
        processed_apps.len(),
        branch.name
    );

    // info about where it was stored
    info!(
        "💾 Resources stored in: '{}/<app_name>'",
        destination_folder
    );

    Ok(())
}

// List of finalizers that prevent deletion of applications
static FINALIZERS: [&str; 2] = [
    "post-delete-finalizer.argocd.argoproj.io",
    "post-delete-finalizer.argoproj.io/cleanup",
];

pub fn remove_obstructive_finalizers() -> Result<(), Box<dyn Error>> {
    let command = "kubectl get applications -A -oyaml";
    let command_output = run_simple_command(command).map_err(|e| {
        error!("❌ Failed to get applications: {}", e.stderr);
        CommandError::new(e)
    })?;
    let yaml_output: Value = serde_yaml::from_str(&command_output.stdout).inspect_err(|_| {
        error!("❌ Failed to parse yaml from command: {}", command);
    })?;

    let applications = match yaml_output["items"].as_sequence() {
        None => return Ok(()),
        Some(apps) => apps,
    };

    for item in applications {
        let (name, namespace) = match (
            item["metadata"]["name"].as_str(),
            item["metadata"]["namespace"].as_str(),
        ) {
            (Some(name), Some(namespace)) => (name, namespace),
            _ => continue,
        };
        let has_finalizers = item["metadata"]["finalizers"]
            .as_sequence()
            .and_then(|f| {
                // check if any of the finalizers are in the list of finalizers to remove
                f.iter()
                    .find(|f| FINALIZERS.contains(&f.as_str().unwrap_or_default()))
            })
            .is_some();

        if has_finalizers {
            debug!("Removing finalizers from Application: {}", name);
            run_simple_command(&format!(
                    "kubectl patch application.argoproj.io {} --type merge --patch {{\"metadata\":{{\"finalizers\":null}}}} -n {}",
                    name, namespace
                ))
                .map_err(|e| {
                    error!("❌ Failed to remove finalizers from Application {} with error: {}", name, e.stderr);
                    CommandError::new(e)
                })?;
        }
    }

    Ok(())
}

pub async fn delete_applications() -> Result<(), Box<dyn Error>> {
    info!("🧼 Removing applications");

    remove_obstructive_finalizers().inspect_err(|_| {
        error!("❌ Failed to remove delete finalizers from Applications");
    })?;

    let verify_no_apps = || -> bool {
        run_simple_command("kubectl get applications -A --no-headers")
            .map(|e| e.stdout.trim().is_empty())
            .unwrap_or_default()
    };

    let mut counter = 0;
    let retry_count = 3;
    loop {
        debug!("🗑 Deleting Applications");

        let _result =
            run_simple_command("kubectl delete applications.argoproj.io --all -A --timeout 10s")
                .inspect_err(|e| {
                    debug!("Error: {}", e.stderr);
                });

        if verify_no_apps() {
            break;
        }

        if counter == retry_count {
            error!(
                "❌ Failed to delete applications after {} retries",
                retry_count
            );
            return Err("Failed to delete applications".into());
        }

        info!("⚠️ Failed to delete applications. Retrying...");
        counter += 1;
    }

    info!("🧼 Removed applications successfully");
    Ok(())
}
