use crate::error::{CommandError, ErrorKind, Result};
use crate::platform::{FileSystem, Platform};
use crate::state::{self, drop_secret_keys, LauncherState, STATE_FILE, SCHEMA_VERSION};
use serde_json::Value;
use std::path::Path;

const SETTINGS_DIR: &str = "steam_settings";
const TOKEN_FILE: &str = "sapie_client.json";
const MIGRATED_BACKUP: &str = "json.0.4.bak";
const FAILED_BACKUP: &str = "json.migration-failed.bak";

pub fn migrate(platform: &dyn Platform) -> Result<LauncherState> {
    let path = platform.paths().data_dir()?.join(STATE_FILE);
    migrate_at(&path, platform)
}

fn migrate_at(path: &Path, platform: &dyn Platform) -> Result<LauncherState> {
    let files = platform.files();
    if !files.exists(path) {
        return Ok(LauncherState::default());
    }
    let raw = files.read(path)?;
    let mut value: Value = serde_json::from_slice(&raw).map_err(|error| {
        fail(path, &raw, files, &format!("Could not read the 0.4 state: {error}"))
    })?;

    let version = value.get("schema_version").and_then(Value::as_u64);
    if version == Some(SCHEMA_VERSION as u64) {
        return Ok(serde_json::from_value(value)?);
    }
    if let Some(version) = version.filter(|version| *version > SCHEMA_VERSION as u64) {
        return Err(fail(
            path,
            &raw,
            files,
            &format!("State schema {version} is newer than supported schema {SCHEMA_VERSION}"),
        ));
    }

    drop_secret_keys(&mut value);
    value["schema_version"] = Value::from(SCHEMA_VERSION);
    let state: LauncherState = serde_json::from_value(value).map_err(|error| {
        fail(
            path,
            &raw,
            files,
            &format!("Could not convert the 0.4 state: {error}"),
        )
    })?;
    restore_token(&state, platform).map_err(|error| {
        fail(
            path,
            &raw,
            files,
            &format!("Could not migrate the 0.4 session token: {error}"),
        )
    })?;

    let backup = backup_bytes(&raw);
    files
        .write(&path.with_extension(MIGRATED_BACKUP), &backup)
        .map_err(|error| {
            fail(
                path,
                &raw,
                files,
                &format!("Could not back up the 0.4 state: {error}"),
            )
        })?;
    let bytes = serde_json::to_vec_pretty(&state).map_err(|error| {
        fail(
            path,
            &raw,
            files,
            &format!("Could not encode the 0.5 state: {error}"),
        )
    })?;
    state::write_atomic(path, &bytes, files).map_err(|error| {
        fail(
            path,
            &raw,
            files,
            &format!("Could not write the 0.5 state: {error}"),
        )
    })?;
    Ok(state)
}

fn backup_bytes(raw: &[u8]) -> Vec<u8> {
    let Ok(mut value) = serde_json::from_slice::<Value>(raw) else {
        return raw.to_vec();
    };
    if drop_secret_keys(&mut value) {
        serde_json::to_vec_pretty(&value).unwrap_or_else(|_| raw.to_vec())
    } else {
        raw.to_vec()
    }
}

fn fail(path: &Path, raw: &[u8], files: &dyn FileSystem, reason: &str) -> CommandError {
    let backup = path.with_extension(FAILED_BACKUP);
    match files.write(&backup, &backup_bytes(raw)) {
        Ok(()) => CommandError::new(
            ErrorKind::Io,
            format!(
                "Migration failed: {reason}. The original state is at {}.",
                backup.display()
            ),
        ),
        Err(error) => CommandError::new(
            ErrorKind::Io,
            format!("Migration failed: {reason}. The original state could not be backed up: {error}"),
        ),
    }
}

fn restore_token(state: &LauncherState, platform: &dyn Platform) -> Result<()> {
    if platform.keychain().get()?.is_some() {
        return Ok(());
    }
    let files = platform.files();
    for record in state.games.values() {
        let dir = Path::new(&record.exe_path).parent().unwrap_or_else(|| Path::new(""));
        for candidate in [dir.join(SETTINGS_DIR).join(TOKEN_FILE), dir.join(TOKEN_FILE)] {
            if !files.exists(&candidate) {
                continue;
            }
            let raw = files.read(&candidate)?;
            let value: Value = serde_json::from_slice(&raw).map_err(|error| {
                CommandError::new(
                    ErrorKind::Io,
                    format!("{}: invalid 0.4 token file: {error}", candidate.display()),
                )
            })?;
            let token = value.get("session_token").and_then(Value::as_str).ok_or_else(|| {
                CommandError::new(
                    ErrorKind::Io,
                    format!("{}: no session_token in the 0.4 token file", candidate.display()),
                )
            })?;
            if token.is_empty() {
                return Err(CommandError::new(
                    ErrorKind::Io,
                    format!("{}: empty 0.4 session token", candidate.display()),
                ));
            }
            platform.keychain().set(token)?;
            return Ok(());
        }
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::platform::FakePlatform;
    use std::path::PathBuf;

    const OLD: &[u8] = br#"{
  "server": "192.168.1.10:8081",
  "username": "alice",
  "steam_id": 76561197960265731,
  "persona_name": "Alice",
  "token_expires_at": 1800000000,
  "games": {
    "/games/mw2/iw4mp.exe": {
      "exe_path": "/games/mw2/iw4mp.exe",
      "arch": "x86",
      "era": "modern",
      "interfaces": ["SteamUser019"],
      "original_library": "/games/mw2/steam_api.dll.sapie-original.bak",
      "library": "/games/mw2/steam_api.dll",
      "appid": 10190,
      "configured": true,
      "launch_args": ["-windowed"],
      "ld_preload": "/games/mw2/libfix.so",
      "last_played": 1700000000
    },
    "/games/hl2/hl2.exe": {
      "exe_path": "/games/hl2/hl2.exe",
      "arch": "x86",
      "era": "legacy",
      "interfaces": [],
      "appid": 220,
      "configured": false
    }
  }
}"#;

    const CURRENT: &[u8] = br#"{
  "schema_version": 2,
  "server": "192.168.1.10:8081",
  "username": "alice",
  "steam_id": 76561197960265731,
  "persona_name": "Alice",
  "token_expires_at": 1800000000,
  "games": {
    "/games/mw2/iw4mp.exe": {
      "exe_path": "/games/mw2/iw4mp.exe",
      "arch": "x86",
      "era": "modern",
      "interfaces": ["SteamUser019"],
      "original_library": "/games/mw2/steam_api.dll.sapie-original.bak",
      "library": "/games/mw2/steam_api.dll",
      "appid": 10190,
      "configured": true,
      "launch_args": ["-windowed"],
      "ld_preload": "/games/mw2/libfix.so",
      "last_played": 1700000000
    },
    "/games/hl2/hl2.exe": {
      "exe_path": "/games/hl2/hl2.exe",
      "arch": "x86",
      "era": "legacy",
      "interfaces": [],
      "appid": 220,
      "configured": false
    }
  }
}"#;

    const TOKEN_PATH: &str = "/games/mw2/steam_settings/sapie_client.json";
    const GAME_FILES: &[(&str, &[u8])] = &[
        ("/games/mw2/steam_api.dll.sapie-original.bak", b"ORIGINAL-DLL"),
        ("/games/mw2/sapie_server.txt", b"127.0.0.1:47584\n"),
        ("/games/mw2/steam_appid.txt", b"10190\n"),
    ];

    #[derive(Clone, Copy, PartialEq, Eq)]
    enum Expect {
        Migrated,
        Failed,
        Unchanged,
    }

    struct Case {
        name: &'static str,
        state: &'static [u8],
        keychain: Option<&'static str>,
        token_file: Option<&'static [u8]>,
        existing_backup: Option<&'static [u8]>,
        expect: Expect,
    }

    const CASES: &[Case] = &[
        Case {
            name: "profile",
            state: OLD,
            keychain: Some("tok-from-keychain"),
            token_file: Some(br#"{"session_token":"tok-from-file"}"#),
            existing_backup: None,
            expect: Expect::Migrated,
        },
        Case {
            name: "recover-token",
            state: OLD,
            keychain: None,
            token_file: Some(br#"{"session_token":"tok-from-file"}"#),
            existing_backup: None,
            expect: Expect::Migrated,
        },
        Case {
            name: "broken-token",
            state: OLD,
            keychain: None,
            token_file: Some(b"not json"),
            existing_backup: None,
            expect: Expect::Failed,
        },
        Case {
            name: "second-run",
            state: CURRENT,
            keychain: Some("tok-0.5"),
            token_file: None,
            existing_backup: Some(b"0.4 original backup"),
            expect: Expect::Unchanged,
        },
    ];

    fn assert_profile(state: &LauncherState) {
        assert_eq!(state.schema_version, SCHEMA_VERSION);
        assert_eq!(state.server.as_deref(), Some("192.168.1.10:8081"));
        assert_eq!(state.username.as_deref(), Some("alice"));
        assert_eq!(state.steam_id, Some(76_561_197_960_265_731));
        assert_eq!(state.persona_name.as_deref(), Some("Alice"));
        assert_eq!(state.token_expires_at, Some(1_800_000_000));
        assert_eq!(state.games.len(), 2);
        let mw2 = &state.games["/games/mw2/iw4mp.exe"];
        assert_eq!(mw2.arch, "x86");
        assert_eq!(mw2.era, "modern");
        assert_eq!(mw2.interfaces, ["SteamUser019"]);
        assert_eq!(
            mw2.original_library.as_deref(),
            Some("/games/mw2/steam_api.dll.sapie-original.bak")
        );
        assert_eq!(mw2.library.as_deref(), Some("/games/mw2/steam_api.dll"));
        assert_eq!(mw2.appid, Some(10190));
        assert!(mw2.configured);
        assert_eq!(mw2.launch_args, ["-windowed"]);
        assert_eq!(mw2.ld_preload.as_deref(), Some("/games/mw2/libfix.so"));
        assert_eq!(mw2.last_played, Some(1_700_000_000));
        let hl2 = &state.games["/games/hl2/hl2.exe"];
        assert_eq!(hl2.appid, Some(220));
        assert!(!hl2.configured);
    }

    #[test]
    fn migration_cases_preserve_every_game_setting_and_the_original() {
        for case in CASES {
            let fake = FakePlatform::default();
            let path = PathBuf::from(format!("/migration/{}/launcher-state.json", case.name));
            fake.seed_file(&path, case.state);
            if let Some(token) = case.keychain {
                fake.keychain().set(token).unwrap();
            }
            if let Some(bytes) = case.token_file {
                fake.seed_file(Path::new(TOKEN_PATH), bytes);
            }
            if let Some(bytes) = case.existing_backup {
                fake.seed_file(&path.with_extension(MIGRATED_BACKUP), bytes);
            }
            for (file, bytes) in GAME_FILES {
                fake.seed_file(Path::new(file), bytes);
            }

            let result = migrate_at(&path, &fake);
            match case.expect {
                Expect::Migrated => {
                    assert_profile(&result.unwrap_or_else(|error| panic!("{}: {error}", case.name)));
                    let stored = fake.stored_file(&path).unwrap();
                    let stored: Value = serde_json::from_slice(&stored).unwrap();
                    assert_eq!(stored["schema_version"], SCHEMA_VERSION);
                    assert!(stored.get("token").is_none());
                    assert!(stored.get("password").is_none());
                    assert_eq!(
                        fake.stored_file(&path.with_extension(MIGRATED_BACKUP)).unwrap().as_slice(),
                        case.state
                    );
                    let expected = if case.keychain.is_some() {
                        case.keychain
                    } else {
                        Some("tok-from-file")
                    };
                    assert_eq!(fake.keychain().get().unwrap().as_deref(), expected);
                }
                Expect::Failed => {
                    let error = result.unwrap_err();
                    assert!(error.message.contains("Migration failed"), "{}", error.message);
                    assert!(error.message.contains(FAILED_BACKUP), "{}", error.message);
                    assert_eq!(fake.stored_file(&path).unwrap().as_slice(), case.state);
                    assert_eq!(
                        fake.stored_file(&path.with_extension(FAILED_BACKUP)).unwrap().as_slice(),
                        case.state
                    );
                    assert_eq!(fake.keychain().get().unwrap(), None);
                }
                Expect::Unchanged => {
                    assert_profile(&result.unwrap_or_else(|error| panic!("{}: {error}", case.name)));
                    assert_eq!(fake.stored_file(&path).unwrap().as_slice(), case.state);
                    assert_eq!(
                        fake.stored_file(&path.with_extension(MIGRATED_BACKUP)).unwrap().as_slice(),
                        case.existing_backup.unwrap()
                    );
                    assert_eq!(fake.keychain().get().unwrap().as_deref(), case.keychain);
                }
            }
            for (file, bytes) in GAME_FILES {
                assert_eq!(
                    fake.stored_file(Path::new(file)).unwrap().as_slice(),
                    *bytes,
                    "{} lost {file}",
                    case.name
                );
            }
            if let Some(bytes) = case.token_file {
                assert_eq!(fake.stored_file(Path::new(TOKEN_PATH)).unwrap().as_slice(), bytes);
            }
        }
    }

    #[test]
    fn a_secret_in_a_0_4_state_is_not_copied_into_the_backup() {
        let fake = FakePlatform::default();
        let path = PathBuf::from("/migration/secrets/launcher-state.json");
        let legacy = br#"{"server":"host:8081","password":"plain","games":{}}"#;
        fake.seed_file(&path, legacy);

        let result = migrate_at(&path, &fake);

        assert!(result.is_ok());
        let backup = fake.stored_file(&path.with_extension(MIGRATED_BACKUP)).unwrap();
        assert!(!String::from_utf8_lossy(&backup).contains("plain"));
        let stored = fake.stored_file(&path).unwrap();
        assert!(!String::from_utf8_lossy(&stored).contains("plain"));
    }
}
