use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT_DIR: AtomicU64 = AtomicU64::new(0);

pub struct GameFixture {
    pub root: PathBuf,
}

impl GameFixture {
    pub fn new(name: &str) -> Self {
        let unique = NEXT_DIR.fetch_add(1, Ordering::Relaxed);
        let root = std::env::temp_dir().join(format!(
            "sapie-game-fixture-{}-{}-{}",
            std::process::id(),
            unique,
            name
        ));
        let _ = fs::remove_dir_all(&root);
        fs::create_dir_all(&root).expect("fixture directory");
        Self { root }
    }

    pub fn write(&self, name: &str, bytes: &[u8]) -> PathBuf {
        let path = self.root.join(name);
        fs::write(&path, bytes).expect("fixture file");
        path
    }
}

impl Drop for GameFixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

pub fn pe_bytes(machine: u16) -> Vec<u8> {
    let mut bytes = vec![0u8; 0x100];
    bytes[..2].copy_from_slice(b"MZ");
    bytes[0x3c..0x40].copy_from_slice(&0x80u32.to_le_bytes());
    bytes[0x80..0x84].copy_from_slice(b"PE\0\0");
    bytes[0x84..0x86].copy_from_slice(&machine.to_le_bytes());
    bytes
}

pub fn steam_api_bytes() -> Vec<u8> {
    steam_api_bytes_with(19, 17)
}

pub fn steam_api_bytes_with(user_version: u16, client_version: u16) -> Vec<u8> {
    let mut bytes =
        format!("SteamUser{user_version:03}\0SteamClient{client_version:03}\0").into_bytes();
    bytes.extend_from_slice(&[0u8; 0x20]);
    bytes
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::detection::{detect_arch, era_from_interfaces, scan_interfaces};

    #[test]
    fn game_fixture_builds_a_real_x64_modern_layout() {
        let fixture = GameFixture::new("x64-modern");
        let exe = fixture.write("game.exe", &pe_bytes(0x8664));
        let library = fixture.write("steam_api64.dll.sapie-original.bak", &steam_api_bytes());

        assert_eq!(detect_arch(&fs::read(exe).unwrap()), "x64");
        let interfaces = scan_interfaces(&fs::read(library).unwrap());
        assert!(interfaces.contains(&"SteamUser019".to_string()));
        assert_eq!(era_from_interfaces(&interfaces), "modern");
    }

    #[test]
    fn game_fixture_builds_a_real_x86_legacy_layout() {
        let fixture = GameFixture::new("x86-legacy");
        let exe = fixture.write("game.exe", &pe_bytes(0x014c));
        let library = fixture.write(
            "steam_api.dll.sapie-original.bak",
            &steam_api_bytes_with(16, 16),
        );

        assert_eq!(detect_arch(&fs::read(exe).unwrap()), "x86");
        assert_eq!(
            era_from_interfaces(&scan_interfaces(&fs::read(library).unwrap())),
            "legacy"
        );
        assert!(Path::new(&fixture.root).is_dir());
    }
}
