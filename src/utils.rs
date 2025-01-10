use log::debug;
use std::error::Error;
use std::path::PathBuf;
use std::process::{Child, Stdio};
use std::{fs, process::Command};

use crate::error::CommandOutput;

pub struct StringPair {
    pub key: String,
    pub value: String,
}

pub fn create_folder_if_not_exists(folder_name: &str) -> Result<(), Box<dyn Error>> {
    if !PathBuf::from(folder_name).is_dir() {
        fs::create_dir(folder_name)?;
    }
    Ok(())
}

pub fn check_if_folder_exists(folder_name: &str) -> bool {
    PathBuf::from(folder_name).is_dir()
}

pub fn delete_folder(folder_name: &str) -> Result<(), Box<dyn Error>> {
    if check_if_folder_exists(folder_name) {
        fs::remove_dir_all(folder_name)?;
    }
    Ok(())
}

pub fn run_command(command: &str) -> Result<CommandOutput, CommandOutput> {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    run_command_from_list(args, None, None)
}

pub fn run_command_with_envs(
    command: &str,
    envs: Option<Vec<StringPair>>,
) -> Result<CommandOutput, CommandOutput> {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    run_command_from_list(args, None, envs)
}

pub fn run_command_in_dir(
    command: &str,
    current_dir: &str,
) -> Result<CommandOutput, CommandOutput> {
    debug!("Running command: {}", command);
    let args = command.split_whitespace().collect::<Vec<&str>>();
    run_command_from_list(args, Some(current_dir), None)
}

pub fn run_command_from_list(
    command: Vec<&str>,
    current_dir: Option<&str>,
    envs: Option<Vec<StringPair>>,
) -> Result<CommandOutput, CommandOutput> {
    debug!("Running shell command: {}", command.join(" "));

    let output = match envs {
        Some(envs) => envs
            .iter()
            .fold(Command::new(command[0]), |mut output, env_var| {
                output.env(&env_var.key, &env_var.value);
                output
            }),
        None => Command::new(command[0]),
    }
    .args(&command[1..])
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
    debug!("Spawning command: {}", args.join(" "));
    Command::new(args[0])
        .args(&args[1..])
        .current_dir(current_dir.unwrap_or("."))
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .spawn()
        .unwrap_or_else(|_| panic!("Failed to execute command: {}", args.join(" ")))
}

pub fn write_to_file(file_name: &str, content: &str) -> Result<(), Box<dyn Error>> {
    debug!("Writing to file: {}", file_name);
    fs::write(file_name, content)?;
    Ok(())
}

pub async fn sleep(seconds: u64) {
    debug!("Sleeping for {} seconds", seconds);
    tokio::time::sleep(tokio::time::Duration::from_secs(seconds)).await;
}
