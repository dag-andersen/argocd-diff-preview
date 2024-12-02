use crate::branch::BranchType;
use crate::utils::run_command;
use crate::Branch;
use log::{debug, info};
use std::{fs, vec};
use std::error::Error;

pub fn generate_diff(
    output_folder: &str,
    base_branch: &Branch,
    target_branch: &Branch,
    diff_ignore: Option<String>,
    max_char_count: Option<usize>,
) -> Result<(), Box<dyn Error>> {
    let max_diff_message_char_count = max_char_count.unwrap_or(65536);

    info!(
        "🔮 Generating diff between {} and {}",
        base_branch.name, target_branch.name
    );

    let patterns_to_ignore = match diff_ignore {
        Some(s) => format!("--ignore-matching-lines {}", s),
        None => "".to_string(),
    };

    // let summary_diff_command = format!(
    //     "git --no-pager diff --compact-summary --no-index {} {} {}",
    //     patterns_to_ignore,
    //     Branch::Base,
    //     Branch::Target
    // );

    // debug!(
    //     "Getting summary diff with command: {}",
    //     summary_diff_command
    // );

    // let summary_as_string =
    //     parse_diff_output(run_command(&summary_diff_command, Some(output_folder)).await);

    let header = |header: &str| -> String {
        format!("### Argo CD Application: {}\n", header)
    };

    let diff_command = &format!(
        "dyff between -o github --omit-header --detect-kubernetes=false {} {}",
        BranchType::Base,
        BranchType::Target,
    );

    debug!("Getting diff with command: {}", diff_command);

    // loop over all folders in the output folder and run the diff command

    let mut outputs: Vec<String> = vec![];

    let search_folder = output_folder;
    debug!("Searching for folders in {}", output_folder);
    for entry in fs::read_dir(search_folder)? {
        let entry = entry?;
        let path = entry.path();
        debug!("Found entry: {:?}", path);
        if path.is_dir() {
            let folder_name = path.to_str().unwrap();
            let output_string: String = match run_command(diff_command, Some(folder_name)) {
                Ok(s) if !s.stdout.is_empty() => s.stdout,
                _ => continue,
            };
            outputs.push(header(folder_name));
            outputs.push(output_string);
        }
        debug!("Finished processing entry: {:?}", path);
    }

    let remaining_max_chars =
        max_diff_message_char_count - markdown_template_length();

    let warning_message = &format!(
        "\n\n ⚠️⚠️⚠️ Diff is too long. Truncated to {} characters. This can be adjusted with the `--max-diff-length` flag",
        max_diff_message_char_count
    );

    let diff_as_string = outputs.join("\n\n");

    let diff_truncated = match remaining_max_chars {
        remaining if remaining > diff_as_string.len() => diff_as_string, // No need to truncate
        remaining if remaining > warning_message.len() => {
            info!(
                "🚨 Diff is too long. Truncating message to {} characters",
                max_diff_message_char_count
            );
            let last_diff_char = remaining - warning_message.len();
            diff_as_string[..last_diff_char].to_string() + warning_message
        }
        _ => return Err("Diff is too long and cannot be truncated. Increase the max length with `--max-diff-length`".into())
    };

    let markdown = print_diff(&"", &diff_truncated);

    let markdown_path = format!("{}/diff.md", output_folder);
    fs::write(&markdown_path, markdown)?;

    info!("🙏 Please check the {} file for differences", markdown_path);

    Ok(())
}

const MARKDOWN_TEMPLATE: &str = r#"
## Argo CD Diff Preview

Summary:
```bash
%summary%
```

<details>
<summary>Diff:</summary>
<br>

```diff
%diff%
```

</details>
"#;

fn markdown_template_length() -> usize {
    MARKDOWN_TEMPLATE
        .replace("%summary%", "")
        .replace("%diff%", "")
        .len()
}

fn print_diff(summary: &str, diff: &str) -> String {
    MARKDOWN_TEMPLATE
        .replace("%summary%", summary)
        .replace("%diff%", diff)
        .replace("&", "/")
        .trim_start()
        .to_string()
}
