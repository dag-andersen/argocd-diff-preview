use crate::utils::{run_command, spawn_command, CommandError};
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

pub async fn cluster_exists() -> bool {
    run_command("minikube status", None).await.is_ok()
}

pub fn delete_cluster(wait: bool) {
    info!("💥 Deleting cluster...");
    let mut child = spawn_command("minikube delete", None);
    if wait {
        match child.wait() {
            Ok(_) => info!("💥 Cluster deleted successfully"),
            Err(e) => error!("❌ Failed to delete cluster: {}", e),
        }
    }
}
