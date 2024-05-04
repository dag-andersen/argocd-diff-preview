use std::collections::HashSet;
use std::fs;
use std::process::{Command, Stdio};
use std::{collections::BTreeMap, error::Error};

use log::{debug, error, info};

use crate::utils::run_command;
use crate::{apply_manifest, apps_file, Branch};

static ERROR_MESSAGES: [&str; 8] = [
    "helm template .",
    "authentication required",
    "authentication failed",
    "path does not exist",
    "error converting YAML to JSON",
    "Unknown desc = `helm template .",
    "Unknown desc = Unable to resolve",
    "is not a valid chart repository or cannot be reached",
];

static TIMEOUT_MESSAGES: [&str; 4] = [
    "Client.Timeout",
    "failed to get git client for repo",
    "rpc error: code = Unknown desc = Get \"https",
    "i/o timeout",
];

pub async fn get_resources(
    branch_type: &Branch,
    timeout: u64,
    output_folder: &str,
) -> Result<(), Box<dyn Error>> {
    info!("🌚 Getting resources for {}", branch_type);

    let app_file = apps_file(branch_type);

    if fs::metadata(app_file).unwrap().len() != 0 {
        if let Err(e) = apply_manifest(app_file) {
            error!("❌ Failed to apply applications for branch: {}", branch_type);
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    let multi_progress = indicatif::MultiProgress::new();
    let progress_bar =  multi_progress.add(indicatif::ProgressBar::new(100));
    let spinner_with_status = multi_progress.add(indicatif::ProgressBar::new_spinner());

    // style including, emoji, progress bar, x out of y, and time left
    progress_bar.set_style(
        indicatif::ProgressStyle::default_bar()
            .template("{spinner:.green} [{elapsed_precise}] [{bar:40.cyan/blue}] {pos}/{len} {msg}")
            .expect("Failed to set style")
            .progress_chars("##-"),
    );

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

        progress_bar.set_length(items.len() as u64);
        progress_bar.set_position(set_of_processed_apps.len() as u64);

        if items.len() == set_of_processed_apps.len() {
            break;
        }

        let mut list_of_timed_out_apps = vec![];
        let mut other_errors = vec![];

        let mut apps_left = 0;

        for item in items {
            let name = item["metadata"]["name"].as_str().unwrap();
            spinner_with_status.set_message(format!("Timeout in {} seconds, Processing application: {}", timeout - start_time.elapsed().as_secs(), name));
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
                    progress_bar.set_position(set_of_processed_apps.len() as u64);
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
                                        other_errors.push((name.to_string(), msg.to_string()));
                                    }
                                    Some(msg) => {
                                        other_errors.push((name.to_string(), msg.to_string()));
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
        if time_elapsed > timeout as u64 {
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

        spinner_with_status.set_message(format!("Timeout in {} seconds, Waiting for applications to sync...", timeout - start_time.elapsed().as_secs()));

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
