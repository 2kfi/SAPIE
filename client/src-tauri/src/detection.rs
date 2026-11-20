use std::fs;
use std::path::{Path, PathBuf};

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

pub(crate) fn detect_arch(bytes: &[u8]) -> String {
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
pub(crate) fn scan_interfaces(bytes: &[u8]) -> Vec<String> {
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
    if !found
        .iter()
        .any(|s| s.starts_with("STEAMCONTROLLER_INTERFACE_VERSION"))
    {
        if let Some(&start) = find_all(bytes, key).first() {
            let end = start + key.len();
            let digits =
                if end + 3 <= bytes.len() && bytes[end..end + 3].iter().all(u8::is_ascii_digit) {
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
pub(crate) fn era_from_interfaces(interfaces: &[String]) -> String {
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

pub(crate) fn find_original_steam_api(dir: &Path) -> Option<PathBuf> {
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

pub(crate) fn ensure_steam_appid(
    dir: &Path,
    exe_path: &str,
) -> std::result::Result<String, String> {
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

pub(crate) fn check_runtime_deps(dir: &Path, dll_name: &str) -> Option<String> {
    if dir.join("libwinpthread-1.dll").is_file() {
        None
    } else {
        Some(format!(
            "libwinpthread-1.dll is not beside {dll_name}. Static builds do not import it; if the game reports it missing, put the DLL there."
        ))
    }
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
        assert!(ensure_steam_appid(&dir, "/games/iw4mp.exe")
            .unwrap()
            .contains("10190"));
        assert_eq!(
            fs::read_to_string(dir.join("steam_appid.txt")).unwrap(),
            "10190\n"
        );
        fs::write(dir.join("steam_appid.txt"), "999\n").unwrap();
        assert!(ensure_steam_appid(&dir, "/games/iw4sp.exe")
            .unwrap()
            .contains("999"));
        assert_eq!(
            fs::read_to_string(dir.join("steam_appid.txt")).unwrap(),
            "999\n"
        );

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
