use crate::error::{CommandError, ErrorKind, Result};
use crate::platform::{FileSystem, Platform, SystemFileSystem};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use std::collections::BTreeMap;
use std::path::Path;

const STATE_FILE: &str = "launcher-state.json";
pub const SCHEMA_VERSION: u32 = 2;

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

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct LauncherState {
    pub schema_version: u32,
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

impl Default for LauncherState {
    fn default() -> Self {
        Self {
            schema_version: SCHEMA_VERSION,
            server: None,
            username: None,
            steam_id: None,
            persona_name: None,
            token_expires_at: None,
            games: BTreeMap::new(),
        }
    }
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

fn backup_at(path: &Path, extension: &str, files: &dyn FileSystem) -> Result<()> {
    if files.exists(path) {
        files.copy(path, &path.with_extension(extension))?;
    }
    Ok(())
}

fn write_atomic(path: &Path, bytes: &[u8], files: &dyn FileSystem) -> Result<()> {
    let temp = path.with_extension("json.tmp");
    files.write(&temp, bytes)?;
    files.rename(&temp, path)?;
    Ok(())
}

pub fn load_from(path: &Path, files: &dyn FileSystem) -> Result<LauncherState> {
    if !files.exists(path) {
        return Ok(LauncherState::default());
    }

    let raw = files.read(path)?;
    let text = String::from_utf8(raw.clone()).map_err(|e| {
        CommandError::new(ErrorKind::Io, format!("Corrupt launcher state: {e}"))
    })?;
    let mut value: Value = match serde_json::from_str(&text) {
        Ok(value) => value,
        Err(_) => {
            backup_at(path, "json.corrupt", files)?;
            let state = LauncherState::default();
            let bytes = serde_json::to_vec_pretty(&state)?;
            write_atomic(path, &bytes, files)?;
            return Ok(state);
        }
    };

    let legacy = value.get("schema_version").is_none()
        || value
            .get("schema_version")
            .and_then(Value::as_u64)
            .is_some_and(|version| version < SCHEMA_VERSION as u64);
    if let Some(version) = value.get("schema_version").and_then(Value::as_u64) {
        if version > SCHEMA_VERSION as u64 {
            return Err(CommandError::new(
                ErrorKind::Io,
                format!(
                    "Launcher state schema {version} is newer than supported schema {SCHEMA_VERSION}."
                ),
            ));
        }
    }

    let scrubbed = drop_secret_keys(&mut value);
    if legacy || scrubbed {
        backup_at(path, "json.bak", files)?;
        value["schema_version"] = Value::from(SCHEMA_VERSION);
        let bytes = serde_json::to_vec_pretty(&value)?;
        write_atomic(path, &bytes, files)?;
    }

    Ok(serde_json::from_value(value)?)
}

pub fn save_to(path: &Path, state: &LauncherState, files: &dyn FileSystem) -> Result<()> {
    let bytes = serde_json::to_vec_pretty(state)?;
    backup_at(path, "json.bak", files)?;
    write_atomic(path, &bytes, files)
}

pub fn load_at(path: &Path) -> Result<LauncherState> {
    load_from(path, &SystemFileSystem)
}

pub fn save_at(path: &Path, state: &LauncherState) -> Result<()> {
    save_to(path, state, &SystemFileSystem)
}

pub fn load(platform: &dyn Platform) -> Result<LauncherState> {
    load_from(&platform.paths().data_dir()?.join(STATE_FILE), platform.files())
}

pub fn save(platform: &dyn Platform, state: &LauncherState) -> Result<()> {
    save_to(&platform.paths().data_dir()?.join(STATE_FILE), state, platform.files())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::platform::FakePlatform;

    fn temp_state(name: &str) -> std::path::PathBuf {
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
        assert_eq!(value["schema_version"], SCHEMA_VERSION);
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
        fs_write(&path, br#"{"server":{"server":"host:8081","username":"a","password":"plain"},"games":{}}"#);
        let loaded = load_at(&path);
        let text = std::fs::read_to_string(&path).unwrap();
        assert!(!text.contains("plain"));
        assert!(!text.contains("password"));
        assert!(loaded.is_err(), "legacy server shape stays unsupported");
        std::fs::remove_file(&path).unwrap();
    }

    #[test]
    fn saves_and_reloads_state() {
        let path = temp_state("roundtrip");
        let _ = std::fs::remove_file(&path);
        let state = LauncherState {
            server: Some("127.0.0.1:8081".into()),
            token_expires_at: Some(1_800_000_000),
            games: [("/games/a.exe".into(), GameRecord::new("/games/a.exe"))]
                .into_iter()
                .collect(),
            ..Default::default()
        };
        save_at(&path, &state).unwrap();
        let loaded = load_at(&path).unwrap();
        assert_eq!(loaded.schema_version, SCHEMA_VERSION);
        assert_eq!(loaded.server, state.server);
        assert_eq!(loaded.token_expires_at, state.token_expires_at);
        assert!(loaded.games.contains_key("/games/a.exe"));
        std::fs::remove_file(&path).unwrap();
    }

    #[test]
    fn platform_state_roundtrip_uses_the_injected_data_dir() {
        let fake = FakePlatform::default();
        let state = LauncherState {
            server: Some("127.0.0.1:8081".into()),
            ..Default::default()
        };
        save(&fake, &state).unwrap();
        assert_eq!(load(&fake).unwrap().server, state.server);
    }

    #[test]
    fn a_0_4_state_migrates_with_no_data_loss() {
        let fake = FakePlatform::default();
        let path = fake.paths().data_dir().unwrap().join(STATE_FILE);
        let legacy = br#"{"server":"host:8081","username":"alice","steam_id":76561197960265731,"persona_name":"Alice","token_expires_at":1800000000,"games":{"/games/mw2/iw4mp.exe":{"exe_path":"/games/mw2/iw4mp.exe","arch":"x86","era":"modern","interfaces":["SteamUser019"],"original_library":"/games/mw2/steam_api.dll.sapie-original.bak","library":"/games/mw2/steam_api.dll","appid":10190,"configured":true,"launch_args":["-windowed"],"ld_preload":"/games/mw2/libfix.so","last_played":1700000000}}}"#;
        fake.seed_file(&path, legacy);

        let loaded = load_from(&path, fake.files()).unwrap();

        assert_eq!(loaded.schema_version, SCHEMA_VERSION);
        assert_eq!(loaded.server.as_deref(), Some("host:8081"));
        assert_eq!(loaded.username.as_deref(), Some("alice"));
        assert_eq!(loaded.steam_id, Some(76561197960265731));
        assert_eq!(loaded.persona_name.as_deref(), Some("Alice"));
        assert_eq!(loaded.token_expires_at, Some(1_800_000_000));
        let game = &loaded.games["/games/mw2/iw4mp.exe"];
        assert_eq!(game.arch, "x86");
        assert_eq!(game.era, "modern");
        assert_eq!(game.interfaces, ["SteamUser019"]);
        assert_eq!(
            game.original_library.as_deref(),
            Some("/games/mw2/steam_api.dll.sapie-original.bak")
        );
        assert_eq!(game.library.as_deref(), Some("/games/mw2/steam_api.dll"));
        assert_eq!(game.appid, Some(10190));
        assert!(game.configured);
        assert_eq!(game.launch_args, ["-windowed"]);
        assert_eq!(game.ld_preload.as_deref(), Some("/games/mw2/libfix.so"));
        assert_eq!(game.last_played, Some(1_700_000_000));

        assert_eq!(fake.stored_file(&path.with_extension("json.bak")).unwrap(), legacy);
        let stored = fake.stored_file(&path).unwrap();
        let migrated: Value = serde_json::from_slice(&stored).unwrap();
        assert_eq!(migrated["schema_version"], SCHEMA_VERSION);
    }

    #[test]
    fn a_truncated_write_leaves_the_previous_file_usable() {
        let fake = FakePlatform::default();
        let path = fake.paths().data_dir().unwrap().join(STATE_FILE);
        let state = LauncherState {
            server: Some("127.0.0.1:8081".into()),
            ..Default::default()
        };
        save_to(&path, &state, fake.files()).unwrap();
        let previous = fake.stored_file(&path).unwrap();

        fake.seed_file(&path.with_extension("json.tmp"), b"{\"schema_version\":2,\"server\":");

        let loaded = load_from(&path, fake.files()).unwrap();
        assert_eq!(loaded.server, state.server);
        assert_eq!(fake.stored_file(&path).unwrap(), previous);
    }

    #[test]
    fn a_corrupt_state_file_is_backed_up_and_reset() {
        let fake = FakePlatform::default();
        let path = fake.paths().data_dir().unwrap().join(STATE_FILE);
        let corrupt = b"{\"schema_version\":2,\"games\":";
        fake.seed_file(&path, corrupt);

        let loaded = load_from(&path, fake.files()).unwrap();

        assert_eq!(loaded.schema_version, SCHEMA_VERSION);
        assert!(loaded.games.is_empty());
        assert_eq!(fake.stored_file(&path.with_extension("json.corrupt")).unwrap(), corrupt);
        assert_eq!(load_from(&path, fake.files()).unwrap().schema_version, SCHEMA_VERSION);
    }

    #[test]
    fn saving_keeps_the_previous_state_as_a_backup() {
        let fake = FakePlatform::default();
        let path = fake.paths().data_dir().unwrap().join(STATE_FILE);
        let first = LauncherState {
            server: Some("first:8081".into()),
            ..Default::default()
        };
        let second = LauncherState {
            server: Some("second:8081".into()),
            ..Default::default()
        };

        save_to(&path, &first, fake.files()).unwrap();
        save_to(&path, &second, fake.files()).unwrap();

        let backup = load_from(&path.with_extension("json.bak"), fake.files()).unwrap();
        assert_eq!(backup.server, first.server);
        assert_eq!(load_from(&path, fake.files()).unwrap().server, second.server);
    }

    fn fs_write(path: &Path, bytes: &[u8]) {
        std::fs::write(path, bytes).unwrap();
    }
}
