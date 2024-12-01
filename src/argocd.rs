use crate::utils::{run_command, CommandError, CommandOutput};
use base64::prelude::*;
use log::{debug, error, info};
use std::error::Error;

pub struct ArgoCDOptions<'a> {
    pub version: Option<&'a str>,
    pub debug: bool,
}

const CONFIG_PATH: &str = "argocd-config";

pub async fn create_namespace() -> Result<(), Box<dyn Error>> {
    run_command("kubectl create ns argocd", None)
        .await
        .map_err(|e| {
            error!("❌ Failed to create namespace argocd");
            CommandError::new(e)
        })?;
    debug!("🦑 Namespace argocd created successfully");
    Ok(())
}

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
                .then_some(format!("-f {}/values.yaml", CONFIG_PATH));
            let values_override_exist =
                std::fs::metadata(format!("{}/values-override.yaml", CONFIG_PATH))
                    .is_ok()
                    .then_some(format!("-f {}/values-override.yaml", CONFIG_PATH));
            (values_exist, values_override_exist)
        }
        Err(_e) => {
            info!("📂 Folder '{}' doesn't exist. Installing Argo CD Helm Chart with default configuration", CONFIG_PATH);
            (None, None)
        }
    };

    // add argo repo to helm
    run_command(
        "helm repo add argo https://argoproj.github.io/argo-helm",
        None,
    )
    .await
    .map_err(|e| {
        error!("❌ Failed to add argo repo");
        CommandError::new(e)
    })?;

    let helm_install_command = format!(
        "helm install argocd argo/argo-cd -n argocd {} {} {}",
        values.unwrap_or_default(),
        values_override.unwrap_or_default(),
        options
            .version
            .map(|a| format!("--version {}", a))
            .unwrap_or_default(),
    );

    run_command(&helm_install_command, None)
        .await
        .map_err(|e| {
            error!("❌ Failed to install Argo CD");
            CommandError::new(e)
        })?;

    info!("🦑 Waiting for Argo CD to start...");

    // wait for argocd-server to be ready
    match run_command(
        "kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=300s",
        None,
    )
    .await
    {
        Ok(_) => info!("🦑 Argo CD is now available"),
        Err(_) => {
            error!("❌ Failed to wait for argocd-server");
            return Err("Failed to wait for argocd-server".to_string().into());
        }
    }

    info!("🦑 Logging in to Argo CD through CLI...");

    let password = {
        debug!("Getting initial admin password...");
        let secret_name = "argocd-initial-admin-secret";
        let command =
            "kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath={.data.password}";

        let mut password_encoded: Option<CommandOutput> = None;
        let mut counter = 0;
        while password_encoded.is_none() {
            password_encoded = match run_command(command, None).await {
                Ok(a) => Some(a),
                Err(e) if counter == 5 => {
                    error!("❌ Failed to get secret {}", secret_name);
                    return Err(Box::new(CommandError::new(e)));
                }
                Err(_) => {
                    counter += 1;
                    tokio::time::sleep(tokio::time::Duration::from_secs(2)).await;
                    debug!("⏳ Retrying to get secret {}", secret_name);
                    None
                }
            }
        }
        let password_encoded = password_encoded.unwrap().stdout;
        let password_decoded = BASE64_STANDARD.decode(password_encoded).map_err(|e| {
            error!("❌ Failed to decode password: {}", e);
            e
        })?;

        String::from_utf8(password_decoded).map_err(|e| {
            error!("❌ failed to convert password to string");
            e
        })?
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
    .map_err(|e| {
        error!("❌ Failed to login to argocd");
        CommandError::new(e)
    })?;

    run_command("argocd app list", None).await.map_err(|e| {
        error!("❌ Failed to run: argocd app list");
        CommandError::new(e)
    })?;

    if options.debug {
        let command = "kubectl get configmap -n argocd -o yaml argocd-cmd-params-cm argocd-cm";
        match run_command(command, None).await {
            Ok(o) => debug!(
                "🔧 ConfigMap argocd-cmd-params-cm and argocd-cm:\n{}\n{}",
                command, &o.stdout
            ),
            Err(e) => {
                error!("❌ Failed to get ConfigMaps");
                return Err(Box::new(CommandError::new(e)));
            }
        }
    }

    info!("🦑 Argo CD installed successfully");
    Ok(())
}
