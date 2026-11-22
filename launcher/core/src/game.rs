use crate::detection::{
    check_runtime_deps, detect_arch, ensure_steam_appid, era_from_interfaces,
    find_original_steam_api, scan_interfaces,
};
use crate::error::{CommandError, ErrorKind, Result};
use crate::platform::Platform;
use crate::state::GameRecord;
use serde::Serialize;
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::Command;

const GAME_PROTOCOL_PORT: u16 = 47584;
const BACKUP_SUFFIX: &str = ".sapie-original.bak";
const DLL_NAMES: [&str; 3] = ["steam_api.dll", "steam_api64.dll", "libsteam_api.so"];
const TOKEN_FILE_NAME: &str = "sapie_client.json";

#[derive(Serialize)]
struct TokenFile<'a> {
    session_token: &'a str,
}

type Progress<'a> = Option<&'a dyn Fn(&str, &str)>;

pub fn report(progress: Progress<'_>, step: &str, message: &str) {
    if let Some(report) = progress {
        report(step, message);
    }
}

#[derive(Clone, Debug, Serialize)]
pub struct DetectResult {
    pub exe_path: String,
    pub arch: String,
    pub era: String,
    pub interfaces: Vec<String>,
    pub original_library: Option<String>,
    pub warning: Option<String>,
}

#[derive(Clone, Debug, Serialize)]
pub struct InstallResult {
    pub arch: String,
    pub era: String,
    pub interfaces: Vec<String>,
    pub original_library: Option<String>,
    pub library: String,
    pub warning: Option<String>,
}

#[derive(Clone, Debug, Serialize)]
pub struct ConfigureResult {
    pub appid: Option<u32>,
    pub game_server: String,
    pub note: String,
}

fn scoped(dir: &Path, name: &str) -> Result<PathBuf> {
    let candidate = Path::new(name);
    if candidate.file_name() != Some(candidate.as_os_str())
        || name.contains('\\')
        || name.contains('/')
    {
        return Err(CommandError::new(
            ErrorKind::InvalidPath,
            "Refusing to write outside the game directory.",
        ));
    }
    let path = dir.join(candidate);
    if !path.starts_with(dir) {
        return Err(CommandError::new(
            ErrorKind::InvalidPath,
            "Refusing to write outside the game directory.",
        ));
    }
    Ok(path)
}

pub fn canonical_game(path: &str) -> Result<(PathBuf, PathBuf)> {
    if path.trim().is_empty() {
        return Err(CommandError::new(
            ErrorKind::InvalidPath,
            "Game path is empty.",
        ));
    }
    let exe = Path::new(path).canonicalize().map_err(|e| match e.kind() {
        std::io::ErrorKind::NotFound => {
            CommandError::path(ErrorKind::NotFound, Path::new(path), "Executable not found")
        }
        std::io::ErrorKind::PermissionDenied => CommandError::path(
            ErrorKind::PermissionDenied,
            Path::new(path),
            "Permission denied",
        ),
        _ => e.into(),
    })?;
    if !exe.is_file() {
        return Err(CommandError::path(
            ErrorKind::InvalidPath,
            &exe,
            "This is not a file",
        ));
    }
    let parent = exe.parent().ok_or_else(|| {
        CommandError::path(
            ErrorKind::InvalidPath,
            &exe,
            "Executable has no parent directory",
        )
    })?;
    Ok((exe.clone(), parent.to_path_buf()))
}

fn dll_name(arch: &str) -> Result<&'static str> {
    match (std::env::consts::OS, arch) {
        ("windows", "x86") => Ok("steam_api.dll"),
        ("windows", "x64") => Ok("steam_api64.dll"),
        ("linux", "x86" | "x64" | "arm64") => Ok("libsteam_api.so"),
        (os, arch) => Err(CommandError::new(
            ErrorKind::NotFound,
            format!("No SAPIE library for {os}/{arch}."),
        )),
    }
}

fn dll_search_dirs(arch: &str, era: &str) -> Vec<PathBuf> {
    let target = format!("{}-{era}", std::env::consts::OS);
    let suffix = format!("{target}-{arch}");
    let mut dirs = Vec::new();
    if let Ok(exe) = std::env::current_exe() {
        if let Some(root) = exe.parent() {
            dirs.push(root.join("steam_api").join(&suffix));
            dirs.push(root.join("resources/steam_api").join(&suffix));
        }
    }
    let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    if let Some(repo) = manifest.parent().and_then(Path::parent) {
        dirs.push(repo.join("out").join(&suffix));
    }
    dirs
}

pub fn find_dll_source(arch: &str, era: &str) -> Result<PathBuf> {
    if era != "legacy" && era != "modern" {
        return Err(CommandError::new(
            ErrorKind::NotFound,
            "Game era could not be determined; refusing to guess a library.",
        ));
    }
    let name = dll_name(arch)?;
    let dirs = dll_search_dirs(arch, era);
    let source = dirs
        .iter()
        .map(|dir| dir.join(name))
        .find(|path| path.is_file())
        .ok_or_else(|| {
            CommandError::new(
                ErrorKind::NotFound,
                format!(
                    "SAPIE library not found for {arch}/{era}. Build the client matrix and place {name} in one of: {}",
                    dirs.iter()
                        .map(|dir| dir.display().to_string())
                        .collect::<Vec<_>>()
                        .join(", ")
                ),
            )
        })?;
    Ok(source)
}

pub fn detect(path: &str, progress: Progress<'_>) -> Result<DetectResult> {
    let (exe, dir) = canonical_game(path)?;
    report(
        progress,
        "detect",
        "Reading the executable and Steam interfaces.",
    );
    let arch = detect_arch(&fs::read(&exe)?);
    let original = find_original_steam_api(&dir);
    let interfaces = match &original {
        Some(path) => scan_interfaces(&fs::read(path)?),
        None => Vec::new(),
    };
    if interfaces.is_empty() {
        return Err(CommandError::new(
            ErrorKind::NotFound,
            "No Steam interface versions found next to this executable. Keep the original library beside it and try again.",
        ));
    }
    let interfaces_file = scoped(&dir, "steam_interfaces.txt")?;
    fs::write(&interfaces_file, format!("{}\n", interfaces.join("\n")))?;
    let era = era_from_interfaces(&interfaces);
    let warning = dll_name(&arch)
        .ok()
        .and_then(|name| check_runtime_deps(&dir, name));
    Ok(DetectResult {
        exe_path: exe.to_string_lossy().into_owned(),
        arch,
        era,
        interfaces,
        original_library: original.map(|path| path.to_string_lossy().into_owned()),
        warning,
    })
}

fn original_backup(dir: &Path, name: &str) -> Result<PathBuf> {
    scoped(dir, &format!("{name}{BACKUP_SUFFIX}"))
}

pub fn install(
    path: &str,
    source: Option<&Path>,
    record: Option<&GameRecord>,
    progress: Progress<'_>,
) -> Result<InstallResult> {
    let (_exe, dir) = canonical_game(path)?;
    let detected = detect(path, progress)?;
    let source = match source {
        Some(path) => path.to_path_buf(),
        None => find_dll_source(&detected.arch, &detected.era)?,
    };
    if !source.is_file() {
        return Err(CommandError::path(
            ErrorKind::NotFound,
            &source,
            "SAPIE library not found",
        ));
    }
    let name = dll_name(&detected.arch)?;
    let destination = scoped(&dir, name)?;
    let temp = scoped(&dir, &format!(".{name}.sapie-tmp"))?;

    report(progress, "backup", "Checking the original library.");
    let mut moved_backup = None;
    let mut backup_path = None;
    for candidate in DLL_NAMES {
        let original = scoped(&dir, candidate)?;
        if !original.is_file() {
            continue;
        }
        let backup = original_backup(&dir, candidate)?;
        if backup.exists() {
            let ours = record
                .and_then(|record| record.original_library.as_deref())
                .and_then(|saved| Path::new(saved).file_name())
                .is_some_and(|saved| saved == backup.file_name().unwrap_or_default());
            if !ours {
                return Err(CommandError::path(
                    ErrorKind::BackupConflict,
                    &backup,
                    "A backup is already here and was not made by this launcher",
                ));
            }
        } else {
            fs::rename(&original, &backup)?;
            moved_backup = Some((backup.clone(), original.clone()));
        }
        backup_path = Some(backup);
    }

    report(progress, "install", "Installing the SAPIE library.");
    let result = (|| -> Result<()> {
        if destination.exists() {
            fs::remove_file(&destination)?;
        }
        fs::copy(source, &temp)?;
        fs::rename(&temp, &destination)?;
        Ok(())
    })();
    if let Err(error) = result {
        let _ = fs::remove_file(&temp);
        if let Some((backup, original)) = moved_backup {
            let _ = fs::rename(backup, original);
        }
        return Err(error);
    }

    Ok(InstallResult {
        arch: detected.arch,
        era: detected.era,
        interfaces: detected.interfaces,
        original_library: backup_path.map(|path| path.to_string_lossy().into_owned()),
        library: destination.to_string_lossy().into_owned(),
        warning: detected.warning,
    })
}

pub fn restore(
    path: &str,
    record: Option<&GameRecord>,
    progress: Progress<'_>,
) -> Result<Option<String>> {
    let (_exe, dir) = canonical_game(path)?;
    report(progress, "restore", "Looking for the original library.");
    let backup = match record
        .and_then(|record| record.original_library.as_deref())
        .map(PathBuf::from)
    {
        Some(path) if path.is_file() && path.starts_with(&dir) => path,
        _ => {
            let found = DLL_NAMES
                .iter()
                .map(|name| original_backup(&dir, name))
                .collect::<Result<Vec<_>>>()?
                .into_iter()
                .find(|path| path.is_file());
            found.ok_or_else(|| {
                CommandError::new(
                    ErrorKind::NotFound,
                    "No original library backup was found next to this game.",
                )
            })?
        }
    };
    let file_name = backup
        .file_name()
        .and_then(|name| name.to_str())
        .unwrap_or("");
    let original_name = file_name.strip_suffix(BACKUP_SUFFIX).ok_or_else(|| {
        CommandError::path(ErrorKind::InvalidPath, &backup, "Unexpected backup name")
    })?;
    let destination = scoped(&dir, original_name)?;
    if destination.exists() {
        let ours = record
            .and_then(|record| record.library.as_deref())
            .map(Path::new)
            .is_some_and(|installed| installed == destination);
        if !ours {
            return Err(CommandError::path(
                ErrorKind::BackupConflict,
                &destination,
                "An unrelated library is already here; move it before restoring",
            ));
        }
        fs::remove_file(&destination)?;
    }
    report(progress, "restore", "Putting the original library back.");
    fs::rename(&backup, &destination)?;
    if let Some(installed) = record.and_then(|record| record.library.as_deref()) {
        let installed = Path::new(installed);
        if installed != destination && installed.starts_with(&dir) && installed.is_file() {
            let _ = fs::remove_file(installed);
        }
    }
    Ok(Some(destination.to_string_lossy().into_owned()))
}

pub fn remove_library(path: &str, record: &GameRecord) -> Result<()> {
    let (_exe, dir) = canonical_game(path)?;
    if let Some(installed) = record.library.as_deref() {
        let installed = Path::new(installed);
        if installed.starts_with(&dir) && installed.is_file() {
            fs::remove_file(installed)?;
        }
    }
    Ok(())
}

pub fn game_server_from_api(server: &str) -> Result<String> {
    let (host, _) = server.rsplit_once(':').ok_or_else(|| {
        CommandError::new(
            ErrorKind::InvalidServer,
            "No saved server address to write.",
        )
    })?;
    let host = host.trim_matches(['[', ']']);
    if host.is_empty() {
        return Err(CommandError::new(
            ErrorKind::InvalidServer,
            "No saved server host to write.",
        ));
    }
    Ok(format!("{host}:{GAME_PROTOCOL_PORT}"))
}

pub fn configure(
    path: &str,
    appid: u32,
    server: &str,
    progress: Progress<'_>,
) -> Result<ConfigureResult> {
    let (_exe, dir) = canonical_game(path)?;
    let game_server = game_server_from_api(server)?;
    report(progress, "configure", "Writing the connection config.");
    let server_file = scoped(&dir, "sapie_server.txt")?;
    fs::write(&server_file, format!("{game_server}\n"))?;

    let (note, saved_appid) = if appid == 0 {
        let note =
            ensure_steam_appid(&dir, path).map_err(|e| CommandError::new(ErrorKind::Io, e))?;
        let saved = fs::read_to_string(scoped(&dir, "steam_appid.txt")?)
            .ok()
            .and_then(|text| text.trim().parse::<u32>().ok())
            .filter(|value| *value != 0);
        (note, saved)
    } else {
        let file = scoped(&dir, "steam_appid.txt")?;
        fs::write(&file, format!("{appid}\n"))?;
        (format!("steam_appid.txt set to {appid}"), Some(appid))
    };
    Ok(ConfigureResult {
        appid: saved_appid,
        game_server,
        note,
    })
}

pub fn unpack_steamstub(path: &str, progress: Progress<'_>) -> Result<Option<String>> {
    if cfg!(not(target_os = "windows")) {
        return Err(CommandError::new(
            ErrorKind::Process,
            "SteamStub unpacking is available on Windows only.",
        ));
    }
    let (exe, _dir) = canonical_game(path)?;
    report(progress, "unpack", "Running the Steamless sidecar.");
    let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let cli = manifest.join("resources/steamless/Steamless.CLI.exe");
    if !cli.is_file() {
        return Err(CommandError::path(
            ErrorKind::NotFound,
            &cli,
            "Steamless CLI not found",
        ));
    }
    let output = Command::new(&cli)
        .current_dir(cli.parent().unwrap_or(&manifest))
        .arg(&exe)
        .output()?;
    let text = format!(
        "{}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    if text.contains("Successfully unpacked file!") {
        let stem = exe
            .file_stem()
            .and_then(|name| name.to_str())
            .ok_or_else(|| {
                CommandError::new(ErrorKind::InvalidPath, "Could not read the game name.")
            })?;
        let unpacked = exe.with_file_name(format!("{stem}.unpacked.exe"));
        if unpacked.is_file() {
            return Ok(Some(unpacked.to_string_lossy().into_owned()));
        }
        return Err(CommandError::new(
            ErrorKind::NotFound,
            "Steamless reported success but produced no unpacked executable.",
        ));
    }
    if text.contains("Failed to unpack file.")
        || text.contains("All unpackers failed")
        || text.contains("Invalid input file given")
    {
        return Ok(None);
    }
    Err(CommandError::new(
        ErrorKind::Process,
        format!(
            "Steamless gave no clear result (exit {:?}): {}",
            output.status.code(),
            text.trim()
        ),
    ))
}

fn write_launch_token(dir: &Path, token: &str) -> Result<PathBuf> {
    let settings = scoped(dir, "steam_settings")?;
    fs::create_dir_all(&settings)?;
    let path = scoped(&settings, TOKEN_FILE_NAME)?;
    if path.exists() {
        fs::remove_file(&path)?;
    }
    let content = serde_json::to_string(&TokenFile {
        session_token: token,
    })?;
    #[cfg(unix)]
    let file = {
        use std::os::unix::fs::OpenOptionsExt;
        OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&path)?
    };
    #[cfg(not(unix))]
    let file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&path)?;
    let mut file = file;
    if let Err(error) = file.write_all(content.as_bytes()) {
        let _ = fs::remove_file(&path);
        return Err(error.into());
    }
    Ok(path)
}

pub fn launch(
    path: &str,
    record: &GameRecord,
    token: &str,
    platform: &dyn Platform,
) -> Result<u32> {
    let (exe, dir) = canonical_game(path)?;
    if !record.configured {
        return Err(CommandError::new(
            ErrorKind::InvalidPath,
            "Configure this game before launching it.",
        ));
    }
    if token.is_empty() {
        return Err(CommandError::new(
            ErrorKind::TokenExpired,
            "The session has ended. Sign in again.",
        ));
    }
    let token_path = write_launch_token(&dir, token)?;
    platform.acl().protect_token(&token_path)?;
    let pid = platform.process().spawn(
        &exe,
        &dir,
        &record.launch_args,
        record.ld_preload.as_deref(),
    );
    if pid.is_err() {
        let _ = fs::remove_file(&token_path);
    }
    pid
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::platform::FakePlatform;

    fn temp_dir(name: &str) -> PathBuf {
        let dir = std::env::temp_dir().join(format!("sapie-game-{}-{name}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    const ORIGINAL: &[u8] = b"SteamUser019\0SteamClient017\0";
    const FOREIGN: &[u8] = b"someone-elses-backup SteamUser016\0";

    #[test]
    fn token_file_is_private_and_contains_only_the_token() {
        let dir = temp_dir("token-file");
        let path = dir.join("steam_settings").join(TOKEN_FILE_NAME);
        fs::create_dir_all(path.parent().unwrap()).unwrap();
        fs::write(&path, "stale").unwrap();

        let token = "t".repeat(43);
        let written = write_launch_token(&dir, &token).unwrap();
        assert_eq!(written, path);
        let value: serde_json::Value =
            serde_json::from_str(&fs::read_to_string(&written).unwrap()).unwrap();
        let object = value.as_object().unwrap();
        assert_eq!(object.len(), 1);
        assert_eq!(object["session_token"], token);
        #[cfg(unix)]
        {
            use std::os::unix::fs::MetadataExt;
            assert_eq!(fs::metadata(&written).unwrap().mode() & 0o777, 0o600);
        }
        fs::remove_dir_all(&dir).unwrap();
    }

    fn fake_exe(dir: &Path, name: &str) -> PathBuf {
        let mut pe = vec![0u8; 0x100];
        pe[..2].copy_from_slice(b"MZ");
        pe[0x3c..0x40].copy_from_slice(&0x80u32.to_le_bytes());
        pe[0x80..0x84].copy_from_slice(b"PE\0\0");
        pe[0x84..0x86].copy_from_slice(&0x8664u16.to_le_bytes());
        let exe = dir.join(name);
        fs::write(&exe, pe).unwrap();
        exe
    }

    #[test]
    fn install_backs_up_the_original_and_restore_puts_it_back() {
        let dir = temp_dir("install");
        let exe = fake_exe(&dir, "game.exe");
        let name = dll_name("x64").unwrap();
        fs::write(dir.join(name), ORIGINAL).unwrap();
        let foreign = dir.join(format!("{name}.sapie-other.bak"));
        fs::write(&foreign, b"untouched").unwrap();
        let source = dir.join("source.bin");
        fs::write(&source, b"sapie-lib").unwrap();

        let installed =
            install(exe.to_str().unwrap(), Some(&source), None, Some(&|_, _| {})).unwrap();
        assert_eq!(installed.arch, "x64");
        assert_eq!(installed.era, "modern");
        let backup = dir.join(format!("{name}{BACKUP_SUFFIX}"));
        assert_eq!(fs::read(&backup).unwrap(), ORIGINAL);
        assert_eq!(fs::read(dir.join(name)).unwrap(), b"sapie-lib");
        let mut record = GameRecord::new(exe.to_str().unwrap());
        record.original_library = installed.original_library.clone();
        record.library = Some(installed.library.clone());
        let restored = restore(exe.to_str().unwrap(), Some(&record), None).unwrap();
        assert!(restored.is_some());
        assert_eq!(fs::read(dir.join(name)).unwrap(), ORIGINAL);
        assert!(!backup.exists());
        assert_eq!(fs::read(&foreign).unwrap(), b"untouched");
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn install_refuses_a_foreign_backup() {
        let dir = temp_dir("backup-conflict");
        let exe = fake_exe(&dir, "game.exe");
        let name = dll_name("x64").unwrap();
        fs::write(dir.join(name), ORIGINAL).unwrap();
        let backup = dir.join(format!("{name}.sapie-original.bak"));
        fs::write(&backup, FOREIGN).unwrap();
        let source = dir.join("source.bin");
        fs::write(&source, b"sapie-lib").unwrap();

        let error = install(exe.to_str().unwrap(), Some(&source), None, None).unwrap_err();
        assert_eq!(error.kind, ErrorKind::BackupConflict);
        assert_eq!(fs::read(&backup).unwrap(), FOREIGN);
        assert_eq!(fs::read(dir.join(name)).unwrap(), ORIGINAL);
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn configuring_writes_server_and_respects_the_appid_choice() {
        let dir = temp_dir("config");
        let exe = fake_exe(&dir, "iw4mp.exe");
        let result = configure(exe.to_str().unwrap(), 480, "192.168.1.10:8081", None).unwrap();
        assert_eq!(result.appid, Some(480));
        assert_eq!(result.game_server, "192.168.1.10:47584");
        assert_eq!(
            fs::read_to_string(dir.join("sapie_server.txt")).unwrap(),
            "192.168.1.10:47584\n"
        );
        assert_eq!(
            fs::read_to_string(dir.join("steam_appid.txt")).unwrap(),
            "480\n"
        );

        let kept = configure(exe.to_str().unwrap(), 0, "127.0.0.1:8081", None).unwrap();
        assert_eq!(kept.appid, Some(480));
        fs::write(dir.join("steam_appid.txt"), "").unwrap();
        let auto = configure(exe.to_str().unwrap(), 0, "127.0.0.1:8081", None).unwrap();
        assert_eq!(auto.appid, Some(10190));
        assert_eq!(
            fs::read_to_string(dir.join("steam_appid.txt")).unwrap(),
            "10190\n"
        );
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn writes_are_scoped_to_the_game_directory() {
        let dir = temp_dir("scope");
        assert!(scoped(&dir, "../outside.txt").is_err());
        assert_eq!(scoped(&dir, "safe.txt").unwrap(), dir.join("safe.txt"));
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn paths_must_be_existing_files() {
        let dir = temp_dir("paths");
        assert_eq!(
            canonical_game(dir.to_str().unwrap()).unwrap_err().kind,
            ErrorKind::InvalidPath
        );
        assert_eq!(
            canonical_game(dir.join("missing.exe").to_str().unwrap())
                .unwrap_err()
                .kind,
            ErrorKind::NotFound
        );
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn launch_uses_the_injected_process_platform() {
        let dir = temp_dir("launch");
        let exe = fake_exe(&dir, "game.exe");
        let mut record = GameRecord::new(exe.to_str().unwrap());
        record.configured = true;
        record.launch_args = vec!["-windowed".into()];
        record.ld_preload = Some("/lib/fake.so".into());
        let fake = FakePlatform::default();
        let pid = launch(exe.to_str().unwrap(), &record, "tok", &fake).unwrap();
        assert_eq!(pid, 4242);
        let calls = fake.spawned();
        assert_eq!(calls.len(), 1);
        assert_eq!(calls[0].exe, exe);
        assert_eq!(calls[0].args, ["-windowed".to_string()]);
        assert_eq!(calls[0].ld_preload.as_deref(), Some("/lib/fake.so"));
        assert!(dir.join("steam_settings").join(TOKEN_FILE_NAME).is_file());
        fs::remove_dir_all(&dir).unwrap();
    }
}
