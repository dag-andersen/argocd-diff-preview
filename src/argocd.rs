use crate::run_command;
use base64::prelude::*;
use log::{debug, error, info};
use std::{error::Error, process::Output};

pub struct ArgoCDOptions<'a> {
    pub version: Option<&'a str>,
    pub debug: bool,
}

const CONFIG_PATH: &str = "argocd-config";

pub async fn install_argo_cd(options: ArgoCDOptions<'_>) -> Result<(), Box<dyn Error>> {
    info!(
        "🦑 Installing Argo CD Helm Chart version: '{}'",
        options.version.unwrap_or("latest")
    );

    let (values, values_override) = match std::fs::read_dir(CONFIG_PATH) {
        Ok(dir) => {
            debug!("📂 Files in folder 'argocd-config':");
            for file in dir {
                debug!("- 📄 {:?}", file.unwrap().file_name());
            }
            let values_exist = std::fs::metadata(format!("{}/values.yaml", CONFIG_PATH))
                .is_ok()
                .then_some("-f argocd-config/values.yaml");
            let values_override_exist =
                std::fs::metadata(format!("{}/values-override.yaml", CONFIG_PATH))
                    .is_ok()
                    .then_some("-f argocd-config/values-override.yaml");
            (values_exist, values_override_exist)
        }
        Err(_e) => {
            info!("📂 Folder '{}' doesn't exist. Installing Argo CD Helm Chart with default configuration", CONFIG_PATH);
            (None, None)
        }
    };

    // create namespace argocd
    match run_command("kubectl create ns argocd", None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Failed to create namespace argocd");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    // add argo repo to helm
    match run_command(
        "helm repo add argo https://argoproj.github.io/argo-helm",
        None,
    )
    .await
    {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Failed to add argo repo");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    info!("🦑 Installing Argo CD Helm Chart");

    let helm_install_command = format!(
        "helm install argocd argo/argo-cd -n argocd {} {} {}",
        values.unwrap_or_default(),
        values_override.unwrap_or_default(),
        options
            .version
            .map(|a| format!("--version {}", a))
            .unwrap_or_default(),
    );

    match run_command(&helm_install_command, None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Failed to install Argo CD");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    info!("🦑 Waiting for Argo CD to start...");

    // wait for argocd-server to be ready
    run_command(
        "kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=300s",
        None,
    )
    .await
    .expect("failed to wait for argocd-server");

    info!("🦑 Argo CD is now available");

    info!("🦑 Logging in to Argo CD through CLI...");

    let password = {
        debug!("Getting initial admin password...");
        let secret_name = "argocd-initial-admin-secret";
        let command =
            "kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath={.data.password}";

        let mut password_encoded: Option<Output> = None;
        let mut counter = 0;
        while password_encoded.is_none() {
            password_encoded = match run_command(&command, None).await {
                Ok(a) => Some(a),
                Err(e) => {
                    if counter == 5 {
                        error!("❌ Failed to get secret {}", secret_name);
                        panic!("error: {}", String::from_utf8_lossy(&e.stderr))
                    }
                    counter += 1;
                    tokio::time::sleep(tokio::time::Duration::from_secs(2)).await;
                    debug!("⏳ Retrying to get secret {}", secret_name);
                    None
                }
            }
        }
        let password_encoded = password_encoded.unwrap().stdout;
        let password_decoded = BASE64_STANDARD
            .decode(password_encoded)
            .expect("failed to decode password");
        let password =
            String::from_utf8(password_decoded).expect("failed to convert password to string");

        password
    };

    // sleep for 5 seconds
    tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;

    // log into Argo CD

    let username = "admin";
    debug!(
        "Logging in to Argo CD with username, {} and password, {}",
        username, password
    );

    run_command(
        &format!(
            "argocd login localhost:8080 --insecure --username {} --password {}",
            username, password
        ),
        None,
    )
    .await
    .expect("failed to login to argocd");

    run_command("argocd app list", None)
        .await
        .expect("Failed to run: argocd app list");

    if options.debug {
        let command = "kubectl get configmap -n argocd -o yaml argocd-cmd-params-cm argocd-cm";
        match run_command(command, None).await {
            Ok(o) => debug!(
                "🔧 Configmap argocd-cmd-params-cm and argocd-cm:\n{}\n{}",
                command,
                String::from_utf8_lossy(&o.stdout)
            ),
            Err(e) => {
                error!("❌ Failed to get configmap");
                panic!("error: {}", String::from_utf8_lossy(&e.stderr))
            }
        }
    }

    info!("🦑 Argo CD installed successfully");
    Ok(())
}
