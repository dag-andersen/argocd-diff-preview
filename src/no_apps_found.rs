use log::info;
use std::error::Error;
use std::fs;

use crate::Selector;

// Message to show when no applications were found

pub async fn write_message(
    output_folder: &str,
    selector: &Option<Vec<Selector>>,
    changed_files: &Option<Vec<String>>,
) -> Result<(), Box<dyn Error>> {

    let selector_string = |s: &Vec<Selector>| {
        s.iter()
            .map(|s| s.to_string())
            .collect::<Vec<String>>()
            .join(",")
    };

    let markdown = match (selector, changed_files) {
        (Some(s), Some(f)) => {
            let message = format!(
                "🔍 Found 0 Applications that matches '{}' and watches these files: '{}'",
                selector_string(s), f.join(", "));
            print_diff(&message)
        }
        (Some(s), None) => {
            let message = format!(
                "🔍 Found 0 Applications after selecting applications matching '{}'",
                selector_string(s)
            );
            print_diff(&message)
        }
        (None, Some(f)) => {
            let message = format!(
                "🔍 Found 0 Applications that watched these files: '{}'",
                f.join(", ")
            );
            print_diff(&message)
        }
        (None, None) => {
            let message = "🔍 Found 0 Applications".to_string();
            print_diff(&message)
        }
    };

    let markdown_path = format!("{}/diff.md", output_folder);
    fs::write(&markdown_path, markdown)?;

    info!("🙏 Please check the {} file for differences", markdown_path);

    Ok(())
}

const MARKDOWN_TEMPLATE: &str = r#"
## Argo CD Diff Preview

%message%
"#;

fn print_diff(message: &str) -> String {
    MARKDOWN_TEMPLATE
        .replace("%message%", message)
        .trim_start()
        .to_string()
}
