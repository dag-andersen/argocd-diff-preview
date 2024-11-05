use crate::{parsing::Application, Operator, Selector};
use log::{debug, error, warn};
use regex::Regex;

const ANNOTATION_IGNORE: &str = "argocd-diff-preview/ignore";
const ANNOTATION_WATCH_PATTERN: &str = "argocd-diff-preview/watch-pattern";

pub fn filter(
    apps: Vec<Application>,
    selector: &Option<Vec<Selector>>,
    files_changed: &Option<Vec<String>>, 
    ignore_invalid_watch_pattern: bool,
) -> Vec<Application> {

    let filtered_apps: Vec<Application> = apps.into_iter().filter_map(|a| {

        // check if the application should be ignored
        if a.yaml["metadata"]["annotations"][ANNOTATION_IGNORE].as_str()
            == Some("true")
        {
            debug!(
                "Ignoring application {:?} due to '{}=true' in file: {}",
                a.name,
                ANNOTATION_IGNORE,
                a.file_name
            );
            return None;
        }

        // loop over labels and check if the selector matches
        if let Some(selector) = selector {
            let labels: Vec<(&str, &str)> = {
                match a.yaml["metadata"]["labels"].as_mapping() {
                    Some(m) => m.iter()
                        .flat_map(|(k, v)| Some((k.as_str()?, v.as_str()?)))
                        .collect(),
                    None => Vec::new(),
                }
            };
            let selected = selector.iter().all(|l| match l.operator {
                Operator::Eq => labels.iter().any(|(k, v)| k == &l.key && v == &l.value),
                Operator::Ne => labels.iter().all(|(k, v)| k != &l.key || v != &l.value),
            });
            if !selected {
                debug!(
                    "Ignoring application {:?} due to label selector mismatch in file: {}",
                    a.name,
                    a.file_name
                );
                return None;
            } else {
                debug!(
                    "Selected application {:?} due to label selector match in file: {}",
                    a.name,
                    a.file_name
                );
            }
        }

        // Check watch pattern annotation
        let pattern_annotation = a.yaml["metadata"]["annotations"][ANNOTATION_WATCH_PATTERN].as_str();
        let list_of_regex_results = pattern_annotation.map(|s| s.split(',').map(|s| Regex::new(s.trim())).collect::<Vec<Result<Regex, regex::Error>>>());

        // Return early if a regex pattern is invalid
        if let Some(pattern_vec) = &list_of_regex_results {
            if let Some(p) = pattern_vec.iter().filter_map(|r| r.as_ref().err()).next() {
                if ignore_invalid_watch_pattern {
                    warn!("🚨 Ignoring application {:?} due to invalid regex pattern in '{}' ({}) - Error: {}",
                        a.name,
                        pattern_annotation.unwrap_or("unknown"),
                        a.file_name,
                        p);
                } else {
                    error!("🚨 Application {:?} has an invalid regex pattern in '{}' ({}) - Error: {}",
                        a.name,
                        pattern_annotation.unwrap_or("unknown"),
                        a.file_name,
                        p);
                    panic!("Invalid regex pattern in annotation");
                }
            }
        }

        let patterns: Option<Vec<Regex>> = list_of_regex_results.map(|v| v.into_iter().flat_map(|r| r.ok()).collect());

        match (files_changed, patterns) {
            (None, _) => {}
            // Check if the application changed.
            (Some(files_changed), _) if files_changed.contains(&a.file_name) => {
                debug!(
                    "Selected application {:?} due to file change in file: {}",
                    a.name,
                    a.file_name
                );
            }
            // Check if the application changed and the regex pattern matches.
            (Some(files_changed), Some(pattern)) if files_changed.iter().any(|f| pattern.iter().any(|r| r.is_match(f))) => {
                debug!(
                    "Selected application {:?} due to regex pattern '{}' matching changed files",
                    a.name,
                    pattern.iter().map(|r| r.as_str()).collect::<Vec<&str>>().join(", "),
                );
            }
            (_, Some(pattern)) => {
                debug!(
                    "Ignoring application {:?} due to regex pattern '{}' not matching changed files",
                    a.name,
                    pattern.iter().map(|r| r.as_str()).collect::<Vec<&str>>().join(", "),
                );
                return None;
            },
            (_, None) => {
                debug!(
                    "Ignoring application {:?} due to missing '{}' annotation ({})",
                    a.name,
                    &ANNOTATION_WATCH_PATTERN,
                    a.file_name
                );
                return None;
            }
        }

        Some(a)
    }).collect();

    filtered_apps
}

