#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use eframe::egui;
use serde::{Deserialize, Serialize};

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

const BG: egui::Color32 = egui::Color32::from_rgb(0x11, 0x19, 0x27);
const SURFACE: egui::Color32 = egui::Color32::from_rgb(0x1b, 0x24, 0x32);
const FIELD: egui::Color32 = egui::Color32::from_rgb(0x0b, 0x11, 0x1c);
const TEXT: egui::Color32 = egui::Color32::from_rgb(0xe9, 0xef, 0xf7);
const MUTED: egui::Color32 = egui::Color32::from_rgb(0xb3, 0xc3, 0xd6);
const EMBER: egui::Color32 = egui::Color32::from_rgb(0xff, 0x9b, 0x6a);
const OK: egui::Color32 = egui::Color32::from_rgb(0x8f, 0xe3, 0xb0);
const ERROR: egui::Color32 = egui::Color32::from_rgb(0xff, 0xb3, 0xa7);

fn state_path() -> Result<PathBuf, String> {
    let dir = dirs::data_dir()
        .ok_or("Could not find the per-user data directory.")?
        .join("com.sapie.launcher");
    fs::create_dir_all(&dir).map_err(|e| e.to_string())?;
    Ok(dir.join("launcher-state.json"))
}

fn load_state() -> Result<PersistedState, String> {
    let path = state_path()?;
    if !path.is_file() {
        return Ok(PersistedState::default());
    }
    let text = fs::read_to_string(path).map_err(|e| e.to_string())?;
    serde_json::from_str(&text).map_err(|e| e.to_string())
}

fn save_state(state: &PersistedState) -> Result<(), String> {
    let text = serde_json::to_string_pretty(state).map_err(|e| e.to_string())?;
    fs::write(state_path()?, text).map_err(|e| e.to_string())
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
        // Prefer the preserved original: the current DLL may be our own copy.
        let backup = dir.join(format!("{name}.sapie-original.bak"));
        if backup.is_file() {
            return Some(backup);
        }
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

fn executable_dir() -> Option<PathBuf> {
    std::env::current_exe().ok()?.parent().map(Path::to_path_buf)
}

fn steamless_dir() -> Option<PathBuf> {
    let mut dirs = Vec::new();
    if let Some(root) = executable_dir() {
        dirs.push(root.join("steamless"));
        dirs.push(root.join("resources/steamless"));
    }
    dirs.push(PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("resources/steamless"));
    dirs.into_iter().find(|dir| dir.is_dir())
}

fn dll_search_dirs(target: &str, era: &str) -> Vec<PathBuf> {
    let suffix = format!("{target}-{era}");
    let mut dirs = Vec::new();
    if let Some(root) = executable_dir() {
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

fn pick_executable() -> Option<String> {
    rfd::FileDialog::new()
        .pick_file()
        .map(|path| path.to_string_lossy().into_owned())
}

fn detect_game(exe_path: String) -> Result<DetectResult, String> {
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

    let mut state = load_state()?;
    let record = state.games.entry(exe_path.clone()).or_insert_with(|| new_record(&exe_path));
    record.arch = arch.clone();
    record.era = era.clone();
    record.interfaces = interfaces.clone();
    record.steam_api_path = steam_api_path.clone();
    save_state(&state)?;

    Ok(DetectResult {
        arch,
        era,
        interfaces,
        steam_api_path,
    })
}

fn unpack_steamstub(exe_path: String) -> Result<Option<String>, String> {
    if cfg!(not(target_os = "windows")) {
        return Err("SteamStub unpacking is available on Windows only.".into());
    }
    let exe = PathBuf::from(&exe_path);
    if !exe.is_file() {
        return Err(format!("Executable not found: {exe_path}"));
    }
    let dir = steamless_dir()
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

fn copy_dll(exe_path: String, era: String) -> Result<String, String> {
    if era != "legacy" && era != "modern" {
        return Err("Game era could not be determined; refusing to guess a DLL.".into());
    }
    let mut state = load_state()?;
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
    let dirs = dll_search_dirs(target, &era);
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
    save_state(&state)?;
    Ok(dest_string)
}

fn set_ld_preload(exe_path: String, lib_path: String) -> Result<(), String> {
    let lib = PathBuf::from(&lib_path);
    if !lib.is_file() {
        return Err(format!("Library not found: {lib_path}"));
    }
    let mut state = load_state()?;
    let record = state
        .games
        .get_mut(&exe_path)
        .ok_or("Detect the game before setting LD_PRELOAD.")?;
    record.ld_preload = Some(lib_path);
    save_state(&state)
}

fn configure_server(
    exe_path: String,
    server: String,
    username: String,
    password: String,
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

    let mut state = load_state()?;
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
    save_state(&state)
}

fn launch_game(exe_path: String) -> Result<(), String> {
    let state = load_state()?;
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

#[derive(Clone, Copy, PartialEq)]
enum Tab {
    Connect,
    Games,
}

struct LogLine {
    level: &'static str,
    text: String,
}

struct LauncherApp {
    tab: Tab,
    state: PersistedState,
    selected: Option<String>,
    connection: Connection,
    remember: bool,
    password_visible: bool,
    preload: String,
    log: Vec<LogLine>,
    error: Option<String>,
}

impl LauncherApp {
    fn new() -> Self {
        let (initial_state, load_error) = match load_state() {
            Ok(state) => (state, None),
            Err(err) => (PersistedState::default(), Some(err)),
        };
        let connection = initial_state
            .server
            .clone()
            .unwrap_or_else(|| Connection { server: String::new(), username: String::new(), password: String::new() });
        let remember = initial_state.server.is_some();
        let mut app = Self {
            tab: Tab::Connect,
            state: PersistedState::default(),
            selected: None,
            connection,
            remember,
            password_visible: false,
            preload: String::new(),
            log: Vec::new(),
            error: None,
        };
        app.log("INFO", "Ready. Add a game executable to begin.".into());
        if let Some(err) = load_error {
            app.log("ERROR", format!("Could not load saved launcher state: {err}"));
        }
        app.state = initial_state;
        app
    }

    fn log(&mut self, level: &'static str, text: String) {
        self.log.push(LogLine { level, text });
    }

    fn result(&mut self, action: &str, result: Result<String, String>) {
        match result {
            Ok(text) => {
                self.error = None;
                self.log("OK", format!("{action}: {text}"));
            }
            Err(err) => {
                let text = format!("{action}: {err}");
                self.error = Some(text.clone());
                self.log("ERROR", text);
            }
        }
        self.refresh();
    }

    fn refresh(&mut self) {
        match load_state() {
            Ok(state) => self.state = state,
            Err(err) => self.log("ERROR", format!("Could not reload launcher state: {err}")),
        }
    }

    fn connect(&mut self) {
        let conn = self.connection.clone();
        let result = validate_server(&conn.server)
            .and_then(|_| if conn.username.trim().is_empty() { Err("Username is required.".into()) } else { Ok(()) })
            .and_then(|_| if conn.password.is_empty() { Err("Password is required.".into()) } else { Ok(()) });
        if let Err(err) = result {
            self.result("Connect", Err(err));
            return;
        }
        if self.remember {
            let mut state = match load_state() {
                Ok(state) => state,
                Err(err) => {
                    self.result("Connect", Err(err));
                    return;
                }
            };
            state.server = Some(conn);
            self.result("Connect", save_state(&state).map(|_| "Connection details saved on this device.".into()));
        } else {
            let server = conn.server.trim().to_string();
            self.result("Connect", Ok(format!("Connection to {server} ready for this session.")));
        }
    }

    fn add_game(&mut self) {
        if let Some(path) = pick_executable() {
            self.selected = Some(path.clone());
            let result = detect_game(path.clone()).map(|info| {
                format!("{} detected as {}/{} with {} interfaces", Path::new(&path).file_name().map(|n| n.to_string_lossy().into_owned()).unwrap_or(path), info.arch, info.era, info.interfaces.len())
            });
            self.result("Detect", result);
        } else {
            self.log("INFO", "No executable selected.".into());
        }
    }

    fn unpack(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        match unpack_steamstub(path.clone()) {
            Ok(Some(unpacked)) => {
                self.selected = Some(unpacked.clone());
                self.log("OK", format!("Steamless: unpacked to {unpacked}"));
                let result = detect_game(unpacked).map(|info| {
                    format!("{} detected as {}/{}", info.arch, info.era, info.interfaces.join(", "))
                });
                self.result("Detect unpacked executable", result);
            }
            Ok(None) => self.result("Steamless", Ok("No SteamStub found in this executable.".into())),
            Err(err) => self.result("Steamless", Err(err)),
        }
    }

    fn install_dll(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        let era = self.state.games.get(&path).map(|r| r.era.clone()).unwrap_or_default();
        self.result("Install DLL", copy_dll(path, era));
    }

    fn configure(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        let conn = self.connection.clone();
        self.result("Configure server", configure_server(path, conn.server, conn.username, conn.password).map(|_| "sapie_server.txt written.".into()));
    }

    fn set_preload(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        self.result("Set LD_PRELOAD", set_ld_preload(path, self.preload.clone()).map(|_| "Library recorded for Linux launches.".into()));
    }

    fn launch(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        self.result("Launch", launch_game(path).map(|_| "Game process started.".into()));
    }

    fn ui_connect(&mut self, ui: &mut egui::Ui) {
        ui.heading("Connect to a server");
        ui.add_space(6.0);
        ui.label(egui::RichText::new("These details are sent to the game as SAPIE_USERNAME and SAPIE_PASSWORD when you launch it.").color(MUTED));
        ui.add_space(10.0);

        egui::Grid::new("connect-grid")
            .num_columns(2)
            .spacing([12.0, 10.0])
            .show(ui, |ui| {
                ui.label("Server (host:port)");
                ui.add(egui::TextEdit::singleline(&mut self.connection.server).desired_width(360.0).hint_text("127.0.0.1:7777"));
                ui.end_row();

                ui.label("Username");
                ui.add(egui::TextEdit::singleline(&mut self.connection.username).desired_width(220.0));
                ui.end_row();

                ui.label("Password");
                ui.horizontal(|ui| {
                    ui.add(egui::TextEdit::singleline(&mut self.connection.password).password(!self.password_visible).desired_width(220.0));
                    if ui.button(if self.password_visible { "Hide" } else { "Show" }).clicked() {
                        self.password_visible = !self.password_visible;
                    }
                });
                ui.end_row();
            });

        ui.add_space(10.0);
        ui.checkbox(&mut self.remember, "Remember details on this device");
        ui.add_space(10.0);
        if ui.button("Connect").clicked() {
            self.connect();
            self.tab = Tab::Games;
        }

        ui.add_space(16.0);
        ui.separator();
        ui.add_space(8.0);
        ui.label(egui::RichText::new("Saved connection").strong());
        match &self.state.server {
            Some(conn) => {
                ui.label(format!("Server: {}", conn.server));
                ui.label(format!("Username: {}", conn.username));
                ui.label(egui::RichText::new("Password: saved in launcher-state.json").color(MUTED));
            }
            None => {
                ui.label(egui::RichText::new("No connection saved yet.").color(MUTED));
            }
        }
    }

    fn ui_games(&mut self, ui: &mut egui::Ui) {
        ui.heading("Games");
        ui.add_space(6.0);
        if ui.button("Add executable").clicked() {
            self.add_game();
        }
        ui.add_space(10.0);

        if self.state.games.is_empty() {
            ui.label(egui::RichText::new("No games yet. Add a game executable to detect its architecture and interface era.").color(MUTED));
        } else {
            egui::ScrollArea::vertical().max_height(150.0).show(ui, |ui| {
                for record in self.state.games.values() {
                    let label = format!(
                        "{}  ({}/{}, {})",
                        Path::new(&record.exe_path).file_name().map(|n| n.to_string_lossy().into_owned()).unwrap_or_else(|| record.exe_path.clone()),
                        record.arch,
                        record.era,
                        if record.configured { "configured" } else { "not configured" }
                    );
                    if ui.selectable_label(self.selected.as_deref() == Some(record.exe_path.as_str()), label).clicked() {
                        self.selected = Some(record.exe_path.clone());
                        self.preload = record.ld_preload.clone().unwrap_or_default();
                    }
                }
            });
        }

        ui.add_space(10.0);
        ui.separator();
        ui.add_space(6.0);

        let has_game = self.selected.is_some();
        let era_known = self.selected.as_ref()
            .and_then(|path| self.state.games.get(path))
            .map(|r| r.era == "legacy" || r.era == "modern")
            .unwrap_or(false);
        ui.horizontal_wrapped(|ui| {
            if ui.add_enabled(has_game, egui::Button::new("Detect again")).clicked() {
                let path = self.selected.clone().unwrap();
                let result = detect_game(path).map(|info| format!("{}/{} with {} interfaces", info.arch, info.era, info.interfaces.len()));
                self.result("Detect", result);
            }
            if ui.add_enabled(has_game, egui::Button::new("Unpack SteamStub")).clicked() {
                self.unpack();
            }
            if ui.add_enabled(has_game && era_known, egui::Button::new("Install DLL")).clicked() {
                self.install_dll();
            }
            if ui.add_enabled(has_game, egui::Button::new("Configure server")).clicked() {
                self.configure();
            }
            if ui.add_enabled(has_game, egui::Button::new("Launch game")).clicked() {
                self.launch();
            }
        });

        ui.add_space(10.0);
        ui.horizontal(|ui| {
            ui.label("LD_PRELOAD library (Linux launches)");
            ui.add(egui::TextEdit::singleline(&mut self.preload).desired_width(280.0));
            if ui.add_enabled(has_game, egui::Button::new("Set")).clicked() {
                self.set_preload();
            }
        });

        ui.add_space(10.0);
        if let Some(path) = &self.selected {
            if let Some(record) = self.state.games.get(path) {
                ui.label(egui::RichText::new("Selected game").strong());
                ui.label(format!("Path: {}", record.exe_path));
                ui.label(format!("Architecture: {}   Era: {}", record.arch, record.era));
                ui.label(format!("Interfaces: {}", if record.interfaces.is_empty() { "none scanned".into() } else { record.interfaces.join(", ") }));
                ui.label(format!("DLL: {}", record.dll_path.as_deref().unwrap_or("not installed")));
                ui.label(format!("Configured: {}", if record.configured { "yes" } else { "no" }));
            }
        } else {
            ui.label(egui::RichText::new("Select a game to see its details.").color(MUTED));
        }
    }

    fn ui_log(&mut self, ui: &mut egui::Ui) {
        ui.horizontal(|ui| {
            ui.label(egui::RichText::new("Activity").strong());
            if ui.button("Clear").clicked() {
                self.log.clear();
                self.error = None;
            }
        });
        ui.separator();
        let (level_color, error_text) = match &self.error {
            Some(text) => (ERROR, text.clone()),
            None => (OK, String::new()),
        };
        if !error_text.is_empty() {
            ui.label(egui::RichText::new(format!("[ERROR] {error_text}")).color(level_color));
        }
        egui::ScrollArea::vertical().stick_to_bottom(true).show(ui, |ui| {
            for line in &self.log {
                let color = match line.level {
                    "ERROR" => ERROR,
                    "OK" => OK,
                    _ => MUTED,
                };
                ui.label(egui::RichText::new(format!("[{}] {}", line.level, line.text)).color(color));
            }
        });
    }
}

impl eframe::App for LauncherApp {
    fn update(&mut self, ctx: &egui::Context, _frame: &mut eframe::Frame) {
        egui::TopBottomPanel::top("header").show(ctx, |ui| {
            ui.horizontal(|ui| {
                ui.heading(egui::RichText::new("SAPIE Launcher").color(EMBER).strong());
                ui.add_space(12.0);
                ui.selectable_value(&mut self.tab, Tab::Connect, "Connect");
                ui.selectable_value(&mut self.tab, Tab::Games, "Games");
            });
        });

        egui::TopBottomPanel::bottom("activity")
            .resizable(true)
            .default_height(180.0)
            .min_height(90.0)
            .show(ctx, |ui| self.ui_log(ui));

        egui::CentralPanel::default().show(ctx, |ui| {
            match self.tab {
                Tab::Connect => self.ui_connect(ui),
                Tab::Games => self.ui_games(ui),
            }
        });
    }
}

fn configure_style(ctx: &egui::Context) {
    let mut style = (*ctx.style()).clone();
    style.visuals = egui::Visuals::dark();
    style.visuals.window_fill = BG;
    style.visuals.panel_fill = BG;
    style.visuals.extreme_bg_color = FIELD;
    style.visuals.faint_bg_color = SURFACE;
    style.visuals.code_bg_color = FIELD;
    style.visuals.widgets.noninteractive.bg_fill = SURFACE;
    style.visuals.widgets.inactive.bg_fill = SURFACE;
    style.visuals.widgets.hovered.bg_fill = egui::Color32::from_rgb(0x2c, 0x39, 0x4f);
    style.visuals.widgets.active.bg_fill = egui::Color32::from_rgb(0x36, 0x44, 0x5c);
    style.visuals.widgets.active.weak_bg_fill = egui::Color32::from_rgb(0x36, 0x44, 0x5c);
    style.visuals.widgets.active.bg_stroke = egui::Stroke::new(2.0_f32, EMBER);
    let text_stroke = egui::Stroke::new(1.0_f32, TEXT);
    style.visuals.widgets.noninteractive.fg_stroke = egui::Stroke::new(1.0_f32, MUTED);
    style.visuals.widgets.inactive.fg_stroke = text_stroke;
    style.visuals.widgets.hovered.fg_stroke = text_stroke;
    style.visuals.widgets.active.fg_stroke = text_stroke;
    style.visuals.selection.bg_fill = egui::Color32::from_rgb(0x7a, 0x2f, 0x13);
    style.visuals.selection.stroke = egui::Stroke::new(2.0_f32, EMBER);
    style.visuals.hyperlink_color = EMBER;
    style.visuals.text_cursor.stroke = egui::Stroke::new(2.0_f32, EMBER);

    let body = egui::FontId::proportional(15.0);
    style.text_styles.insert(egui::TextStyle::Body, body.clone());
    style.text_styles.insert(egui::TextStyle::Button, body);
    style.text_styles.insert(egui::TextStyle::Heading, egui::FontId::proportional(22.0));
    style.text_styles.insert(egui::TextStyle::Small, egui::FontId::proportional(13.0));
    style.text_styles.insert(egui::TextStyle::Monospace, egui::FontId::monospace(13.0));
    ctx.set_style(style);
}

fn main() -> eframe::Result<()> {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_inner_size([1100.0, 720.0])
            .with_min_inner_size([860.0, 560.0]),
        ..Default::default()
    };
    eframe::run_native(
        "SAPIE Launcher",
        options,
        Box::new(|cc| {
            configure_style(&cc.egui_ctx);
            Ok(Box::new(LauncherApp::new()))
        }),
    )
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
    fn prefers_original_backup_over_installed_dll() {
        let dir = std::env::temp_dir().join(format!("sapie-detect-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        fs::write(dir.join("steam_api.dll"), b"ours").unwrap();
        let backup = dir.join("steam_api.dll.sapie-original.bak");
        fs::write(&backup, b"original").unwrap();
        assert_eq!(find_original_steam_api(&dir), Some(backup));
        fs::remove_dir_all(&dir).unwrap();
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
