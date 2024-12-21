use log::{debug, error, warn};
use regex::Regex;
use serde_yaml::Mapping;
use std::error::Error;

use crate::{parsing::K8sResource, selector::Operator, Selector};

const ANNOTATION_WATCH_PATTERN: &str = "argocd-diff-preview/watch-pattern";
const ANNOTATION_IGNORE: &str = "argocd-diff-preview/ignore";

#[derive(PartialEq, Clone)]
pub enum ApplicationKind {
    Application,
    ApplicationSet,
}

#[derive(Clone)]
pub struct ArgoResource {
    pub yaml: serde_yaml::Value,
    pub kind: ApplicationKind,
    pub name: String,
    pub namespace: String,
    // Where the resource was found
    pub file_name: String,
}

impl ArgoResource {
    pub fn as_string(&self) -> Result<String, Box<dyn Error>> {
        Ok(serde_yaml::to_string(&self.yaml)?)
    }
}

impl PartialEq for ArgoResource {
    fn eq(&self, other: &Self) -> bool {
        self.yaml == other.yaml
    }
}

impl ArgoResource {
    pub fn set_project_to_default(mut self) -> Result<ArgoResource, Box<dyn Error>> {
        let spec = match self.kind {
            ApplicationKind::Application => self.yaml["spec"].as_mapping_mut(),
            ApplicationKind::ApplicationSet => {
                self.yaml["spec"]["template"]["spec"].as_mapping_mut()
            }
        };

        match spec {
            None => Err(format!("No 'spec' key found in Application: {}", self.name).into()),
            Some(spec) => {
                spec["project"] = serde_yaml::Value::String("default".to_string());
                Ok(self)
            }
        }
    }

    pub fn point_destination_to_in_cluster(mut self) -> Result<ArgoResource, Box<dyn Error>> {
        let spec = match self.kind {
            ApplicationKind::Application => self.yaml["spec"].as_mapping_mut(),
            ApplicationKind::ApplicationSet => {
                self.yaml["spec"]["template"]["spec"].as_mapping_mut()
            }
        };

        match spec {
            None => Err(format!("No 'spec' key found in Application: {}", self.name).into()),
            Some(spec) if spec.contains_key("destination") => {
                spec["destination"]["name"] = serde_yaml::Value::String("in-cluster".to_string());
                spec["destination"]
                    .as_mapping_mut()
                    .map(|a| a.remove("server"));
                Ok(self)
            }
            Some(_) => Err(format!(
                "No 'spec.destination' key found in Application: {}",
                self.name
            )
            .into()),
        }
    }

    pub fn remove_sync_policy(mut self) -> ArgoResource {
        let spec = match self.kind {
            ApplicationKind::Application => self.yaml["spec"].as_mapping_mut(),
            ApplicationKind::ApplicationSet => {
                self.yaml["spec"]["template"]["spec"].as_mapping_mut()
            }
        };
        match spec {
            Some(spec) => {
                spec.remove("syncPolicy");
            }
            None => debug!(
                "Can't remove 'syncPolicy' because 'spec' key not found in file: {}",
                self.file_name
            ),
        }
        self
    }

    pub fn redirect_sources(
        mut self,
        repo: &str,
        branch: &str,
    ) -> Result<ArgoResource, Box<dyn Error>> {
        let spec = match self.kind {
            ApplicationKind::Application => self.yaml["spec"].as_mapping_mut(),
            ApplicationKind::ApplicationSet => {
                self.yaml["spec"]["template"]["spec"].as_mapping_mut()
            }
        };

        match spec {
            None => Err(format!("No 'spec' key found in Application: {}", self.name).into()),
            Some(spec) if spec.contains_key("source") => {
                if spec["source"]["chart"].as_str().is_some() {
                    return Ok(self);
                }
                match spec["source"]["repoURL"].as_str() {
                    Some(url) if url.to_lowercase().contains(&repo.to_lowercase()) => {
                        spec["source"]["targetRevision"] =
                            serde_yaml::Value::String(branch.to_string());
                    }
                    _ => debug!(
                        "Found no 'repoURL' under spec.source in file: {}",
                        self.file_name
                    ),
                }
                Ok(self)
            }
            Some(spec) if spec.contains_key("sources") => {
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
                Ok(self)
            }
            Some(_) => Err(format!(
                "No 'spec.source' or 'spec.sources' key found in Application: {}",
                self.name
            )
            .into()),
        }
    }

    pub fn redirect_generators(
        mut self,
        repo: &str,
        branch: &str,
    ) -> Result<ArgoResource, Box<dyn Error>> {
        if self.kind != ApplicationKind::ApplicationSet {
            return Ok(self);
        }

        let spec = self.yaml["spec"].as_mapping_mut();

        match spec {
            None => Err(format!("No 'spec' key found in ApplicationSet: {}", self.name).into()),
            Some(spec) => {
                if spec.contains_key("generators") {
                    if redirect_git_generator(spec, repo, branch) {
                        debug!(
                            "Patched git generators in ApplicationSet: {} in file: {}",
                            self.name, self.file_name
                        );
                    }
                    if let Some(g) = spec["generators"].as_sequence_mut() {
                        for generator in g {
                            if let Some(i) = generator["matrix"].as_mapping_mut() {
                                if redirect_git_generator(i, repo, branch) {
                                    debug!(
                                        "Patched git generators in matrix generators in ApplicationSet: {} in file: {}",
                                        self.name,
                                        &self.file_name
                                    );
                                }
                            }
                        }
                    }
                }
                Ok(self)
            }
        }
    }

    pub fn from_k8s_resource(k8s_resource: K8sResource) -> Option<ArgoResource> {
        let kind = k8s_resource.yaml["kind"]
            .as_str()
            .and_then(|kind| match kind {
                "Application" => Some(ApplicationKind::Application),
                "ApplicationSet" => Some(ApplicationKind::ApplicationSet),
                _ => None,
            })?;

        let namespace = k8s_resource.yaml["metadata"]["namespace"]
            .as_str()
            .unwrap_or("default");

        match k8s_resource.yaml["metadata"]["name"].as_str() {
            Some(name) => Some(ArgoResource {
                kind,
                file_name: k8s_resource.file_name,
                name: name.to_string(),
                namespace: namespace.to_string(),
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

// Returns true if the generators were patched
fn redirect_git_generator(v: &mut Mapping, repo: &str, branch: &str) -> bool {
    let mut patched = false;
    if v.contains_key("generators") {
        if let Some(i) = v["generators"].as_sequence_mut() {
            for generator in i {
                if let Some(git) = generator["git"].as_mapping_mut() {
                    if git.contains_key("repoURL") {
                        match git["repoURL"].as_str() {
                            Some(url) if url.to_lowercase().contains(&repo.to_lowercase()) => {
                                git["revision"] = serde_yaml::Value::String(branch.to_string());
                                debug!("Redirected 'repoURL' in git generator",);
                                patched = true;
                            }
                            Some(_url) => {
                                debug!("Found no matching 'repoURL' in git generator")
                            }
                            _ => (),
                        }
                    }
                }
            }
        }
    }
    patched
}
