use std::path::PathBuf;
use std::process::Stdio;
use std::{
    fs,
    process::{Command, Output},
};

pub fn create_folder_if_not_exists(folder_name: &str) {
    if !PathBuf::from(folder_name).is_dir() {
        fs::create_dir(folder_name).expect("Unable to create directory");
    }
}

pub fn check_if_folder_exists(folder_name: &str) -> bool {
    PathBuf::from(folder_name).is_dir()
}

pub async fn run_command(command: &str, current_dir: Option<&str>) -> Result<Output, Output> {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    let output = Command::new(args[0])
        .args(&args[1..])
        .env(
            "ARGOCD_OPTS",
            "--port-forward --port-forward-namespace=argocd",
        )
        .current_dir(current_dir.unwrap_or("."))
        .output()
        .expect(format!("Failed to execute command: {}", command).as_str());

    if !output.status.success() {
        return Err(output);
    }

    Ok(output)
}

pub fn spawn_command(command: &str, current_dir: Option<&str>) {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    Command::new(args[0])
        .args(&args[1..])
        .current_dir(current_dir.unwrap_or("."))
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .expect(format!("Failed to execute command: {}", command).as_str());
}
