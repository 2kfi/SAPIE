#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

mod auth;
mod commands;
mod detection;
mod error;
mod game;
mod state;

use auth::KeychainTokenStore;
use commands::AppState;
use std::sync::Arc;

fn main() {
    tauri::Builder::default()
        .manage(
            AppState::new(Arc::new(KeychainTokenStore))
                .expect("SAPIE Launcher could not read its local state"),
        )
        .invoke_handler(tauri::generate_handler![
            commands::detect_game,
            commands::install_library,
            commands::restore_library,
            commands::configure_game,
            commands::launch_game,
            commands::unpack_steamstub,
            commands::login,
            commands::sign_out,
            commands::test_connection,
            commands::library_list,
            commands::library_add,
            commands::library_remove,
        ])
        .run(tauri::generate_context!())
        .expect("SAPIE Launcher failed to start");
}
