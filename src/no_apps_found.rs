use std::error::Error;
use std::fs;

use crate::Selector;

// Message to show when no applications were found

pub async fn write_message(
    output_folder: &str,
    selector: &Option<Vec<Selector>>,
    changed_files: &Option<Vec<String>>,
) -> Result<(), Box<dyn Error>> {
    let message = get_message(selector, changed_files);

    let markdown = generate_markdown(&message);
    let markdown_path = format!("{}/diff.md", output_folder);
    fs::write(markdown_path, markdown)?;

    Ok(())
}

const MARKDOWN_TEMPLATE: &str = r#"
## Argo CD Diff Preview

%message%
"#;

fn generate_markdown(message: &str) -> String {
    MARKDOWN_TEMPLATE
        .replace("%message%", message)
        .trim_start()
        .to_string()
}

pub fn get_message(
    selector: &Option<Vec<Selector>>,
    changed_files: &Option<Vec<String>>,
) -> String {
    let selector_string = |s: &Vec<Selector>| {
        s.iter()
            .map(|s| s.to_string())
            .collect::<Vec<String>>()
            .join(",")
    };

    match (selector, changed_files) {
        (Some(s), Some(f)) => format!(
            "Found no changed Applications that matched '{}' and watched these files: '{}'",
            selector_string(s),
            f.join("`, `")
        ),
        (Some(s), None) => format!(
            "Found no changed Applications that matched '{}'",
            selector_string(s)
        ),
        (None, Some(f)) => format!(
            "Found no changed Applications that watched these files: '{}'",
            f.join("`, `")
        ),
        (None, None) => "Found no Applications".to_string(),
    }
}
