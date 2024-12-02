use crate::{
    error::CommandError,
    utils::{run_command, spawn_command},
};
use log::{error, info};
use std::error::Error;

pub fn is_installed() -> bool {
    run_command("which minikube", None).is_ok()
}

pub fn create_cluster() -> Result<(), Box<dyn Error>> {
    // check if docker is running
    run_command("docker ps", None).map_err(|o| {
        error!("❌ Docker is not running");
        CommandError::new(o)
    })?;

    info!("🚀 Creating cluster...");
    run_command("minikube delete", None).map_err(CommandError::new)?;

    run_command("minikube start", None)
        .map(|_| {
            info!("🚀 Cluster created successfully");
            Ok(())
        })
        .map_err(|e| {
            error!("❌ Failed to create cluster");
            CommandError::new(e)
        })?
}

pub fn cluster_exists() -> bool {
    run_command("minikube status", None).is_ok()
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
