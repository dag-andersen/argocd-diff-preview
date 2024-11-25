use log::{debug, error, warn};
use regex::Regex;

use crate::{parsing::K8sResource, selector::Operator, Selector};

const ANNOTATION_WATCH_PATTERN: &str = "argocd-diff-preview/watch-pattern";
const ANNOTATION_IGNORE: &str = "argocd-diff-preview/ignore";

#[derive(PartialEq)]
pub enum ApplicationKind {
    Application,
    ApplicationSet,
}

pub struct ArgoResource {
    pub file_name: String,
    pub yaml: serde_yaml::Value,
    pub kind: ApplicationKind,
    pub name: String,
}

impl std::fmt::Display for ArgoResource {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        write!(f, "{}", serde_yaml::to_string(&self.yaml).unwrap())
    }
}

impl PartialEq for ArgoResource {
    fn eq(&self, other: &Self) -> bool {
        self.yaml == other.yaml
    }
}

impl ArgoResource {
    pub fn set_namespace(mut self, namespace: &str) -> ArgoResource {
        self.yaml["metadata"]["namespace"] = serde_yaml::Value::String(namespace.to_string());
        self
    }

    pub fn set_project_to_default(mut self) -> ArgoResource {
        let spec = match self.kind {
            ApplicationKind::Application => self.yaml["spec"].as_mapping_mut().unwrap(),
            ApplicationKind::ApplicationSet => self.yaml["spec"]["template"]["spec"]
                .as_mapping_mut()
                .unwrap(),
        };
        spec["project"] = serde_yaml::Value::String("default".to_string());
        self
    }

    pub fn point_destination_to_in_cluster(mut self) -> ArgoResource {
        let spec = match self.kind {
            ApplicationKind::Application => self.yaml["spec"].as_mapping_mut().unwrap(),
            ApplicationKind::ApplicationSet => self.yaml["spec"]["template"]["spec"]
                .as_mapping_mut()
                .unwrap(),
        };
        if spec.contains_key("destination") {
            spec["destination"]["name"] = serde_yaml::Value::String("in-cluster".to_string());
            spec["destination"]
                .as_mapping_mut()
                .map(|a| a.remove("server"));
        }
        self
    }

    pub fn remove_sync_policy(mut self) -> ArgoResource {
        let spec = match self.kind {
            ApplicationKind::Application => self.yaml["spec"].as_mapping_mut().unwrap(),
            ApplicationKind::ApplicationSet => self.yaml["spec"]["template"]["spec"]
                .as_mapping_mut()
                .unwrap(),
        };
        spec.remove("syncPolicy");
        self
    }

    pub fn redirect_sources(mut self, repo: &str, branch: &str) -> ArgoResource {
        let spec = match self.kind {
            ApplicationKind::Application => self.yaml["spec"].as_mapping_mut().unwrap(),
            ApplicationKind::ApplicationSet => self.yaml["spec"]["template"]["spec"]
                .as_mapping_mut()
                .unwrap(),
        };
        if spec.contains_key("source") {
            if spec["source"]["chart"].as_str().is_some() {
                return self;
            }
            match spec["source"]["repoURL"].as_str() {
                Some(url) if url.to_lowercase().contains(&repo.to_lowercase()) => {
                    spec["source"]["targetRevision"] = serde_yaml::Value::String(branch.to_string())
                }
                _ => debug!(
                    "Found no 'repoURL' under spec.source in file: {}",
                    self.file_name
                ),
            }
        } else if spec.contains_key("sources") {
            if let Some(sources) = spec["sources"].as_sequence_mut() {
                for source in sources {
                    if source["chart"].as_str().is_some() {
                        continue;
                    }
                    match source["repoURL"].as_str() {
                        Some(url) if url.to_lowercase().contains(&repo.to_lowercase()) => {
                            source["targetRevision"] =
                                serde_yaml::Value::String(branch.to_string());
                        }
                        _ => debug!(
                            "Found no 'repoURL' under spec.sources[] in file: {}",
                            self.file_name
                        ),
                    }
                }
            }
        }
        self
    }

    pub fn from_k8s_resource(k8s_resource: K8sResource) -> Option<ArgoResource> {
        let kind = k8s_resource.yaml["kind"]
            .as_str()
            .and_then(|kind| match kind {
                "Application" => Some(ApplicationKind::Application),
                "ApplicationSet" => Some(ApplicationKind::ApplicationSet),
                _ => None,
            })?;

        match k8s_resource.yaml["metadata"]["name"].as_str() {
            Some(name) => Some(ArgoResource {
                kind,
                file_name: k8s_resource.file_name,
                name: name.to_string(),
                yaml: k8s_resource.yaml,
            }),
            _ => None,
        }
    }

    pub fn filter(
        self,
        selector: &Option<Vec<Selector>>,
        files_changed: &Option<Vec<String>>,
        ignore_invalid_watch_pattern: bool,
    ) -> Option<ArgoResource> {
        // check if the application should be ignored
        if self.yaml["metadata"]["annotations"][ANNOTATION_IGNORE].as_str() == Some("true") {
            debug!(
                "Ignoring application {:?} due to '{}=true' in file: {}",
                self.name, ANNOTATION_IGNORE, self.file_name
            );
            return None;
        }

        // loop over labels and check if the selector matches
        if let Some(selector) = selector {
            let labels: Vec<(&str, &str)> = {
                match self.yaml["metadata"]["labels"].as_mapping() {
                    Some(m) => m
                        .iter()
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
                    self.name, self.file_name
                );
                return None;
            } else {
                debug!(
                    "Selected application {:?} due to label selector match in file: {}",
                    self.name, self.file_name
                );
            }
        }

        // Check watch pattern annotation
        let pattern_annotation =
            self.yaml["metadata"]["annotations"][ANNOTATION_WATCH_PATTERN].as_str();
        let list_of_regex_results = pattern_annotation.map(|s| {
            s.split(',')
                .map(|s| Regex::new(s.trim()))
                .collect::<Vec<Result<Regex, regex::Error>>>()
        });

        // Return early if a regex pattern is invalid
        if let Some(pattern_vec) = &list_of_regex_results {
            if let Some(p) = pattern_vec.iter().filter_map(|r| r.as_ref().err()).next() {
                if ignore_invalid_watch_pattern {
                    warn!("🚨 Ignoring application {:?} due to invalid regex pattern in '{}' ({}) - Error: {}",
                        self.name,
                        pattern_annotation.unwrap_or("unknown"),
                        self.file_name,
                        p);
                } else {
                    error!(
                        "🚨 Application {:?} has an invalid regex pattern in '{}' ({}) - Error: {}",
                        self.name,
                        pattern_annotation.unwrap_or("unknown"),
                        self.file_name,
                        p
                    );
                    panic!("Invalid regex pattern in annotation");
                }
            }
        }

        let patterns: Option<Vec<Regex>> =
            list_of_regex_results.map(|v| v.into_iter().flat_map(|r| r.ok()).collect());

        match (files_changed, patterns) {
            (None, _) => {}
            // Check if the application changed.
            (Some(files_changed), _) if files_changed.contains(&self.file_name) => {
                debug!(
                    "Selected application {:?} due to file change in file: {}",
                    self.name, self.file_name
                );
            }
            // Check if the application changed and the regex pattern matches.
            (Some(files_changed), Some(pattern))
                if files_changed
                    .iter()
                    .any(|f| pattern.iter().any(|r| r.is_match(f))) =>
            {
                debug!(
                    "Selected application {:?} due to regex pattern '{}' matching changed files",
                    self.name,
                    pattern
                        .iter()
                        .map(|r| r.as_str())
                        .collect::<Vec<&str>>()
                        .join(", "),
                );
            }
            (_, Some(pattern)) => {
                debug!(
                    "Ignoring application {:?} due to regex pattern '{}' not matching changed files",
                    self.name,
                    pattern.iter().map(|r| r.as_str()).collect::<Vec<&str>>().join(", "),
                );
                return None;
            }
            (_, None) => {
                debug!(
                    "Ignoring application {:?} due to missing '{}' annotation ({})",
                    self.name, &ANNOTATION_WATCH_PATTERN, self.file_name
                );
                return None;
            }
        }

        Some(self)
    }
}
