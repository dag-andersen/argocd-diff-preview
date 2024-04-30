use std::path::PathBuf;
use std::process::Stdio;
use std::error::Error;
use std::{
    fs::{self, File},
    io::Write,
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

pub fn write_to_file(s: &str, file_name: &str) {
    fs::remove_file(file_name).unwrap_or_default();
    let mut f = File::create_new(file_name).expect("Unable to create file");
    f.write_all(s.as_bytes()).expect("Unable to write file");
}

pub async fn run_command(command: &str, current_dir: Option<&str>) -> Result<Output, Output> {
    let args = command.split_whitespace().collect::<Vec<&str>>();
    let output = Command::new(args[0])
        .args(&args[1..])
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

pub async fn run_command_output_to_file(
    command: &str,
    file_name: &str,
    create_folder: bool,
) -> Result<Output, Output> {
    let output = run_command(command, None).await?;
    save_to_file(&output.stdout, file_name, create_folder)
        .await
        .expect("failed to save output to file");
    Ok(output)
}

pub async fn save_to_file(
    s: &Vec<u8>,
    file_name: &str,
    create_folder: bool,
) -> Result<(), Box<dyn Error>> {
    if create_folder {
        let path = PathBuf::from(file_name);
        let parent = path.parent().unwrap();
        if !parent.is_dir() {
            fs::create_dir_all(parent).expect("Unable to create directory");
        }
    }
    fs::remove_file(file_name).unwrap_or_default();
    let mut f = File::create_new(file_name).expect("Unable to create file");
    f.write_all(s).expect("Unable to write file");

    Ok(())
}
