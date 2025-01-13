use log::debug;
use std::error::Error;
use std::io::Write;
use std::path::PathBuf;
use std::process::{Child, Stdio};
use std::{fs, process::Command};

use crate::error::CommandOutput;

#[derive(Default)]
pub struct CommandConfig<'a> {
    pub command: &'a str,
    pub current_dir: Option<&'a str>,
    pub stdin: Option<&'a str>,
    pub envs: Option<Vec<StringPair<'a>>>,
}

pub struct StringPair<'a> {
    pub key: &'a str,
    pub value: &'a str,
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

pub fn run_simple_command(command: &str) -> Result<CommandOutput, CommandOutput> {
    run_command(CommandConfig {
        command,
        ..Default::default()
    })
}

pub fn run_command(config: CommandConfig) -> Result<CommandOutput, CommandOutput> {
    let args = config.command.split_whitespace().collect::<Vec<&str>>();
    debug!("Running shell command: {}", config.command);

    let mut child = match config.envs {
        Some(envs) => envs
            .iter()
            .fold(Command::new(args[0]), |mut output, env_var| {
                output.env(env_var.key, env_var.value);
                output
            }),
        None => Command::new(args[0]),
    }
    .args(&args[1..])
    .current_dir(config.current_dir.unwrap_or("."))
    .stdin(Stdio::piped())
    .stdout(Stdio::piped())
    .stderr(Stdio::piped())
    .spawn()
    .unwrap_or_else(|_| panic!("Failed to execute command: {}", config.command));

    if let Some(stdin_str) = config.stdin {
        let mut stdin = child
            .stdin
            .take()
            .unwrap_or_else(|| panic!("Failed to open stdin for command: {}", config.command));
        stdin
            .write_all(stdin_str.as_bytes())
            .unwrap_or_else(|_| panic!("Failed to write to stdin for command: {}", config.command));
    }

    let output = child
        .wait_with_output()
        .unwrap_or_else(|_| panic!("Failed to wait for output for command: {}", config.command));

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

fn spawn_command_from_list(args: Vec<&str>, current_dir: Option<&str>) -> Child {
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
