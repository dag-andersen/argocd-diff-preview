use crate::{
    error::{CommandError, CommandOutput},
    utils::run_command,
};
use base64::prelude::*;
use log::{debug, error, info};
use std::error::Error;

pub struct ArgoCDOptions<'a> {
    pub version: Option<&'a str>,
    pub debug: bool,
}

pub const ARGO_CD_NAMESPACE: &str = "argocd-diff-preview";

const CONFIG_PATH: &str = "argocd-config";

pub fn create_namespace(s: &str) -> Result<(), Box<dyn Error>> {
    let already_exist = run_command(&format!("kubectl get ns {}", s)).is_ok();

    match already_exist {
        true => {
            debug!("🦑 Namespace {} already exists", s);
            Ok(())
        }
        false => {
            run_command(&format!("kubectl create ns {}", s)).map_err(|e| {
                error!("❌ Failed to create namespace {}", s);
                CommandError::new(e)
            })?;
            debug!("🦑 Namespace {} created successfully", s);
            Ok(())
        }
    }
}

pub async fn install_argo_cd(options: ArgoCDOptions<'_>) -> Result<(), Box<dyn Error>> {
    info!(
        "🦑 Installing Argo CD Helm Chart version: '{}'",
        options.version.unwrap_or("latest")
    );

    let (values_pre, values, values_post) = match std::fs::read_dir(CONFIG_PATH) {
        Ok(dir) => {
            debug!("📂 Files in folder 'argocd-config':");
            for file in dir {
                debug!("- 📄 {:?}", file.unwrap().file_name());
            }
            let values_pre = std::fs::metadata(format!("{}/values-pre.yaml", CONFIG_PATH))
                .is_ok()
                .then_some(format!("-f {}/values-pre.yaml", CONFIG_PATH));
            let values = std::fs::metadata(format!("{}/values.yaml", CONFIG_PATH))
                .is_ok()
                .then_some(format!("-f {}/values.yaml", CONFIG_PATH));
            let values_post = std::fs::metadata(format!("{}/values-post.yaml", CONFIG_PATH))
                .is_ok()
                .then_some(format!("-f {}/values-post.yaml", CONFIG_PATH));
            (values, values_pre, values_post)
        }
        Err(_e) => {
            info!("📂 Folder '{}' doesn't exist. Installing Argo CD Helm Chart with default configuration", CONFIG_PATH);
            (None, None, None)
        }
    };

    // add argo repo to helm
    run_command("helm repo add argo https://argoproj.github.io/argo-helm").map_err(|e| {
        error!("❌ Failed to add argo repo");
        CommandError::new(e)
    })?;

    let helm_install_command = format!(
        "helm install argocd argo/argo-cd -n {} {} {} {}",
        ARGO_CD_NAMESPACE,
        values.unwrap_or_default(),
        values_post.unwrap_or_default(),
        options
            .version
            .map(|a| format!("--version {}", a))
            .unwrap_or_default(),
    );

    debug!("Installing Argo CD Helm Chart...");
    run_command(&helm_install_command).map_err(|e| {
        error!("❌ Failed to install Argo CD");
        CommandError::new(e)
    })?;

    info!("🦑 Waiting for Argo CD to start...");

    // wait for argocd-server to be ready
    match run_command(
        &format!(
            "kubectl wait --for=condition=available deployment/argocd-server -n {} --timeout=300s",
            ARGO_CD_NAMESPACE
        )
    ) {
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
        let command = &format!(
            "kubectl -n {} get secret {} -o jsonpath={{.data.password}}",
            ARGO_CD_NAMESPACE,
            secret_name
        );

        let mut password_encoded: Option<CommandOutput> = None;
        let mut counter = 0;
        while password_encoded.is_none() {
            password_encoded = match run_command(command) {
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
        let password_decoded = BASE64_STANDARD.decode(password_encoded).inspect_err(|e| {
            error!("❌ Failed to decode password: {}", e);
        })?;

        String::from_utf8(password_decoded).inspect_err(|_e| {
            error!("❌ failed to convert password to string");
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

    run_command(&format!(
        "argocd login localhost:8080 --insecure --username {} --password {}",
        username, password
    ))
    .map_err(|e| {
        error!("❌ Failed to login to argocd");
        CommandError::new(e)
    })?;

    run_command("argocd app list").map_err(|e| {
        error!("❌ Failed to run: argocd app list");
        CommandError::new(e)
    })?;

    if options.debug {
        let command = format!(
            "kubectl get configmap -n {} -o yaml argocd-cmd-params-cm argocd-cm",
            ARGO_CD_NAMESPACE
        );
        match run_command(&command) {
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

    // Add extra permissions to the default AppProject
    let _ = run_command("argocd proj add-source-namespace default *", None);

    info!("🦑 Argo CD installed successfully");
    Ok(())
}
