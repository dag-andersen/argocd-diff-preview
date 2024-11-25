use crate::{
    run_command,
    utils::{spawn_command, CommandError},
};
use log::{error, info};
use std::error::Error;

pub async fn is_installed() -> bool {
    run_command("which minikube", None).await.is_ok()
}

pub async fn create_cluster() -> Result<(), Box<dyn Error>> {
    // check if docker is running
    run_command("docker ps", None).await.map_err(|o| {
        error!("❌ Docker is not running");
        CommandError::new(o)
    })?;

    info!("🚀 Creating cluster...");
    run_command("minikube delete", None)
        .await
        .map_err(CommandError::new)?;

    run_command("minikube start", None)
        .await
        .map(|_| {
            info!("🚀 Cluster created successfully");
            Ok(())
        })
        .map_err(|e| {
            error!("❌ Failed to create cluster");
            CommandError::new(e)
        })?
}

pub fn delete_cluster(wait: bool) {
    info!("💥 Deleting cluster...");
    let mut child = spawn_command("minikube delete", None);
    if wait {
        child.wait().unwrap();
    }
}
