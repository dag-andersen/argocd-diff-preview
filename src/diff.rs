use log::info;
use std::fs;
use std::{error::Error, process::Output};

use crate::utils::run_command;
use crate::Branch;

pub async fn generate_diff(
    output_folder: &str,
    base_branch_name: &str,
    target_branch_name: &str,
    diff_ignore: Option<String>,
) -> Result<(), Box<dyn Error>> {
    info!(
        "🔮 Generating diff between {} and {}",
        base_branch_name, target_branch_name
    );

    let list_of_patterns_to_ignore = match diff_ignore {
        Some(s) => s
            .split(",")
            .map(|s| format!("--ignore-matching-lines={}", s))
            .collect::<Vec<String>>()
            .join(" "),
        None => "".to_string(),
    };

    let parse_diff_output = |output: Result<Output, Output>| -> String {
        match output {
            Ok(_) => "No changes found".to_string(),
            Err(e) => String::from_utf8_lossy(&e.stdout).to_string(),
        }
    };

    let summary_as_string = parse_diff_output(
        run_command(
            &format!(
                "git --no-pager diff --compact-summary --no-index {} {} {}",
                list_of_patterns_to_ignore,
                Branch::Base,
                Branch::Target
            ),
            Some(output_folder),
        )
        .await,
    );

    let diff_as_string = parse_diff_output(
        run_command(
            &format!(
                "git --no-pager diff -U10 --no-index {} {} {}",
                list_of_patterns_to_ignore,
                Branch::Base,
                Branch::Target
            ),
            Some(output_folder),
        )
        .await,
    );

    let markdown = print_diff(&summary_as_string, &diff_as_string);

    let markdown_path = format!("{}/diff.md", output_folder);
    fs::write(&markdown_path, &markdown)?;

    info!("🙏 Please check the {} file for differences", markdown_path);

    Ok(())
}

fn print_diff(summary: &str, diff: &str) -> String {
    let markdown = r#"
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

    markdown
        .replace("%summary%", summary)
        .replace("%diff%", diff)
}
