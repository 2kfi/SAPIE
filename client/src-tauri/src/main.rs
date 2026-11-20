#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

#[allow(dead_code)]
mod detection;

fn main() {
    tauri::Builder::default()
        .run(tauri::generate_context!())
        .expect("SAPIE Launcher failed to start");
}
