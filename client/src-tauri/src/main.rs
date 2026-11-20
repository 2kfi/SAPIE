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
const SURFACE_DEEP: egui::Color32 = egui::Color32::from_rgb(0x15, 0x1e, 0x2c);
const EDGE: egui::Color32 = egui::Color32::from_rgb(0x35, 0x45, 0x5c);
const HIGHLIGHT: egui::Color32 = egui::Color32::from_rgb(0x55, 0x6a, 0x84);
const FIELD: egui::Color32 = egui::Color32::from_rgb(0x0b, 0x11, 0x1c);
const TEXT: egui::Color32 = egui::Color32::from_rgb(0xe9, 0xef, 0xf7);
const MUTED: egui::Color32 = egui::Color32::from_rgb(0xb3, 0xc3, 0xd6);
const EMBER: egui::Color32 = egui::Color32::from_rgb(0xff, 0x9b, 0x6a);
const OK: egui::Color32 = egui::Color32::from_rgb(0x8f, 0xe3, 0xb0);
const WARN: egui::Color32 = egui::Color32::from_rgb(0xff, 0xd1, 0x8a);
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

fn ensure_steam_appid(dir: &Path, exe_path: &str) -> Result<String, String> {
    let path = dir.join("steam_appid.txt");
    if path.is_file() {
        let existing = fs::read_to_string(&path).map_err(|e| e.to_string())?;
        let value = existing.trim().to_string();
        if !value.is_empty() {
            return Ok(format!("steam_appid.txt kept {value}"));
        }
    }
    let name = Path::new(exe_path)
        .file_name()
        .and_then(|n| n.to_str())
        .unwrap_or("")
        .to_ascii_lowercase();
    if name == "iw4mp.exe" || name == "iw4sp.exe" {
        fs::write(&path, "10190\n").map_err(|e| e.to_string())?;
        return Ok("steam_appid.txt set to 10190".into());
    }
    Ok("steam_appid.txt left untouched (not an MW2 executable)".into())
}

fn check_runtime_deps(dir: &Path, dll_name: &str) -> Option<String> {
    if dir.join("libwinpthread-1.dll").is_file() {
        None
    } else {
        Some(format!(
            "libwinpthread-1.dll is not beside {dll_name}. Static builds do not import it; if the game reports it missing, put the DLL there."
        ))
    }
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
) -> Result<String, String> {
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
    let appid_note = ensure_steam_appid(dir, &exe_path)?;

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
    save_state(&state)?;
    Ok(format!("sapie_server.txt written. {appid_note}"))
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

#[derive(Clone, Copy, PartialEq, Eq)]
enum Step {
    Connect,
    Select,
    Detect,
    Install,
    Configure,
    Launch,
}

const STEPS: [(Step, &str); 6] = [
    (Step::Connect, "Connect"),
    (Step::Select, "Select game"),
    (Step::Detect, "Detect"),
    (Step::Install, "Install"),
    (Step::Configure, "Configure"),
    (Step::Launch, "Launch"),
];

impl Step {
    fn index(self) -> usize {
        match self {
            Step::Connect => 0,
            Step::Select => 1,
            Step::Detect => 2,
            Step::Install => 3,
            Step::Configure => 4,
            Step::Launch => 5,
        }
    }
}

struct LogLine {
    level: &'static str,
    text: String,
}

struct LauncherApp {
    step: Step,
    state: PersistedState,
    selected: Option<String>,
    connection: Connection,
    remember: bool,
    password_visible: bool,
    preload: String,
    connect_ready: bool,
    appid_note: Option<String>,
    dep_warning: Option<String>,
    font_note: String,
    log: Vec<LogLine>,
    error: Option<String>,
}

impl LauncherApp {
    fn new(font_note: String) -> Self {
        let (initial_state, load_error) = match load_state() {
            Ok(state) => (state, None),
            Err(err) => (PersistedState::default(), Some(err)),
        };
        let connection = initial_state
            .server
            .clone()
            .unwrap_or_else(|| Connection { server: String::new(), username: String::new(), password: String::new() });
        let remember = initial_state.server.is_some();
        let selected = initial_state.games.keys().next().cloned();
        let preload = selected
            .as_deref()
            .and_then(|path| initial_state.games.get(path))
            .and_then(|record| record.ld_preload.clone())
            .unwrap_or_default();
        let mut app = Self {
            step: Step::Connect,
            state: PersistedState::default(),
            selected,
            connection,
            remember,
            password_visible: false,
            preload,
            connect_ready: false,
            appid_note: None,
            dep_warning: None,
            font_note,
            log: Vec::new(),
            error: None,
        };
        app.log("INFO", "Start with the server details.".into());
        if let Some(err) = load_error {
            app.log("ERROR", format!("Could not load saved launcher state: {err}"));
        }
        app.state = initial_state;
        app
    }

    fn log(&mut self, level: &'static str, text: String) {
        self.log.push(LogLine { level, text });
    }

    fn record(&self) -> Option<&GameRecord> {
        self.selected
            .as_deref()
            .and_then(|path| self.state.games.get(path))
    }

    fn step_done(&self, step: Step) -> bool {
        match step {
            Step::Connect => self.connect_ready,
            Step::Select => self.record().is_some(),
            Step::Detect => self.record().map_or(false, |record| {
                (record.era == "legacy" || record.era == "modern") && !record.interfaces.is_empty()
            }),
            Step::Install => self.record().map_or(false, |record| record.dll_path.is_some()),
            Step::Configure => self.record().map_or(false, |record| record.configured),
            Step::Launch => false,
        }
    }

    fn can_open(&self, step: Step) -> bool {
        (0..step.index()).all(|index| self.step_done(STEPS[index].0))
    }

    fn advance(&mut self) {
        if let Some((next, _)) = STEPS.get(self.step.index() + 1) {
            self.step = *next;
        }
    }

    fn result(&mut self, action: &str, result: Result<String, String>) -> bool {
        let succeeded = result.is_ok();
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
        succeeded
    }

    fn refresh(&mut self) {
        match load_state() {
            Ok(state) => self.state = state,
            Err(err) => self.log("ERROR", format!("Could not reload launcher state: {err}")),
        }
    }

    fn connect(&mut self) {
        let conn = self.connection.clone();
        let validated = validate_server(&conn.server)
            .and_then(|_| if conn.username.trim().is_empty() { Err("Username is required.".into()) } else { Ok(()) })
            .and_then(|_| if conn.password.is_empty() { Err("Password is required.".into()) } else { Ok(()) });
        if let Err(err) = validated {
            self.result("Connect details", Err(err));
            return;
        }
        let outcome = if self.remember {
            let mut state = match load_state() {
                Ok(state) => state,
                Err(err) => {
                    self.result("Connect details", Err(err));
                    return;
                }
            };
            state.server = Some(conn);
            save_state(&state).map(|_| "Connection details saved on this device.".into())
        } else {
            Ok("Connection details ready for this session.".into())
        };
        if self.result("Connect details", outcome) {
            self.connect_ready = true;
            self.advance();
        }
    }

    fn select_game(&mut self) {
        let Some(path) = pick_executable() else {
            self.log("INFO", "No executable selected.".into());
            return;
        };
        if !Path::new(&path).is_file() {
            self.result("Select game", Err(format!("Executable not found: {path}")));
            return;
        }
        let mut state = match load_state() {
            Ok(state) => state,
            Err(err) => {
                self.result("Select game", Err(err));
                return;
            }
        };
        let name = Path::new(&path)
            .file_name()
            .map(|value| value.to_string_lossy().into_owned())
            .unwrap_or_else(|| path.clone());
        state.games.entry(path.clone()).or_insert_with(|| new_record(&path));
        let outcome = save_state(&state).map(|_| format!("{name} selected."));
        if self.result("Select game", outcome) {
            self.selected = Some(path);
            self.preload.clear();
            self.advance();
        }
    }

    fn detect(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        let result = detect_game(path).map(|info| {
            format!(
                "{} detected as {}/{} with {} interfaces",
                self.selected
                    .as_deref()
                    .and_then(|p| Path::new(p).file_name())
                    .map(|n| n.to_string_lossy().into_owned())
                    .unwrap_or_else(|| "executable".into()),
                info.arch,
                info.era,
                info.interfaces.len()
            )
        });
        if self.result("Detect", result) {
            self.advance();
        }
    }

    fn unpack(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        match unpack_steamstub(path) {
            Ok(Some(unpacked)) => {
                self.selected = Some(unpacked);
                let result = detect_game(self.selected.clone().unwrap()).map(|info| {
                    format!("{} detected as {}/{}", info.arch, info.era, info.interfaces.join(", "))
                });
                self.result("Detect unpacked executable", result);
            }
            Ok(None) => {
                self.result("Steamless", Ok("No SteamStub found in this executable.".into()));
            }
            Err(err) => {
                self.result("Steamless", Err(err));
            }
        }
    }

    fn install(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        let era = self.record().map(|record| record.era.clone()).unwrap_or_default();
        let outcome = copy_dll(path, era).map(|destination| {
            let dir = Path::new(&destination).parent().unwrap_or_else(|| Path::new("."));
            let name = Path::new(&destination)
                .file_name()
                .map(|value| value.to_string_lossy().into_owned())
                .unwrap_or_else(|| "SAPIE DLL".into());
            self.dep_warning = check_runtime_deps(dir, &name);
            format!("{name} installed beside the game.")
        });
        if self.result("Install DLL", outcome) {
            if let Some(warning) = &self.dep_warning {
                self.log("WARN", warning.clone());
            }
            self.advance();
        }
    }

    fn configure(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        let conn = self.connection.clone();
        let outcome = configure_server(path, conn.server, conn.username, conn.password);
        if let Ok(message) = &outcome {
            self.appid_note = Some(message.clone());
        }
        if self.result("Configure game", outcome) {
            self.advance();
        }
    }

    fn set_preload(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        self.result("Set LD_PRELOAD", set_ld_preload(path, self.preload.clone()).map(|_| "Library recorded for Linux launches.".into()));
    }

    fn launch(&mut self) {
        let Some(path) = self.selected.clone() else { return };
        self.result("Launch", launch_game(path).map(|_| "Game process started.".into()));
    }

    fn ui_current(&mut self, ui: &mut egui::Ui) {
        ui.label(
            egui::RichText::new(format!("{} / {}", self.step.index() + 1, STEPS.len()))
                .small()
                .color(MUTED),
        );
        ui.heading(STEPS[self.step.index()].1);
        ui.add_space(4.0);
        match self.step {
            Step::Connect => self.ui_connect(ui),
            Step::Select => self.ui_select(ui),
            Step::Detect => self.ui_detect(ui),
            Step::Install => self.ui_install(ui),
            Step::Configure => self.ui_configure(ui),
            Step::Launch => self.ui_launch(ui),
        }
    }

    fn ui_connect(&mut self, ui: &mut egui::Ui) {
        ui.label(egui::RichText::new("Details are passed to the game as SAPIE_USERNAME and SAPIE_PASSWORD. No server handshake happens here.").color(MUTED));
        ui.add_space(8.0);

        egui::Grid::new("connect-grid")
            .num_columns(2)
            .spacing([12.0, 6.0])
            .show(ui, |ui| {
                ui.label("Server (host:port)");
                ui.add(egui::TextEdit::singleline(&mut self.connection.server).desired_width(300.0).hint_text("127.0.0.1:7777"));
                ui.end_row();

                ui.label("Username");
                ui.add(egui::TextEdit::singleline(&mut self.connection.username).desired_width(200.0));
                ui.end_row();

                ui.label("Password");
                ui.horizontal(|ui| {
                    ui.add(egui::TextEdit::singleline(&mut self.connection.password).password(!self.password_visible).desired_width(200.0));
                    if ui.button(if self.password_visible { "Hide" } else { "Show" }).clicked() {
                        self.password_visible = !self.password_visible;
                    }
                });
                ui.end_row();
            });

        ui.add_space(6.0);
        ui.checkbox(&mut self.remember, "Remember on this device");
        ui.add_space(8.0);
        let label = if self.remember {
            "Save connection details"
        } else {
            "Use details for this session"
        };
        if primary(ui, label) {
            self.connect();
        }
    }

    fn ui_select(&mut self, ui: &mut egui::Ui) {
        ui.label(egui::RichText::new("Picking a file only records its path. Nothing is written yet.").color(MUTED));
        ui.add_space(8.0);
        if primary(ui, "Choose executable") {
            self.select_game();
        }
        ui.add_space(8.0);
        if self.state.games.is_empty() {
            ui.label(egui::RichText::new("No games recorded yet.").color(MUTED));
            return;
        }
        ui.label(egui::RichText::new("Known games").strong());
        let paths: Vec<String> = self.state.games.keys().cloned().collect();
        egui::ScrollArea::vertical().max_height(120.0).show(ui, |ui| {
            for path in paths {
                let name = Path::new(&path)
                    .file_name()
                    .map(|value| value.to_string_lossy().into_owned())
                    .unwrap_or_else(|| path.clone());
                if ui.selectable_label(self.selected.as_deref() == Some(path.as_str()), name).clicked() {
                    self.preload = self
                        .state
                        .games
                        .get(&path)
                        .and_then(|record| record.ld_preload.clone())
                        .unwrap_or_default();
                    self.selected = Some(path);
                    self.advance();
                }
            }
        });
    }

    fn ui_detect(&mut self, ui: &mut egui::Ui) {
        ui.label(egui::RichText::new("Reads the original steam_api DLL beside the executable and writes steam_interfaces.txt.").color(MUTED));
        if let Some(record) = self.record() {
            ui.add_space(4.0);
            ui.label(egui::RichText::new(record.exe_path.clone()).monospace().small());
        } else {
            ui.add_space(4.0);
            ui.label(egui::RichText::new("Select a game first.").color(WARN));
        }
        ui.add_space(8.0);
        if primary(ui, "Detect game") {
            self.detect();
        }
        if self.selected.is_some() {
            ui.add_space(4.0);
            if ui.button("Unpack SteamStub (optional)").clicked() {
                self.unpack();
            }
        }
    }

    fn ui_install(&mut self, ui: &mut egui::Ui) {
        let era = self.record().map(|record| record.era.clone()).unwrap_or_else(|| "unknown".into());
        ui.label(egui::RichText::new("Copies the DLL for the detected architecture and era, backing up the original.").color(MUTED));
        ui.add_space(4.0);
        ui.label(format!("Detected era: {era}"));
        ui.add_space(8.0);
        if primary(ui, "Install SAPIE DLL") {
            self.install();
        }
        if let Some(warning) = &self.dep_warning {
            ui.add_space(6.0);
            ui.label(egui::RichText::new(warning).color(WARN));
        }
    }

    fn ui_configure(&mut self, ui: &mut egui::Ui) {
        ui.label(egui::RichText::new("Writes sapie_server.txt and steam_appid.txt beside the executable.").color(MUTED));
        ui.add_space(4.0);
        ui.label(egui::RichText::new(format!("Server: {}", self.connection.server.trim())).monospace().small());
        ui.add_space(8.0);
        if primary(ui, "Write game config") {
            self.configure();
        }
        if let Some(note) = &self.appid_note {
            ui.add_space(6.0);
            ui.label(egui::RichText::new(note).small().color(MUTED));
        }
    }

    fn ui_launch(&mut self, ui: &mut egui::Ui) {
        ui.label(egui::RichText::new("Starts the game with the saved credentials. Success means the process spawned.").color(MUTED));
        if let Some(record) = self.record() {
            ui.add_space(4.0);
            ui.label(egui::RichText::new(record.exe_path.clone()).monospace().small());
            ui.label(format!("Configured: {}", if record.configured { "yes" } else { "no" }));
        }
        ui.add_space(8.0);
        if primary(ui, "Launch game") {
            self.launch();
        }
        if cfg!(not(target_os = "windows")) {
            ui.add_space(8.0);
            ui.horizontal(|ui| {
                ui.label("LD_PRELOAD");
                ui.add(egui::TextEdit::singleline(&mut self.preload).desired_width(220.0));
                if ui.button("Set").clicked() {
                    self.set_preload();
                }
            });
        }
    }

    fn ui_state(&mut self, ui: &mut egui::Ui) {
        ui.label(egui::RichText::new("Current state").strong());
        ui.separator();
        let field = |ui: &mut egui::Ui, label: &str, value: String| {
            ui.horizontal(|ui| {
                ui.label(egui::RichText::new(label).small().color(MUTED));
                ui.label(egui::RichText::new(value).monospace().small());
            });
        };
        field(ui, "Connect", if self.connect_ready { "ready".into() } else { "pending".into() });
        let name = self
            .selected
            .as_deref()
            .and_then(|path| Path::new(path).file_name())
            .map(|value| value.to_string_lossy().into_owned())
            .unwrap_or_else(|| "none".into());
        field(ui, "Game", name);
        let record = self.record();
        field(ui, "Arch / era", format!("{} / {}", record.map_or("unknown", |r| r.arch.as_str()), record.map_or("unknown", |r| r.era.as_str())));
        field(ui, "Interfaces", record.map_or("0".into(), |r| r.interfaces.len().to_string()));
        field(ui, "DLL", record.and_then(|r| r.dll_path.as_deref()).unwrap_or("not installed").into());
        field(ui, "Config", if record.map_or(false, |r| r.configured) { "written" } else { "pending" }.into());
        ui.add_space(8.0);
        ui.label(egui::RichText::new(&self.font_note).small().color(MUTED));
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
        if let Some(text) = &self.error {
            ui.label(egui::RichText::new(format!("[ERROR] {text}")).color(ERROR));
        }
        egui::ScrollArea::vertical().stick_to_bottom(true).show(ui, |ui| {
            for line in &self.log {
                let color = match line.level {
                    "ERROR" => ERROR,
                    "OK" => OK,
                    "WARN" => WARN,
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
            ui.spacing_mut().item_spacing = egui::vec2(6.0, 2.0);
            ui.horizontal(|ui| {
                ui.label(egui::RichText::new("SAPIE").color(EMBER).strong());
                ui.label(egui::RichText::new("Launcher").strong());
            });
            ui.add_space(4.0);
            ui.horizontal(|ui| {
                for (index, (step, label)) in STEPS.iter().enumerate() {
                    let status = if self.step_done(*step) {
                        "done"
                    } else if self.step == *step {
                        "now"
                    } else {
                        "wait"
                    };
                    let text = format!("{}. {}\n{}", index + 1, label, status);
                    if ui
                        .add_enabled(self.can_open(*step), egui::Button::new(egui::RichText::new(text).small()))
                        .clicked()
                    {
                        self.step = *step;
                    }
                }
            });
        });

        egui::TopBottomPanel::bottom("activity")
            .resizable(true)
            .default_height(140.0)
            .min_height(80.0)
            .show(ctx, |ui| self.ui_log(ui));

        egui::SidePanel::right("state")
            .resizable(false)
            .default_width(230.0)
            .show(ctx, |ui| surface(ui, |ui| self.ui_state(ui)));

        egui::CentralPanel::default().show(ctx, |ui| {
            ui.spacing_mut().item_spacing = egui::vec2(8.0, 5.0);
            surface(ui, |ui| self.ui_current(ui));
        });
    }
}

fn surface(ui: &mut egui::Ui, content: impl FnOnce(&mut egui::Ui)) {
    // egui has no backdrop blur; layered fills and a top highlight approximate glass.
    egui::Frame {
        fill: SURFACE_DEEP,
        stroke: egui::Stroke::new(1.0_f32, EDGE),
        rounding: egui::Rounding::same(8.0),
        inner_margin: egui::Margin::symmetric(12.0, 10.0),
        outer_margin: egui::Margin::symmetric(0.0, 6.0),
        ..Default::default()
    }
    .show(ui, |ui| {
        let rect = ui.available_rect_before_wrap();
        let highlight = egui::Rect::from_x_y_ranges(rect.x_range(), rect.top()..=rect.top() + 1.0);
        ui.painter().rect_filled(highlight, 0.0, HIGHLIGHT);
        ui.add_space(5.0);
        content(ui);
    });
}

fn primary(ui: &mut egui::Ui, label: &str) -> bool {
    ui.add(
        egui::Button::new(egui::RichText::new(label).color(BG).strong())
            .fill(EMBER)
            .min_size(egui::vec2(170.0, 30.0)),
    )
    .clicked()
}

fn font_bytes(family: &str) -> Option<Vec<u8>> {
    let mut candidate = None;
    if cfg!(windows) {
        if let Ok(root) = std::env::var("WINDIR") {
            let dir = PathBuf::from(root).join("Fonts");
            for name in [format!("{family}-Regular.ttf"), format!("{family} Regular.ttf")] {
                let path = dir.join(name);
                if path.is_file() {
                    candidate = Some(path);
                    break;
                }
            }
        }
    } else if cfg!(target_os = "macos") {
        let mut dirs = vec![PathBuf::from("/Library/Fonts")];
        if let Some(home) = dirs::home_dir() {
            dirs.push(home.join("Library/Fonts"));
        }
        for dir in dirs {
            let path = dir.join(format!("{family}-Regular.ttf"));
            if path.is_file() {
                candidate = Some(path);
                break;
            }
        }
    } else if let Ok(output) = Command::new("fc-match").args(["-f", "%{file}|%{family}", family]).output() {
        if output.status.success() {
            let text = String::from_utf8_lossy(&output.stdout);
            if let Some((file, matched)) = text.trim().split_once('|') {
                if matched.to_lowercase().contains(&family.to_lowercase()) && Path::new(file).is_file() {
                    candidate = Some(PathBuf::from(file));
                }
            }
        }
    }
    candidate.and_then(|path| fs::read(&path).ok())
}

fn configure_style(ctx: &egui::Context) -> String {
    let mut fonts = egui::FontDefinitions::default();
    let mut loaded = Vec::new();
    if let Some(bytes) = font_bytes("Manrope") {
        fonts.font_data.insert("Manrope".into(), egui::FontData::from_owned(bytes));
        if let Some(family) = fonts.families.get_mut(&egui::FontFamily::Proportional) {
            family.insert(0, "Manrope".into());
        }
        loaded.push("Manrope");
    }
    if let Some(bytes) = font_bytes("IBM Plex Mono") {
        fonts.font_data.insert("IBM Plex Mono".into(), egui::FontData::from_owned(bytes));
        if let Some(family) = fonts.families.get_mut(&egui::FontFamily::Monospace) {
            family.insert(0, "IBM Plex Mono".into());
        }
        loaded.push("IBM Plex Mono");
    }

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
    style.visuals.widgets.noninteractive.rounding = egui::Rounding::same(4.0);
    style.visuals.widgets.inactive.rounding = egui::Rounding::same(4.0);
    let text_stroke = egui::Stroke::new(1.0_f32, TEXT);
    style.visuals.widgets.noninteractive.fg_stroke = egui::Stroke::new(1.0_f32, MUTED);
    style.visuals.widgets.inactive.fg_stroke = text_stroke;
    style.visuals.widgets.hovered.fg_stroke = text_stroke;
    style.visuals.widgets.active.fg_stroke = text_stroke;
    style.visuals.selection.bg_fill = egui::Color32::from_rgb(0x7a, 0x2f, 0x13);
    style.visuals.selection.stroke = egui::Stroke::new(2.0_f32, EMBER);
    style.visuals.hyperlink_color = EMBER;
    style.visuals.text_cursor.stroke = egui::Stroke::new(2.0_f32, EMBER);

    let body = egui::FontId::proportional(14.0);
    style.text_styles.insert(egui::TextStyle::Body, body.clone());
    style.text_styles.insert(egui::TextStyle::Button, body);
    style.text_styles.insert(egui::TextStyle::Heading, egui::FontId::proportional(19.0));
    style.text_styles.insert(egui::TextStyle::Small, egui::FontId::proportional(12.0));
    style.text_styles.insert(egui::TextStyle::Monospace, egui::FontId::monospace(12.0));

    ctx.set_fonts(fonts);
    ctx.set_style(style);
    if loaded.is_empty() {
        "Fonts: built-in fallback; install Manrope and IBM Plex Mono to use them.".into()
    } else {
        format!("Fonts: {}", loaded.join(" + "))
    }
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
            let font_note = configure_style(&cc.egui_ctx);
            Ok(Box::new(LauncherApp::new(font_note)))
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

    #[test]
    fn steam_appid_is_scoped_to_mw2_and_user_editable() {
        let dir = std::env::temp_dir().join(format!("sapie-appid-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        assert!(ensure_steam_appid(&dir, "/games/iw4mp.exe").unwrap().contains("10190"));
        assert_eq!(fs::read_to_string(dir.join("steam_appid.txt")).unwrap(), "10190\n");
        fs::write(dir.join("steam_appid.txt"), "999\n").unwrap();
        assert!(ensure_steam_appid(&dir, "/games/iw4sp.exe").unwrap().contains("999"));
        assert_eq!(fs::read_to_string(dir.join("steam_appid.txt")).unwrap(), "999\n");

        let other = std::env::temp_dir().join(format!("sapie-appid-other-{}", std::process::id()));
        let _ = fs::remove_dir_all(&other);
        fs::create_dir_all(&other).unwrap();
        ensure_steam_appid(&other, "/games/other.exe").unwrap();
        assert!(!other.join("steam_appid.txt").exists());
        fs::remove_dir_all(&dir).unwrap();
        fs::remove_dir_all(&other).unwrap();
    }

    #[test]
    fn runtime_dep_check_reports_missing_winpthread() {
        let dir = std::env::temp_dir().join(format!("sapie-deps-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        let warning = check_runtime_deps(&dir, "steam_api.dll").unwrap();
        assert!(warning.contains("libwinpthread-1.dll"));
        fs::write(dir.join("libwinpthread-1.dll"), b"").unwrap();
        assert!(check_runtime_deps(&dir, "steam_api.dll").is_none());
        fs::remove_dir_all(&dir).unwrap();
    }
}
