use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use serde::{Deserialize, Serialize};
use tauri::Manager;

#[derive(Clone, Serialize, Deserialize)]
struct Connection {
    server: String,
    username: String,
    password: String,
}

#[derive(Clone, Serialize, Deserialize)]
struct GameRecord {
    exe_path: String,
    arch: String,
    era: String,
    interfaces: Vec<String>,
    steam_api_path: Option<String>,
    dll_path: Option<String>,
    ld_preload: Option<String>,
    configured: bool,
}

#[derive(Default, Serialize, Deserialize)]
struct PersistedState {
    server: Option<Connection>,
    games: BTreeMap<String, GameRecord>,
}

#[derive(Serialize)]
struct DetectResult {
    arch: String,
    era: String,
    interfaces: Vec<String>,
    steam_api_path: Option<String>,
}

const INTERFACE_NAMES: &[&str] = &[
    "SteamClient",
    "SteamGameServer",
    "SteamGameServerStats",
    "SteamUser",
    "SteamFriends",
    "SteamUtils",
    "SteamMatchMaking",
    "SteamMatchMakingServers",
    "STEAMUSERSTATS_INTERFACE_VERSION",
    "STEAMAPPS_INTERFACE_VERSION",
    "SteamNetworking",
    "STEAMREMOTESTORAGE_INTERFACE_VERSION",
    "STEAMSCREENSHOTS_INTERFACE_VERSION",
    "STEAMHTTP_INTERFACE_VERSION",
    "STEAMUNIFIEDMESSAGES_INTERFACE_VERSION",
    "STEAMUGC_INTERFACE_VERSION",
    "STEAMAPPLIST_INTERFACE_VERSION",
    "STEAMMUSIC_INTERFACE_VERSION",
    "STEAMMUSICREMOTE_INTERFACE_VERSION",
    "STEAMHTMLSURFACE_INTERFACE_VERSION_",
    "STEAMINVENTORY_INTERFACE_V",
    "SteamController",
    "SteamMasterServerUpdater",
    "STEAMVIDEO_INTERFACE_V",
];

fn state_path(app: &tauri::AppHandle) -> Result<PathBuf, String> {
    let dir = app.path().app_data_dir().map_err(|e| e.to_string())?;
    fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    Ok(dir.join("launcher-state.json"))
}

fn load_state(app: &tauri::AppHandle) -> Result<PersistedState, String> {
    let path = state_path(app)?;
    if !path.is_file() {
        return Ok(PersistedState::default());
    }
    let text = fs::read_to_string(path).map_err(|e| e.to_string())?;
    serde_json::from_str(&text).map_err(|e| e.to_string())
}

fn save_state(app: &tauri::AppHandle, state: &PersistedState) -> Result<(), String> {
    let text = serde_json::to_string_pretty(state).map_err(|e| e.to_string())?;
    fs::write(state_path(app)?, text).map_err(|e| e.to_string())
}

fn new_record(exe_path: &str) -> GameRecord {
    GameRecord {
        exe_path: exe_path.into(),
        arch: "unknown".into(),
        era: "unknown".into(),
        interfaces: Vec::new(),
        steam_api_path: None,
        dll_path: None,
        ld_preload: None,
        configured: false,
    }
}

fn detect_arch(bytes: &[u8]) -> String {
    if bytes.len() >= 0x40 && &bytes[..2] == b"MZ" {
        let pe = u32::from_le_bytes(bytes[0x3c..0x40].try_into().unwrap()) as usize;
        if pe + 6 <= bytes.len() && &bytes[pe..pe + 4] == b"PE\0\0" {
            let machine = u16::from_le_bytes(bytes[pe + 4..pe + 6].try_into().unwrap());
            return match machine {
                0x014c => "x86",
                0x8664 => "x64",
                0xaa64 => "arm64",
                _ => "unknown",
            }
            .into();
        }
    }
    if bytes.len() >= 20 && &bytes[..4] == b"\x7fELF" {
        return match bytes[4] {
            1 => "x86",
            2 if u16::from_le_bytes([bytes[18], bytes[19]]) == 0xb7 => "arm64",
            2 => "x64",
            _ => "unknown",
        }
        .into();
    }
    "unknown".into()
}

fn find_all(haystack: &[u8], needle: &[u8]) -> Vec<usize> {
    if needle.is_empty() || haystack.len() < needle.len() {
        return Vec::new();
    }
    (0..=haystack.len() - needle.len())
        .filter(|&i| &haystack[i..i + needle.len()] == needle)
        .collect()
}

// Mirrors client/generate_interfaces_file.cpp: literal name + exactly three digits.
fn scan_interfaces(bytes: &[u8]) -> Vec<String> {
    let mut found = Vec::new();
    for name in INTERFACE_NAMES {
        for start in find_all(bytes, name.as_bytes()) {
            let end = start + name.len();
            if end + 3 <= bytes.len() && bytes[end..end + 3].iter().all(u8::is_ascii_digit) {
                let value = String::from_utf8_lossy(&bytes[start..end + 3]).into_owned();
                if !found.contains(&value) {
                    found.push(value);
                }
            }
        }
    }

    let key = b"STEAMCONTROLLER_INTERFACE_VERSION";
    if !found.iter().any(|s| s.starts_with("STEAMCONTROLLER_INTERFACE_VERSION")) {
        if let Some(&start) = find_all(bytes, key).first() {
            let end = start + key.len();
            let digits = if end + 3 <= bytes.len()
                && bytes[end..end + 3].iter().all(u8::is_ascii_digit)
            {
                3
            } else {
                0
            };
            found.push(String::from_utf8_lossy(&bytes[start..end + digits]).into_owned());
        }
    }
    found
}

// Display-only: SAPIE_INTERFACE_ERA is a no-op in the DLL, so this never picks a DLL variant.
fn era_from_interfaces(interfaces: &[String]) -> String {
    let version = interfaces
        .iter()
        .find_map(|s| s.strip_prefix("SteamUser"))
        .and_then(|v| v.parse::<u32>().ok())
        .or_else(|| {
            interfaces
                .iter()
                .find_map(|s| s.strip_prefix("SteamClient"))
                .and_then(|v| v.parse::<u32>().ok())
        });
    match version {
        Some(v) if v < 18 => "legacy",
        Some(_) => "modern",
        None => "unknown",
    }
    .into()
}

fn find_original_steam_api(dir: &Path) -> Option<PathBuf> {
    for name in ["steam_api.dll", "steam_api64.dll", "libsteam_api.so"] {
        let path = dir.join(name);
        if path.is_file() {
            return Some(path);
        }
    }
    let mut candidates: Vec<PathBuf> = fs::read_dir(dir)
        .ok()?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| {
            path.is_file()
                && path
                    .file_name()
                    .and_then(|n| n.to_str())
                    .map(|n| {
                        let lower = n.to_ascii_lowercase();
                        !lower.contains(".sapie-original")
                            && ((lower.starts_with("steam_api") && lower.ends_with(".dll"))
                                || (lower.starts_with("libsteam_api") && lower.ends_with(".so")))
                    })
                    .unwrap_or(false)
        })
        .collect();
    candidates.sort();
    candidates.into_iter().next()
}

fn steamless_dir(app: &tauri::AppHandle) -> Option<PathBuf> {
    let root = app.path().resource_dir().ok()?;
    for candidate in [root.join("resources/steamless"), root.join("steamless")] {
        if candidate.is_dir() {
            return Some(candidate);
        }
    }
    None
}

fn dll_search_dirs(app: &tauri::AppHandle, target: &str, era: &str) -> Vec<PathBuf> {
    let suffix = format!("{target}-{era}");
    let mut dirs = Vec::new();
    if let Ok(root) = app.path().resource_dir() {
        dirs.push(root.join("steam_api").join(&suffix));
        dirs.push(root.join("resources/steam_api").join(&suffix));
    }
    let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    if let Some(repo) = manifest.parent().and_then(Path::parent) {
        dirs.push(repo.join("out").join(&suffix));
    }
    dirs
}

fn validate_server(server: &str) -> Result<(), String> {
    let server = server.trim();
    let (host, port) = server
        .split_once(':')
        .ok_or("Server must be in host:port form (for example 127.0.0.1:7777).")?;
    if host.is_empty() || host.chars().any(char::is_whitespace) {
        return Err("Server host is empty or contains spaces.".into());
    }
    let port = port
        .parse::<u16>()
        .map_err(|_| format!("Invalid server port: {port}"))?;
    if port == 0 {
        return Err("Server port must be between 1 and 65535.".into());
    }
    Ok(())
}

#[tauri::command]
fn pick_executable(app: tauri::AppHandle) -> Result<Option<String>, String> {
    let (tx, rx) = std::sync::mpsc::channel();
    app.run_on_main_thread(move || {
        let picked = rfd::FileDialog::new().pick_file();
        let _ = tx.send(picked.map(|path| path.to_string_lossy().into_owned()));
    })
    .map_err(|e| e.to_string())?;
    rx.recv().map_err(|e| e.to_string())
}

#[tauri::command]
fn detect_game(exe_path: String, app: tauri::AppHandle) -> Result<DetectResult, String> {
    let exe = PathBuf::from(&exe_path);
    if !exe.is_file() {
        return Err(format!("Executable not found: {exe_path}"));
    }
    let dir = exe
        .parent()
        .ok_or_else(|| format!("Executable has no parent directory: {exe_path}"))?;
    let arch = detect_arch(&fs::read(&exe).map_err(|e| e.to_string())?);
    let steam_api = find_original_steam_api(dir);
    let interfaces = match &steam_api {
        Some(path) => scan_interfaces(&fs::read(path).map_err(|e| e.to_string())?),
        None => Vec::new(),
    };
    if interfaces.is_empty() {
        return Err(
            "No Steam interface versions found next to this executable. Keep the original steam_api DLL beside it and try again."
                .into(),
        );
    }
    fs::write(
        dir.join("steam_interfaces.txt"),
        format!("{}\n", interfaces.join("\n")),
    )
    .map_err(|e| e.to_string())?;
    let era = era_from_interfaces(&interfaces);
    let steam_api_path = steam_api.map(|p| p.to_string_lossy().into_owned());

    let mut state = load_state(&app)?;
    let record = state.games.entry(exe_path.clone()).or_insert_with(|| new_record(&exe_path));
    record.arch = arch.clone();
    record.era = era.clone();
    record.interfaces = interfaces.clone();
    record.steam_api_path = steam_api_path.clone();
    save_state(&app, &state)?;

    Ok(DetectResult {
        arch,
        era,
        interfaces,
        steam_api_path,
    })
}

#[tauri::command]
fn unpack_steamstub(exe_path: String, app: tauri::AppHandle) -> Result<Option<String>, String> {
    if cfg!(not(target_os = "windows")) {
        return Err("SteamStub unpacking is available on Windows only.".into());
    }
    let exe = PathBuf::from(&exe_path);
    if !exe.is_file() {
        return Err(format!("Executable not found: {exe_path}"));
    }
    let dir = steamless_dir(&app)
        .ok_or_else(|| "Steamless resource folder is missing from this launcher.".to_string())?;
    let cli = dir.join("Steamless.CLI.exe");
    if !cli.is_file() {
        return Err(format!("Steamless CLI not found: {}", cli.display()));
    }
    let output = Command::new(&cli)
        .current_dir(&dir)
        .arg(&exe)
        .output()
        .map_err(|e| format!("Could not run Steamless CLI: {e}"))?;
    let text = format!(
        "{}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    if text.contains("Successfully unpacked file!") {
        let stem = exe
            .file_stem()
            .and_then(|s| s.to_str())
            .ok_or("Could not read executable name.")?;
        let unpacked = exe.with_file_name(format!("{stem}.unpacked.exe"));
        if unpacked.is_file() {
            return Ok(Some(unpacked.to_string_lossy().into_owned()));
        }
        return Err("Steamless reported success but produced no .unpacked.exe file.".into());
    }
    if text.contains("Failed to unpack file.")
        || text.contains("All unpackers failed")
        || text.contains("Invalid input file given")
    {
        return Ok(None);
    }
    Err(format!(
        "Steamless gave no clear result (exit {:?}): {}",
        output.status.code(),
        text.trim()
    ))
}

#[tauri::command]
fn copy_dll(exe_path: String, era: String, app: tauri::AppHandle) -> Result<String, String> {
    if era != "legacy" && era != "modern" {
        return Err("Game era could not be determined; refusing to guess a DLL.".into());
    }
    let mut state = load_state(&app)?;
    let record = state
        .games
        .get(&exe_path)
        .ok_or("Detect the game before copying the DLL.")?;
    let arch = if record.arch == "unknown" {
        let exe = PathBuf::from(&exe_path);
        detect_arch(&fs::read(&exe).map_err(|e| e.to_string())?)
    } else {
        record.arch.clone()
    };
    let (target, dll_name) = match (std::env::consts::OS, arch.as_str()) {
        ("windows", "x86") => ("windows-x86", "steam_api.dll"),
        ("windows", "x64") => ("windows-x64", "steam_api64.dll"),
        ("linux", "x86") => ("linux-x86", "libsteam_api.so"),
        ("linux", "x64") => ("linux-x64", "libsteam_api.so"),
        ("linux", "arm64") => ("linux-arm64", "libsteam_api.so"),
        (os, detected) => {
            return Err(format!("No SAPIE client artifact for {os}/{detected}."));
        }
    };
    let dirs = dll_search_dirs(&app, target, &era);
    let source = dirs
        .iter()
        .map(|dir| dir.join(dll_name))
        .find(|path| path.is_file())
        .ok_or_else(|| {
            format!(
                "SAPIE DLL not found for {target}/{era}. Build the client matrix and place {dll_name} in one of: {}",
                dirs.iter()
                    .map(|p| p.display().to_string())
                    .collect::<Vec<_>>()
                    .join(", ")
            )
        })?;
    let exe = PathBuf::from(&exe_path);
    let dir = exe
        .parent()
        .ok_or_else(|| format!("Executable has no parent directory: {exe_path}"))?;
    for name in ["steam_api.dll", "steam_api64.dll", "libsteam_api.so"] {
        let original = dir.join(name);
        if original.is_file() {
            let backup = dir.join(format!("{name}.sapie-original.bak"));
            if !backup.exists() {
                fs::rename(&original, &backup).map_err(|e| e.to_string())?;
            }
        }
    }
    let destination = dir.join(dll_name);
    fs::copy(&source, &destination).map_err(|e| e.to_string())?;
    let dest_string = destination.to_string_lossy().into_owned();
    let record = state
        .games
        .get_mut(&exe_path)
        .ok_or("Detect the game before copying the DLL.")?;
    record.dll_path = Some(dest_string.clone());
    save_state(&app, &state)?;
    Ok(dest_string)
}

#[tauri::command]
fn set_ld_preload(exe_path: String, lib_path: String, app: tauri::AppHandle) -> Result<(), String> {
    let lib = PathBuf::from(&lib_path);
    if !lib.is_file() {
        return Err(format!("Library not found: {lib_path}"));
    }
    let mut state = load_state(&app)?;
    let record = state
        .games
        .get_mut(&exe_path)
        .ok_or("Detect the game before setting LD_PRELOAD.")?;
    record.ld_preload = Some(lib_path);
    save_state(&app, &state)
}

#[tauri::command]
fn configure_server(
    exe_path: String,
    server: String,
    username: String,
    password: String,
    app: tauri::AppHandle,
) -> Result<(), String> {
    validate_server(&server)?;
    if username.trim().is_empty() {
        return Err("Username is required.".into());
    }
    if password.is_empty() {
        return Err("Password is required.".into());
    }
    let exe = PathBuf::from(&exe_path);
    if !exe.is_file() {
        return Err(format!("Executable not found: {exe_path}"));
    }
    let dir = exe
        .parent()
        .ok_or_else(|| format!("Executable has no parent directory: {exe_path}"))?;
    let server = server.trim().to_string();
    fs::write(dir.join("sapie_server.txt"), format!("{server}\n"))
        .map_err(|e| e.to_string())?;

    let mut state = load_state(&app)?;
    state.server = Some(Connection {
        server: server.clone(),
        username,
        password,
    });
    let record = state
        .games
        .entry(exe_path.clone())
        .or_insert_with(|| new_record(&exe_path));
    record.configured = true;
    save_state(&app, &state)
}

#[tauri::command]
fn launch_game(exe_path: String, app: tauri::AppHandle) -> Result<(), String> {
    let state = load_state(&app)?;
    let record = state
        .games
        .get(&exe_path)
        .ok_or("Add and configure this game before launching.")?;
    if !record.configured {
        return Err("Connect to a server and configure this game before launching.".into());
    }
    let connection = state
        .server
        .as_ref()
        .ok_or("No server connection saved. Connect first.")?;
    let exe = PathBuf::from(&exe_path);
    if !exe.is_file() {
        return Err(format!("Executable not found: {exe_path}"));
    }
    let dir = exe
        .parent()
        .ok_or_else(|| format!("Executable has no parent directory: {exe_path}"))?;
    let mut command = Command::new(&exe);
    command
        .current_dir(dir)
        .env("SAPIE_USERNAME", &connection.username)
        .env("SAPIE_PASSWORD", &connection.password);
    if std::env::consts::OS == "linux" {
        if let Some(lib) = &record.ld_preload {
            command.env("LD_PRELOAD", lib);
        }
    }
    command
        .spawn()
        .map_err(|e| format!("Could not start the game: {e}"))?;
    Ok(())
}

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            pick_executable,
            detect_game,
            unpack_steamstub,
            copy_dll,
            set_ld_preload,
            configure_server,
            launch_game,
        ])
        .run(tauri::generate_context!())
        .expect("error while running SAPIE launcher");
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn scans_interface_versions_like_goldberg() {
        let bytes = b"\x00SteamClient017\x00SteamUser019\x00SteamUserStats001\x00STEAMCONTROLLER_INTERFACE_VERSION\x00";
        let found = scan_interfaces(bytes);
        assert!(found.contains(&"SteamClient017".to_string()));
        assert!(found.contains(&"SteamUser019".to_string()));
        assert!(found.contains(&"STEAMCONTROLLER_INTERFACE_VERSION".to_string()));
        assert!(!found.iter().any(|s| s.starts_with("SteamUserStats")));
        assert_eq!(era_from_interfaces(&found), "modern");

        let legacy = scan_interfaces(b"SteamUser016\0SteamFriends015\0");
        assert_eq!(era_from_interfaces(&legacy), "legacy");
    }

    #[test]
    fn parses_pe_and_elf_architectures() {
        let mut pe = vec![0u8; 0x100];
        pe[..2].copy_from_slice(b"MZ");
        pe[0x3c..0x40].copy_from_slice(&0x80u32.to_le_bytes());
        pe[0x80..0x84].copy_from_slice(b"PE\0\0");
        for (machine, expected) in [(0x014cu16, "x86"), (0x8664, "x64"), (0xaa64, "arm64")] {
            pe[0x84..0x86].copy_from_slice(&machine.to_le_bytes());
            assert_eq!(detect_arch(&pe), expected);
        }

        let mut elf = vec![0u8; 20];
        elf[..4].copy_from_slice(b"\x7fELF");
        elf[4] = 1;
        assert_eq!(detect_arch(&elf), "x86");
        elf[4] = 2;
        elf[18..20].copy_from_slice(&0xb7u16.to_le_bytes());
        assert_eq!(detect_arch(&elf), "arm64");
        elf[18..20].copy_from_slice(&0x3eu16.to_le_bytes());
        assert_eq!(detect_arch(&elf), "x64");
    }
}
