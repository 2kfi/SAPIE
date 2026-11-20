use crate::error::{CommandError, Result};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

const STATE_DIR: &str = "com.sapie.launcher";
const STATE_FILE: &str = "launcher-state.json";

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
pub struct GameRecord {
    pub exe_path: String,
    #[serde(default)]
    pub arch: String,
    #[serde(default)]
    pub era: String,
    #[serde(default)]
    pub interfaces: Vec<String>,
    #[serde(default)]
    pub original_library: Option<String>,
    #[serde(default)]
    pub library: Option<String>,
    #[serde(default)]
    pub appid: Option<u32>,
    #[serde(default)]
    pub configured: bool,
    #[serde(default)]
    pub launch_args: Vec<String>,
    #[serde(default)]
    pub ld_preload: Option<String>,
    #[serde(default)]
    pub last_played: Option<u64>,
}

impl GameRecord {
    pub fn new(exe_path: &str) -> Self {
        Self {
            exe_path: exe_path.to_string(),
            arch: "unknown".into(),
            era: "unknown".into(),
            ..Self::default()
        }
    }
}

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
pub struct LauncherState {
    #[serde(default)]
    pub server: Option<String>,
    #[serde(default)]
    pub username: Option<String>,
    #[serde(default)]
    pub steam_id: Option<u64>,
    #[serde(default)]
    pub persona_name: Option<String>,
    // Session expiry is metadata, not the secret itself. The token lives in the keychain.
    #[serde(default)]
    pub token_expires_at: Option<u64>,
    #[serde(default)]
    pub games: BTreeMap<String, GameRecord>,
}

pub fn state_path() -> Result<PathBuf> {
    let dir = dirs::data_dir()
        .ok_or_else(|| {
            CommandError::new(
                crate::error::ErrorKind::Io,
                "No per-user data directory found.",
            )
        })?
        .join(STATE_DIR);
    fs::create_dir_all(&dir)?;
    Ok(dir.join(STATE_FILE))
}

fn drop_secret_keys(value: &mut Value) -> bool {
    let mut changed = false;
    match value {
        Value::Object(map) => {
            let keys: Vec<String> = map
                .keys()
                .filter(|key| {
                    let lower = key.to_ascii_lowercase();
                    matches!(
                        lower.as_str(),
                        "password" | "token" | "session_token" | "secret"
                    )
                })
                .cloned()
                .collect();
            for key in keys {
                map.remove(&key);
                changed = true;
            }
            for child in map.values_mut() {
                changed |= drop_secret_keys(child);
            }
        }
        Value::Array(items) => {
            for child in items {
                changed |= drop_secret_keys(child);
            }
        }
        _ => {}
    }
    changed
}

pub fn load_state_at(path: &Path) -> Result<LauncherState> {
    if !path.is_file() {
        return Ok(LauncherState::default());
    }
    let mut value: Value = serde_json::from_str(&fs::read_to_string(path)?)?;
    if drop_secret_keys(&mut value) {
        // A 0.2 state file kept the password. Scrub it before anything else reads it.
        fs::write(
            path,
            serde_json::to_string_pretty(&value).unwrap_or_default(),
        )?;
    }
    Ok(serde_json::from_value(value)?)
}

pub fn save_state_at(path: &Path, state: &LauncherState) -> Result<()> {
    let text = serde_json::to_string_pretty(state)
        .map_err(|e| CommandError::new(crate::error::ErrorKind::Io, e.to_string()))?;
    let temp = path.with_extension("json.tmp");
    fs::write(&temp, text)?;
    if path.exists() {
        fs::remove_file(path)?;
    }
    fs::rename(&temp, path)?;
    Ok(())
}

pub fn load_state() -> Result<LauncherState> {
    load_state_at(&state_path()?)
}

pub fn save_state(state: &LauncherState) -> Result<()> {
    save_state_at(&state_path()?, state)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn temp_state(name: &str) -> PathBuf {
        std::env::temp_dir().join(format!("sapie-state-{}-{name}.json", std::process::id()))
    }

    #[test]
    fn state_never_contains_secrets() {
        let state = LauncherState {
            server: Some("192.168.1.2:8081".into()),
            username: Some("alice".into()),
            games: [("/games/game.exe".into(), GameRecord::new("/games/game.exe"))]
                .into_iter()
                .collect(),
            ..Default::default()
        };
        let value = serde_json::to_value(&state).unwrap();
        let keys: Vec<&String> = value.as_object().unwrap().keys().collect();
        for forbidden in ["password", "session_token", "token", "secret"] {
            assert!(
                !keys.iter().any(|key| *key == forbidden),
                "state leaked {forbidden}"
            );
        }
        assert_eq!(value["username"], "alice");
    }

    #[test]
    fn loading_scrubs_legacy_secrets() {
        let path = temp_state("legacy");
        fs::write(
            &path,
            r#"{"server":{"server":"host:8081","username":"a","password":"plain"},"games":{}}"#,
        )
        .unwrap();
        let text = fs::read_to_string(&path).unwrap();
        assert!(text.contains("plain"));
        let loaded = load_state_at(&path);
        let text = fs::read_to_string(&path).unwrap();
        assert!(!text.contains("plain"));
        assert!(!text.contains("password"));
        assert!(loaded.is_err(), "legacy server shape stays unsupported");
        fs::remove_file(&path).unwrap();
    }

    #[test]
    fn saves_and_reloads_state() {
        let path = temp_state("roundtrip");
        let _ = fs::remove_file(&path);
        let state = LauncherState {
            server: Some("127.0.0.1:8081".into()),
            token_expires_at: Some(1_800_000_000),
            games: [("/games/a.exe".into(), GameRecord::new("/games/a.exe"))]
                .into_iter()
                .collect(),
            ..Default::default()
        };
        save_state_at(&path, &state).unwrap();
        let loaded = load_state_at(&path).unwrap();
        assert_eq!(loaded.server, state.server);
        assert_eq!(loaded.token_expires_at, state.token_expires_at);
        assert!(loaded.games.contains_key("/games/a.exe"));
        fs::remove_file(&path).unwrap();
    }
}
