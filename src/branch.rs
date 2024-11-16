pub enum BranchType {
    Base,
    Target,
}

pub struct Branch {
    pub name: String,
    pub branch_type: BranchType,
}

impl Branch {
    pub fn app_file(&self) -> &'static str {
        match self.branch_type {
            BranchType::Base => "apps_base_branch.yaml",
            BranchType::Target => "apps_target_branch.yaml",
        }
    }

    pub fn folder_name(&self) -> &str {
        match self.branch_type {
            BranchType::Base => "base-branch",
            BranchType::Target => "target-branch",
        }
    }
}

impl std::fmt::Display for BranchType {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            BranchType::Base => write!(f, "base"),
            BranchType::Target => write!(f, "target"),
        }
    }
}
