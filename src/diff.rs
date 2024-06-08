use log::{debug, info};
use std::fs;
use std::{error::Error, process::Output};

use crate::utils::run_command;
use crate::Branch;

pub async fn generate_diff(
    output_folder: &str,
    base_branch_name: &str,
    target_branch_name: &str,
    diff_ignore: Option<String>,
    line_count: Option<usize>,
    max_char_count: Option<usize>,
) -> Result<(), Box<dyn Error>> {
    let max_diff_message_char_count = max_char_count.unwrap_or(65536);

    info!(
        "🔮 Generating diff between {} and {}",
        base_branch_name, target_branch_name
    );

    let patterns_to_ignore = match diff_ignore {
        Some(s) => format!("--ignore-matching-lines {}", s),
        None => "".to_string(),
    };

    let parse_diff_output = |output: Result<Output, Output>| -> String {
        let o = match output {
            Err(e) if !e.stderr.is_empty() => panic!(
                "Error running diff command with error: {}",
                String::from_utf8_lossy(&e.stderr)
            ),
            Ok(e) => String::from_utf8_lossy(&e.stdout).trim_end().to_string(),
            Err(e) => String::from_utf8_lossy(&e.stdout).trim_end().to_string(),
        };
        if o.trim().is_empty() {
            return "No changes found".to_string();
        } else {
            o
        }
    };

    let summary_diff_command = format!(
        "git --no-pager diff --compact-summary --no-index {} {} {}",
        patterns_to_ignore,
        Branch::Base,
        Branch::Target
    );

    debug!(
        "Getting summary diff with command: {}",
        summary_diff_command
    );

    let summary_as_string =
        parse_diff_output(run_command(&summary_diff_command, Some(output_folder)).await);

    let diff_command = &format!(
        "git --no-pager diff --no-prefix -U{} --no-index {} {} {}",
        line_count.unwrap_or(10),
        patterns_to_ignore,
        Branch::Base,
        Branch::Target
    );

    debug!("Getting diff with command: {}", diff_command);

    let diff_as_string = parse_diff_output(run_command(diff_command, Some(output_folder)).await);

    let remaining_max_chars =
        max_diff_message_char_count - markdown_template_length() - summary_as_string.len();

    let warning_message = &format!(
        "\n\n ⚠️⚠️⚠️ Diff is too long. Truncated to {} characters. This can be adjusted with the `--max-diff-length` flag",
        max_diff_message_char_count
    );

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

    let markdown = print_diff(&summary_as_string, &diff_truncated);

    let markdown_path = format!("{}/diff.md", output_folder);
    fs::write(&markdown_path, &markdown)?;

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
        .trim_start()
        .to_string()
}
