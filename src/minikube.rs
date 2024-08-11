use crate::{run_command, utils::spawn_command};
use log::{error, info};
use std::error::Error;

pub async fn is_installed() -> bool {
    run_command("which minikube", None).await.is_ok()
}

pub async fn create_cluster() -> Result<(), Box<dyn Error>> {
    // check if docker is running
    match run_command("docker ps", None).await {
        Ok(_) => (),
        Err(e) => {
            error!("❌ Docker is not running");
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    }

    info!("🚀 Creating cluster...");
    match run_command("minikube delete", None).await {
        Ok(o) => o,
        Err(e) => {
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    };

    match run_command("minikube start", None).await {
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

pub fn delete_cluster() {
    info!("💥 Deleting cluster...");
    spawn_command("minikube delete", None);
}
