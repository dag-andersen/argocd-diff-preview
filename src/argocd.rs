use base64::prelude::*;
use std::{
    error::Error,
    io::Write,
    process::{Command, Stdio},
};

use log::{debug, error, info};

use crate::run_command;

static ARGOCD_CMD_PARAMS_CM: &str = r#"
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/name: argocd-cmd-params-cm
    app.kubernetes.io/part-of: argocd
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  reposerver.git.request.timeout: "150s"
  reposerver.parallelism.limit: "300"
"#;

pub async fn install_argo_cd(version: Option<&str>) -> Result<(), Box<dyn Error>> {
    info!("🦑 Installing Argo CD...");

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
        version.unwrap_or("stable")
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
        .expect("failed to execute process");

    let child_stdin = child.stdin.as_mut().expect("Failed to open stdin");
    child_stdin
        .write_all(ARGOCD_CMD_PARAMS_CM.as_bytes())
        .expect("Failed to write to stdin");
    child.wait_with_output().expect("Failed to read stdout");

    run_command(
        "kubectl -n argocd rollout restart deploy argocd-repo-server",
        None,
    )
    .await
    .expect("failed to restart argocd-repo-server");
    run_command(
        "kubectl -n argocd rollout status deployment/argocd-repo-server --timeout=60s",
        None,
    )
    .await
    .expect("failed to wait for argocd-repo-server");

    info!("🦑 Logging in to Argo CD through CLI...");
    debug!("Port-forwarding Argo CD server...");

    // port-forward Argo CD server
    Command::new("kubectl")
        .arg("-n")
        .arg("argocd")
        .arg("port-forward")
        .arg("service/argocd-server")
        .arg("8080:443")
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect("failed to execute process");

    debug!("Getting initial admin password...");
    let secret_name = "argocd-initial-admin-secret";
    let command =
        "kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath={.data.password}";
    debug!("Running command: {}", command);
    let password_encoded = match run_command(&command, None).await {
        Ok(a) => a,
        Err(e) => {
            error!("❌ Failed to get secret {}", secret_name);
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    };

    let password_decoded_vec = BASE64_STANDARD
        .decode(password_encoded.stdout)
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
