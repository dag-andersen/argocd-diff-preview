use crate::{
    error::{CommandError, CommandOutput},
    utils::{self, run_command, run_command_with_envs, StringPair},
};
use base64::prelude::*;
use log::{debug, error, info};
use serde_yaml::Value;
use std::error::Error;

pub struct ArgoCDInstallation {
    pub namespace: String,
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

        let (values_pre, values, values_post) = match std::fs::read_dir(&self.config_path) {
            Ok(dir) => {
                debug!("📂 Files in folder 'argocd-config':");
                for file in dir {
                    debug!("- 📄 {:?}", file.unwrap().file_name());
                }
                let values_pre = std::fs::metadata(format!("{}/values-pre.yaml", self.config_path))
                    .is_ok()
                    .then_some(format!("-f {}/values-pre.yaml", self.config_path));
                let values = std::fs::metadata(format!("{}/values.yaml", self.config_path))
                    .is_ok()
                    .then_some(format!("-f {}/values.yaml", self.config_path));
                let values_post =
                    std::fs::metadata(format!("{}/values-post.yaml", self.config_path))
                        .is_ok()
                        .then_some(format!("-f {}/values-post.yaml", self.config_path));
                (values, values_pre, values_post)
            }
            Err(_e) => {
                info!(
                    "📂 Folder '{}' doesn't exist. Installing Argo CD Helm Chart with default configuration",
                    self.config_path
                );
                (None, None, None)
            }
        };

        // add argo repo to helm
        run_command("helm repo add argo https://argoproj.github.io/argo-helm").map_err(
            |e| {
                error!("❌ Failed to add argo repo");
                CommandError::new(e)
            },
        )?;

        // helm update
        run_command("helm repo update").map_err(|e| {
            error!("❌ Failed to update helm repo");
            CommandError::new(e)
        })?;

        let helm_install_command = format!(
            "helm install argocd argo/argo-cd -n {} {} {} {} {}",
            self.namespace,
            values.unwrap_or_default(),
            values_pre.unwrap_or_default(),
            values_post.unwrap_or_default(),
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
                return Err("Failed to wait for argocd-server".into());
            }
        }

        match run_command("helm list -A -o yaml") {
            Ok(o) => {
                let as_yaml: Value = serde_yaml::from_str(&o.stdout)?;
                match (
                    as_yaml[0]["chart"].as_str(),
                    as_yaml[0]["app_version"].as_str(),
                ) {
                    (Some(chart_version), Some(app_version)) => {
                        info!(
                            "🦑 Installed Chart version: '{}' and App version: '{}'",
                            chart_version, app_version
                        );
                    }
                    _ => {
                        error!("❌ Failed to get chart version");
                    }
                }
            }
            Err(e) => {
                error!("❌ Failed to list helm charts");
                return Err(Box::new(CommandError::new(e)));
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
                        utils::sleep(2).await;
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

        utils::sleep(5).await;

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

        // Add extra permissions to the default AppProject
        let _ = self.run_argocd_command("argocd proj add-source-namespace default *");

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

    pub fn appset_generate(&self, app_set_path: &str) -> Result<CommandOutput, CommandOutput> {
        self.run_argocd_command(&format!("argocd appset generate {} -o yaml", app_set_path))
    }
}
