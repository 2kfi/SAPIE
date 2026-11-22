use std::fmt;
use std::path::Path;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ErrorKind {
    InvalidArgument,
    InvalidPath,
    NotFound,
    PermissionDenied,
    BackupConflict,
    InvalidServer,
    Unreachable,
    BadCredentials,
    ServerTooOld,
    ServerError,
    TokenExpired,
    KeychainError,
    Io,
    Process,
}

#[derive(Debug, Clone)]
pub struct CommandError {
    pub kind: ErrorKind,
    pub message: String,
}

pub type Result<T> = std::result::Result<T, CommandError>;

impl CommandError {
    pub fn new(kind: ErrorKind, message: impl Into<String>) -> Self {
        Self {
            kind,
            message: message.into(),
        }
    }

    pub fn path(kind: ErrorKind, path: &Path, message: &str) -> Self {
        Self::new(kind, format!("{}: {}", path.display(), message))
    }
}

impl fmt::Display for CommandError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.message)
    }
}

impl std::error::Error for CommandError {}

impl From<std::io::Error> for CommandError {
    fn from(error: std::io::Error) -> Self {
        let kind = match error.kind() {
            std::io::ErrorKind::NotFound => ErrorKind::NotFound,
            std::io::ErrorKind::PermissionDenied => ErrorKind::PermissionDenied,
            _ => ErrorKind::Io,
        };
        Self::new(kind, error.to_string())
    }
}

impl From<serde_json::Error> for CommandError {
    fn from(error: serde_json::Error) -> Self {
        Self::new(ErrorKind::Io, format!("Corrupt launcher state: {error}"))
    }
}
