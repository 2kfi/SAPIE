use crate::auth::{self, SessionInfo, TokenStore};
use crate::error::{CommandError, ErrorKind, Result};
use crate::game::{self, ConfigureResult, DetectResult, InstallResult};
use crate::state::{self, GameRecord, LauncherState};
use serde::Serialize;
use std::sync::{Arc, Mutex};
use tauri::{AppHandle, Emitter, State};

const PROGRESS_EVENT: &str = "sapie-progress";

#[derive(Clone, Serialize)]
struct ProgressEvent {
    step: String,
    message: String,
}

pub struct AppState {
    pub store: Arc<dyn TokenStore>,
    data: Mutex<LauncherState>,
}

impl AppState {
    pub fn new(store: Arc<dyn TokenStore>) -> Result<Self> {
        Ok(Self {
            store,
            data: Mutex::new(state::load_state()?),
        })
    }

    fn lock(&self) -> Result<std::sync::MutexGuard<'_, LauncherState>> {
        self.data.lock().map_err(|_| {
            CommandError::new(ErrorKind::Io, "Launcher state is locked by another task.")
        })
    }

    fn snapshot(&self) -> Result<LauncherState> {
        Ok(self.lock()?.clone())
    }

    fn update<T>(&self, apply: impl FnOnce(&mut LauncherState) -> Result<T>) -> Result<T> {
        let mut data = self.lock()?;
        let value = apply(&mut data)?;
        state::save_state(&data)?;
        Ok(value)
    }
}

async fn blocking<T: Send + 'static>(
    task: impl FnOnce() -> Result<T> + Send + 'static,
) -> Result<T> {
    tauri::async_runtime::spawn_blocking(task)
        .await
        .map_err(|e| CommandError::new(ErrorKind::Process, format!("Task failed to start: {e}")))?
}

fn emit(app: &AppHandle, step: &str, message: &str) {
    let _ = app.emit(
        PROGRESS_EVENT,
        ProgressEvent {
            step: step.to_string(),
            message: message.to_string(),
        },
    );
}

pub(crate) fn add_record(data: &mut LauncherState, mut record: GameRecord) -> GameRecord {
    let key = record.exe_path.clone();
    if let Some(existing) = data.games.get_mut(&key) {
        existing.arch = record.arch;
        existing.era = record.era;
        existing.interfaces = std::mem::take(&mut record.interfaces);
        existing.original_library = record.original_library;
        record = existing.clone();
    } else {
        data.games.insert(key.clone(), record.clone());
    }
    record
}

pub(crate) fn remove_record(data: &mut LauncherState, path: &str) -> Option<GameRecord> {
    data.games.remove(path)
}

#[tauri::command]
pub async fn detect_game(app: AppHandle, path: String) -> Result<DetectResult> {
    blocking(move || {
        let handle = app.clone();
        let report = |step: &str, message: &str| emit(&handle, step, message);
        game::detect(&path, Some(&report))
    })
    .await
}

#[tauri::command]
pub async fn install_library(
    app: AppHandle,
    data: State<'_, AppState>,
    path: String,
) -> Result<InstallResult> {
    let record = data.snapshot()?.games.get(&path).cloned();
    let task_path = path.clone();
    let installed = blocking(move || {
        let handle = app.clone();
        let report = |step: &str, message: &str| emit(&handle, step, message);
        game::install(&task_path, None, record.as_ref(), Some(&report))
    })
    .await?;
    data.update(|state| {
        let record = state
            .games
            .get_mut(&path)
            .ok_or_else(|| CommandError::new(ErrorKind::NotFound, "Game is not in the library."))?;
        record.arch = installed.arch.clone();
        record.era = installed.era.clone();
        record.interfaces = installed.interfaces.clone();
        record.original_library = installed.original_library.clone();
        record.library = Some(installed.library.clone());
        Ok(())
    })?;
    Ok(installed)
}

#[tauri::command]
pub async fn restore_library(
    app: AppHandle,
    data: State<'_, AppState>,
    path: String,
) -> Result<Option<String>> {
    let record = data.snapshot()?.games.get(&path).cloned();
    let task_path = path.clone();
    let restored = blocking(move || {
        let handle = app.clone();
        let report = |step: &str, message: &str| emit(&handle, step, message);
        game::restore(&task_path, record.as_ref(), Some(&report))
    })
    .await?;
    data.update(|state| {
        if let Some(record) = state.games.get_mut(&path) {
            record.original_library = None;
            record.library = None;
        }
        Ok(())
    })?;
    Ok(restored)
}

#[tauri::command]
pub async fn configure_game(
    app: AppHandle,
    data: State<'_, AppState>,
    path: String,
    appid: u32,
) -> Result<ConfigureResult> {
    let server = data.snapshot()?.server.ok_or_else(|| {
        CommandError::new(
            ErrorKind::InvalidServer,
            "Sign in before configuring a game.",
        )
    })?;
    let task_path = path.clone();
    let configured = blocking(move || {
        let handle = app.clone();
        let report = |step: &str, message: &str| emit(&handle, step, message);
        game::configure(&task_path, appid, &server, Some(&report))
    })
    .await?;
    data.update(|state| {
        let record = state
            .games
            .get_mut(&path)
            .ok_or_else(|| CommandError::new(ErrorKind::NotFound, "Game is not in the library."))?;
        record.appid = configured.appid;
        record.configured = true;
        Ok(())
    })?;
    Ok(configured)
}

#[tauri::command]
pub async fn launch_game(data: State<'_, AppState>, path: String) -> Result<u32> {
    let snapshot = data.snapshot()?;
    let record = snapshot
        .games
        .get(&path)
        .cloned()
        .ok_or_else(|| CommandError::new(ErrorKind::NotFound, "Game is not in the library."))?;
    let store = data.store.clone();
    let token = blocking(move || store.get()).await?.ok_or_else(|| {
        CommandError::new(
            ErrorKind::TokenExpired,
            "The session has ended. Sign in again.",
        )
    })?;
    if auth::is_expired(snapshot.token_expires_at) {
        return Err(CommandError::new(
            ErrorKind::TokenExpired,
            "The session has expired. Sign in again.",
        ));
    }
    let username = snapshot.username.clone();
    let task_path = path.clone();
    let pid =
        blocking(move || game::launch(&task_path, &record, username.as_deref(), &token)).await?;
    data.update(|state| {
        if let Some(record) = state.games.get_mut(&path) {
            record.last_played = Some(auth::now_secs());
        }
        Ok(())
    })?;
    Ok(pid)
}

#[tauri::command]
pub async fn unpack_steamstub(app: AppHandle, path: String) -> Result<Option<String>> {
    blocking(move || {
        let handle = app.clone();
        let report = |step: &str, message: &str| emit(&handle, step, message);
        game::unpack_steamstub(&path, Some(&report))
    })
    .await
}

#[tauri::command]
pub async fn login(
    data: State<'_, AppState>,
    server: String,
    username: String,
    password: String,
) -> Result<SessionInfo> {
    let store = data.store.clone();
    let login_name = username.clone();
    let session = blocking(move || auth::login(&server, &login_name, &password, &store)).await?;
    data.update(|state| {
        state.server = Some(session.server.clone());
        state.username = Some(username);
        state.steam_id = Some(session.steam_id);
        state.persona_name = Some(session.persona_name.clone());
        state.token_expires_at = Some(session.expires_at);
        Ok(())
    })?;
    Ok(session)
}

#[tauri::command]
pub async fn sign_out(data: State<'_, AppState>) -> Result<()> {
    let store = data.store.clone();
    blocking(move || store.clear()).await?;
    data.update(|state| {
        state.steam_id = None;
        state.persona_name = None;
        state.token_expires_at = None;
        Ok(())
    })
}

#[tauri::command]
pub async fn test_connection(server: String) -> Result<()> {
    blocking(move || auth::test_connection(&server)).await
}

#[tauri::command]
pub async fn library_list(data: State<'_, AppState>) -> Result<Vec<GameRecord>> {
    Ok(data.snapshot()?.games.into_values().collect())
}

#[tauri::command]
pub async fn library_add(
    app: AppHandle,
    data: State<'_, AppState>,
    path: String,
) -> Result<GameRecord> {
    let detected = blocking(move || {
        let handle = app.clone();
        let report = |step: &str, message: &str| emit(&handle, step, message);
        game::detect(&path, Some(&report))
    })
    .await?;
    let mut record = GameRecord::new(&detected.exe_path);
    record.arch = detected.arch;
    record.era = detected.era;
    record.interfaces = detected.interfaces;
    record.original_library = detected.original_library;
    data.update(|state| Ok(add_record(state, record)))
}

#[tauri::command]
pub async fn library_remove(app: AppHandle, data: State<'_, AppState>, path: String) -> Result<()> {
    let record = data
        .snapshot()?
        .games
        .get(&path)
        .cloned()
        .ok_or_else(|| CommandError::new(ErrorKind::NotFound, "Game is not in the library."))?;
    let path_for_task = path.clone();
    blocking(move || {
        let handle = app.clone();
        let report = |step: &str, message: &str| emit(&handle, step, message);
        match game::restore(&path_for_task, Some(&record), Some(&report)) {
            Ok(_) => Ok(()),
            Err(error) if error.kind == ErrorKind::NotFound => {
                game::remove_library(&path_for_task, &record)
            }
            Err(error) => Err(error),
        }
    })
    .await?;
    data.update(|state| {
        remove_record(state, &path);
        Ok(())
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn adding_and_removing_records_touches_only_the_library() {
        let mut state = LauncherState {
            server: Some("127.0.0.1:8081".into()),
            ..Default::default()
        };
        let record = add_record(&mut state, GameRecord::new("/games/a.exe"));
        assert_eq!(record.exe_path, "/games/a.exe");
        assert!(state.games.contains_key("/games/a.exe"));
        add_record(&mut state, GameRecord::new("/games/a.exe"));
        assert_eq!(state.games.len(), 1);
        assert_eq!(state.server.as_deref(), Some("127.0.0.1:8081"));
        assert!(remove_record(&mut state, "/games/a.exe").is_some());
        assert!(state.games.is_empty());
    }

    #[test]
    fn state_has_no_secret_fields() {
        let state = LauncherState::default();
        let json = serde_json::to_string(&state).unwrap();
        for forbidden in ["\"password\"", "\"session_token\"", "\"token\""] {
            assert!(!json.contains(forbidden));
        }
    }
}
