use crate::{run_command, utils::spawn_command};
use log::{error, info};
use std::error::Error;

pub async fn is_installed() -> bool {
    run_command("which kind", None).await.is_ok()
}

pub async fn create_cluster(cluster_name: &str) -> Result<(), Box<dyn Error>> {
    // check if docker is running
    match run_command("docker ps", None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Docker is not running");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    info!("🚀 Creating cluster...");
    match run_command(
        &format!("kind delete cluster --name {}", cluster_name),
        None,
    )
    .await
    {
        Ok(_) => (),
        Err(e) => {
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    };

    match run_command(
        &format!("kind create cluster --name {}", cluster_name),
        None,
    )
    .await
    {
        Ok(_) => {
            info!("🚀 Cluster created successfully");
            Ok(())
        }
        Err(e) => {
            error!("❌ Failed to Create cluster");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }
}

pub fn delete_cluster(cluster_name: &str) {
    info!("💥 Deleting cluster...");
    spawn_command(
        &format!("kind delete cluster --name {}", cluster_name),
        None,
    );
}
