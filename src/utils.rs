use std::error::Error;
use std::path::PathBuf;
use std::process::{Child, Stdio};
use std::{fs, process::Command};

use crate::error::CommandOutput;

pub fn create_folder_if_not_exists(folder_name: &str) -> Result<(), Box<dyn Error>> {
    if !PathBuf::from(folder_name).is_dir() {
        fs::create_dir_all(folder_name)?;
    }
    Ok(())
}

pub fn check_if_folder_exists(folder_name: &str) -> bool {
    PathBuf::from(folder_name).is_dir()
}

pub fn run_command(
    command: &str,
    current_dir: Option<&str>,
) -> Result<CommandOutput, CommandOutput> {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    run_command_from_list(args, current_dir)
}

pub fn run_command_from_list(
    command: Vec<&str>,
    current_dir: Option<&str>,
) -> Result<CommandOutput, CommandOutput> {
    let output = Command::new(command[0])
        .args(&command[1..])
        .env(
            "ARGOCD_OPTS",
            "--port-forward --port-forward-namespace=argocd",
        )
        .current_dir(current_dir.unwrap_or("."))
        .output()
        .unwrap_or_else(|_| panic!("Failed to execute command: {}", command.join(" ")));

    match output.status.success() {
        true => Ok(CommandOutput {
            stdout: String::from_utf8_lossy(&output.stdout).to_string(),
            stderr: String::from_utf8_lossy(&output.stderr).to_string(),
        }),
        false => Err(CommandOutput {
            stdout: String::from_utf8_lossy(&output.stdout).to_string(),
            stderr: String::from_utf8_lossy(&output.stderr).to_string(),
        }),
    }
}

pub fn spawn_command(command: &str, current_dir: Option<&str>) -> Child {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    spawn_command_from_list(args, current_dir)
}

pub fn spawn_command_from_list(args: Vec<&str>, current_dir: Option<&str>) -> Child {
    Command::new(args[0])
        .args(&args[1..])
        .current_dir(current_dir.unwrap_or("."))
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .unwrap_or_else(|_| panic!("Failed to execute command: {}", args.join(" ")))
}
