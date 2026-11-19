#[tauri::command]
fn pick_executable() -> Result<String, String> {
    Err("Not implemented in M0: executable picker".into())
}

#[tauri::command]
fn copy_dll() -> Result<(), String> {
    Err("Not implemented in M0: DLL copy".into())
}

#[tauri::command]
fn set_ld_preload() -> Result<(), String> {
    Err("Not implemented in M0: LD_PRELOAD setup".into())
}

#[tauri::command]
fn launch_game() -> Result<(), String> {
    Err("Not implemented in M0: game launch".into())
}

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            pick_executable,
            copy_dll,
            set_ld_preload,
            launch_game,
        ])
        .run(tauri::generate_context!())
        .expect("error while running SAPIE launcher");
}
