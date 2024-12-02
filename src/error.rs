use std::error::Error;
use std::fmt;

#[derive(Debug)]
pub struct CommandOutput {
    pub stdout: String,
    pub stderr: String,
}

#[derive(Debug)]
pub struct CommandError {
    stderr: String,
}

impl CommandError {
    pub fn new(s: CommandOutput) -> Self {
        CommandError { stderr: s.stderr }
    }
}

impl Error for CommandError {}

impl fmt::Display for CommandError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.stderr)
    }
}
