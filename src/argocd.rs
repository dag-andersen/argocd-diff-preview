use crate::run_command;
use base64::prelude::*;
use log::{debug, error, info};
use std::{
    error::Error,
    io::Write,
    process::{Command, Output, Stdio},
};

static ARGOCD_CONFIGMAPS: &str = r#"
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cm
    app.kubernetes.io/part-of: argocd
data:
  kustomize.buildOptions: "%kube_build_options%"
  admin.enabled: "true"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
  labels:
    app.kubernetes.io/name: argocd-cmd-params-cm
    app.kubernetes.io/part-of: argocd
data:
  reposerver.git.request.timeout: "150s"
  reposerver.parallelism.limit: "300"
"#;

pub struct ArgoCDOptions<'a> {
    pub version: Option<&'a str>,
    pub kube_build_options: Option<&'a str>,
}

pub async fn install_argo_cd(options: ArgoCDOptions<'_>) -> Result<(), Box<dyn Error>> {
    let version = options.version.unwrap_or("stable");
    info!("🦑 Installing Argo CD version: '{}'", version);

    match run_command("kubectl create ns argocd", None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Failed to create namespace argocd");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    // Install Argo CD
    let install_url = format!(
        "https://raw.githubusercontent.com/argoproj/argo-cd/{}/manifests/install.yaml",
        version
    );
    match run_command(&format!("kubectl -n argocd apply -f {}", install_url), None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Failed to install Argo CD");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }
    info!("🦑 Waiting for Argo CD to start...");

    // apply argocd-cmd-params-cm
    let mut child = Command::new("kubectl")
        .arg("apply")
        .arg("-f")
        .arg("-")
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("Failed to apply argocd-cmd-params-cm");

    let config_map = ARGOCD_CONFIGMAPS.replace("%kube_build_options%", &options.kube_build_options.unwrap_or(""));

    let child_stdin = child.stdin.as_mut().expect("Failed to open stdin");
    child_stdin
        .write_all(config_map.as_bytes())
        .expect("Failed to write to stdin");
    child.wait_with_output().expect("Failed to read stdout");

    run_command(
        "kubectl -n argocd rollout restart deploy argocd-repo-server",
        None,
    )
    .await
    .expect("Failed to restart argocd-repo-server");
    run_command(
        "kubectl -n argocd rollout status deployment/argocd-repo-server --timeout=60s",
        None,
    )
    .await
    .expect("Failed to wait for argocd-repo-server");

    // wait for argocd-server to be ready
    run_command(
        "kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=300s",
        None,
    )
    .await
    .expect("failed to wait for argocd-server");

    info!("🦑 Argo CD is now available");

    info!("🦑 Logging in to Argo CD through CLI...");

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

    let password_encoded_unwrapped = password_encoded.unwrap();

    let password_decoded_vec = BASE64_STANDARD
        .decode(password_encoded_unwrapped.stdout)
        .expect("failed to decode password");
    let password =
        String::from_utf8(password_decoded_vec).expect("failed to convert password to string");

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

    info!("🦑 Argo CD installed successfully");
    Ok(())
}
