use crate::{
    error::CommandError,
    utils::{run_command, spawn_command},
};
use log::{debug, error, info};
use std::error::Error;

pub fn is_installed() -> bool {
    run_command("which kind").is_ok()
}

pub fn create_cluster(cluster_name: &str) -> Result<(), Box<dyn Error>> {
    // check if docker is running
    run_command("docker ps").map_err(|o| {
        error!("❌ Docker is not running");
        CommandError::new(o)
    })?;

    info!("🚀 Creating cluster...");
    run_command(&format!("kind delete cluster --name {}", cluster_name))
        .map_err(CommandError::new)?;

    run_command(&format!("kind create cluster --name {}", cluster_name))
        .map(|_| {
            info!("🚀 Cluster created successfully");
            Ok(())
        })
        .map_err(|e| {
            error!("❌ Failed to create cluster");
            CommandError::new(e)
        })?
}

pub fn cluster_exists(cluster_name: &str) -> bool {
    match run_command("kind get clusters") {
        Ok(o) if o.stdout.trim() == cluster_name => true,
        Ok(o) => {
            debug!("❌ Cluster '{}' not found in: {}", cluster_name, o.stdout);
            false
        }
        Err(_) => false,
    }
}

pub fn delete_cluster(cluster_name: &str, wait: bool) {
    info!("💥 Deleting cluster...");
    let mut child = spawn_command(
        &format!("kind delete cluster --name {}", cluster_name),
        None,
    );
    if wait {
        match child.wait() {
            Ok(_) => info!("💥 Cluster deleted successfully"),
            Err(e) => error!("❌ Failed to delete cluster: {}", e),
        }
    }
}
