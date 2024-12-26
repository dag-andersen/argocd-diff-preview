use crate::{
    error::{CommandError, CommandOutput},
    utils::{run_command, run_command_with_envs, StringPair},
};
use base64::prelude::*;
use log::{debug, error, info};
use std::error::Error;

pub struct ArgoCDInstallation {
    namespace: String,
    version: Option<String>,
    config_path: String,
}

impl ArgoCDInstallation {
    pub fn new(
        namespace: &str,
        version: Option<String>,
        custom_config_path: Option<String>,
    ) -> Self {
        Self {
            namespace: namespace.to_string(),
            version,
            config_path: custom_config_path.unwrap_or("argocd-config".to_string()),
        }
    }

    pub async fn install_argo_cd(&self, debug: bool) -> Result<(), Box<dyn Error>> {
        info!(
            "🦑 Installing Argo CD Helm Chart version: '{}'",
            self.version.clone().unwrap_or("latest".to_string())
        );

        let (values, values_override) = match std::fs::read_dir(&self.config_path) {
            Ok(dir) => {
                debug!("📂 Files in folder '{}':", self.config_path);
                for file in dir {
                    debug!("- 📄 {:?}", file.unwrap().file_name());
                }
                let values_exist = std::fs::metadata(format!("{}/values.yaml", self.config_path))
                    .is_ok()
                    .then_some(format!("-f {}/values.yaml", self.config_path));
                let values_override_exist =
                    std::fs::metadata(format!("{}/values-override.yaml", self.config_path))
                        .is_ok()
                        .then_some(format!("-f {}/values-override.yaml", self.config_path));
                (values_exist, values_override_exist)
            }
            Err(_e) => {
                info!("📂 Folder '{}' doesn't exist. Installing Argo CD Helm Chart with default configuration", self.config_path);
                (None, None)
            }
        };

        // add argo repo to helm
        run_command("helm repo add argo https://argoproj.github.io/argo-helm").map_err(|e| {
            error!("❌ Failed to add argo repo");
            CommandError::new(e)
        })?;

        let helm_install_command = format!(
            "helm install argocd argo/argo-cd -n {} {} {} {}",
            self.namespace,
            values.unwrap_or_default(),
            values_override.unwrap_or_default(),
            self.version
                .clone()
                .map(|a| format!("--version {}", a))
                .unwrap_or_default(),
        );

        run_command(&helm_install_command).map_err(|e| {
            error!("❌ Failed to install Argo CD");
            CommandError::new(e)
        })?;

        info!("🦑 Waiting for Argo CD to start...");

        // wait for argocd-server to be ready
        match run_command(&format!(
            "kubectl wait --for=condition=available deployment/argocd-server -n {} --timeout=300s",
            self.namespace
        )) {
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
                self.namespace, secret_name
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
            let password_decoded = BASE64_STANDARD.decode(password_encoded).map_err(|e| {
                error!("❌ Failed to decode password: {}", e);
                e
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

        self.run_argocd_command(&format!(
            "argocd login localhost:8080 --insecure --username {} --password {}",
            username, password
        ))
        .map_err(|e| {
            error!("❌ Failed to login to argocd");
            CommandError::new(e)
        })?;

        self.run_argocd_command("argocd app list").map_err(|e| {
            error!("❌ Failed to run: argocd app list");
            CommandError::new(e)
        })?;

        if debug {
            let command = format!(
                "kubectl get configmap -n {} -o yaml argocd-cmd-params-cm argocd-cm",
                self.namespace
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

        info!("🦑 Argo CD installed successfully");
        Ok(())
    }

    fn run_argocd_command(&self, command: &str) -> Result<CommandOutput, CommandOutput> {
        run_command_with_envs(
            command,
            Some(vec![StringPair {
                key: "ARGOCD_OPTS".to_string(),
                value: format!("--port-forward --port-forward-namespace={}", self.namespace),
            }]),
        )
    }

    pub fn get_manifests(&self, app_name: &str) -> Result<CommandOutput, CommandOutput> {
        self.run_argocd_command(&format!("argocd app manifests {}", app_name))
    }

    pub fn refresh_app(&self, app_name: &str) -> Result<CommandOutput, CommandOutput> {
        self.run_argocd_command(&format!("argocd app get {} --refresh", app_name))
    }
}
