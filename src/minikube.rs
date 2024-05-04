use log::{error, info};
use std::error::Error;

use crate::{run_command, utils::spawn_command};

pub async fn is_installed() -> bool {
    match run_command("which minikube", None).await {
        Ok(_) => true,
        Err(_) => false,
    }
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
    match run_command(&format!("minikube delete"), None).await {
        Ok(o) => o,
        Err(e) => {
            panic!("error: {}", String::from_utf8_lossy(&e.stderr))
        }
    };

    match run_command(&format!("minikube start"), None).await {
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
    spawn_command(&format!("minikube delete"), None);
}
