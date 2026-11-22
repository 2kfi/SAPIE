use crate::error::{CommandError, ErrorKind, Result};
use std::collections::BTreeMap;
use std::fs;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::Mutex;

const STATE_DIR: &str = "com.sapie.launcher";

pub trait Paths: Send + Sync {
    fn data_dir(&self) -> Result<PathBuf>;
}

pub trait Keychain: Send + Sync {
    fn get(&self) -> Result<Option<String>>;
    fn set(&self, token: &str) -> Result<()>;
    fn clear(&self) -> Result<()>;
}

pub trait Process: Send + Sync {
    fn spawn(
        &self,
        exe: &Path,
        dir: &Path,
        args: &[String],
        ld_preload: Option<&str>,
    ) -> Result<u32>;
}

pub trait FileSystem: Send + Sync {
    fn read(&self, path: &Path) -> Result<Vec<u8>>;
    fn write(&self, path: &Path, bytes: &[u8]) -> Result<()>;
    fn copy(&self, from: &Path, to: &Path) -> Result<()>;
    fn rename(&self, from: &Path, to: &Path) -> Result<()>;
    fn remove(&self, path: &Path) -> Result<()>;
    fn exists(&self, path: &Path) -> bool;
}

pub trait Acl: Send + Sync {
    fn protect_token(&self, path: &Path) -> Result<()>;
}

pub trait Updater: Send + Sync {
    fn latest_version(&self, channel: &str) -> Result<Option<String>>;
}

pub trait Platform: Send + Sync {
    fn paths(&self) -> &dyn Paths;
    fn keychain(&self) -> &dyn Keychain;
    fn process(&self) -> &dyn Process;
    fn files(&self) -> &dyn FileSystem;
    fn acl(&self) -> &dyn Acl;
    fn updater(&self) -> &dyn Updater;
}

pub struct SystemPlatform;

pub struct SystemFileSystem;

impl FileSystem for SystemFileSystem {
    fn read(&self, path: &Path) -> Result<Vec<u8>> {
        Ok(fs::read(path)?)
    }

    fn write(&self, path: &Path, bytes: &[u8]) -> Result<()> {
        let mut file = fs::File::create(path)?;
        file.write_all(bytes)?;
        file.sync_all()?;
        Ok(())
    }

    fn copy(&self, from: &Path, to: &Path) -> Result<()> {
        fs::copy(from, to)?;
        Ok(())
    }

    fn rename(&self, from: &Path, to: &Path) -> Result<()> {
        fs::rename(from, to)?;
        Ok(())
    }

    fn remove(&self, path: &Path) -> Result<()> {
        fs::remove_file(path)?;
        Ok(())
    }

    fn exists(&self, path: &Path) -> bool {
        path.is_file()
    }
}

impl Paths for SystemPlatform {
    fn data_dir(&self) -> Result<PathBuf> {
        let dir = dirs::data_dir()
            .ok_or_else(|| CommandError::new(ErrorKind::Io, "No per-user data directory found."))?
            .join(STATE_DIR);
        std::fs::create_dir_all(&dir)?;
        Ok(dir)
    }
}

impl Keychain for SystemPlatform {
    fn get(&self) -> Result<Option<String>> {
        let entry = keyring::Entry::new("com.sapie.launcher", "session-token").map_err(|e| {
            CommandError::new(
                ErrorKind::KeychainError,
                format!("Could not open the OS keychain: {e}"),
            )
        })?;
        match entry.get_password() {
            Ok(token) => Ok(Some(token)),
            Err(keyring::Error::NoEntry) => Ok(None),
            Err(e) => Err(CommandError::new(
                ErrorKind::KeychainError,
                format!("Could not read the session token: {e}"),
            )),
        }
    }

    fn set(&self, token: &str) -> Result<()> {
        let entry = keyring::Entry::new("com.sapie.launcher", "session-token").map_err(|e| {
            CommandError::new(
                ErrorKind::KeychainError,
                format!("Could not open the OS keychain: {e}"),
            )
        })?;
        entry.set_password(token).map_err(|e| {
            CommandError::new(
                ErrorKind::KeychainError,
                format!("Could not save the session token: {e}"),
            )
        })
    }

    fn clear(&self) -> Result<()> {
        let entry = keyring::Entry::new("com.sapie.launcher", "session-token").map_err(|e| {
            CommandError::new(
                ErrorKind::KeychainError,
                format!("Could not open the OS keychain: {e}"),
            )
        })?;
        match entry.delete_credential() {
            Ok(()) | Err(keyring::Error::NoEntry) => Ok(()),
            Err(e) => Err(CommandError::new(
                ErrorKind::KeychainError,
                format!("Could not remove the session token: {e}"),
            )),
        }
    }
}

impl Process for SystemPlatform {
    fn spawn(
        &self,
        exe: &Path,
        dir: &Path,
        args: &[String],
        ld_preload: Option<&str>,
    ) -> Result<u32> {
        let mut command = Command::new(exe);
        command.current_dir(dir).args(args);
        #[cfg(target_os = "linux")]
        if let Some(lib) = ld_preload {
            command.env("LD_PRELOAD", lib);
        }
        #[cfg(not(target_os = "linux"))]
        let _ = ld_preload;
        let child = command.spawn().map_err(|e| {
            CommandError::new(ErrorKind::Process, format!("Could not start the game: {e}"))
        })?;
        Ok(child.id())
    }
}

impl Acl for SystemPlatform {
    fn protect_token(&self, path: &Path) -> Result<()> {
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o600))?;
        }
        #[cfg(not(unix))]
        {
            // shortcut: the Windows owner-only DACL arrives with the Windows platform stage (M24-b).
            let _ = path;
        }
        Ok(())
    }
}

impl Updater for SystemPlatform {
    fn latest_version(&self, channel: &str) -> Result<Option<String>> {
        let _ = channel;
        // shortcut: signed manifests and channel apply land in M29. The 0.4 shell had no updater.
        Ok(None)
    }
}

impl Platform for SystemPlatform {
    fn paths(&self) -> &dyn Paths {
        self
    }
    fn keychain(&self) -> &dyn Keychain {
        self
    }
    fn process(&self) -> &dyn Process {
        self
    }
    fn files(&self) -> &dyn FileSystem {
        &SystemFileSystem
    }
    fn acl(&self) -> &dyn Acl {
        self
    }
    fn updater(&self) -> &dyn Updater {
        self
    }
}

#[derive(Clone, Debug)]
pub struct SpawnCall {
    pub exe: PathBuf,
    pub dir: PathBuf,
    pub args: Vec<String>,
    pub ld_preload: Option<String>,
}

#[derive(Default)]
pub struct FakeFileSystem {
    files: Mutex<BTreeMap<PathBuf, Vec<u8>>>,
}

impl FakeFileSystem {
    pub fn seed(&self, path: &Path, bytes: &[u8]) {
        self.files
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .insert(path.to_path_buf(), bytes.to_vec());
    }

    pub fn stored(&self, path: &Path) -> Option<Vec<u8>> {
        self.files
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .get(path)
            .cloned()
    }
}

impl FileSystem for FakeFileSystem {
    fn read(&self, path: &Path) -> Result<Vec<u8>> {
        self.stored(path).ok_or_else(|| {
            CommandError::path(ErrorKind::NotFound, path, "File not found in fake filesystem")
        })
    }

    fn write(&self, path: &Path, bytes: &[u8]) -> Result<()> {
        self.seed(path, bytes);
        Ok(())
    }

    fn copy(&self, from: &Path, to: &Path) -> Result<()> {
        let bytes = self.read(from)?;
        self.seed(to, &bytes);
        Ok(())
    }

    fn rename(&self, from: &Path, to: &Path) -> Result<()> {
        let bytes = self.read(from)?;
        {
            let mut files = self.files.lock().unwrap_or_else(|e| e.into_inner());
            files.remove(from);
            files.insert(to.to_path_buf(), bytes);
        }
        Ok(())
    }

    fn remove(&self, path: &Path) -> Result<()> {
        let mut files = self.files.lock().unwrap_or_else(|e| e.into_inner());
        match files.remove(path) {
            Some(_) => Ok(()),
            None => Err(CommandError::path(
                ErrorKind::NotFound,
                path,
                "File not found in fake filesystem",
            )),
        }
    }

    fn exists(&self, path: &Path) -> bool {
        self.files
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .contains_key(path)
    }
}

#[derive(Default)]
pub struct FakePlatform {
    token: Mutex<Option<String>>,
    spawns: Mutex<Vec<SpawnCall>>,
    files: FakeFileSystem,
    pub data_dir: Option<PathBuf>,
}

impl FakePlatform {
    pub fn with_data_dir(dir: PathBuf) -> Self {
        Self {
            data_dir: Some(dir),
            ..Self::default()
        }
    }

    pub fn spawned(&self) -> Vec<SpawnCall> {
        self.spawns
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .clone()
    }

    pub fn seed_file(&self, path: &Path, bytes: &[u8]) {
        self.files.seed(path, bytes);
    }

    pub fn stored_file(&self, path: &Path) -> Option<Vec<u8>> {
        self.files.stored(path)
    }
}

impl Paths for FakePlatform {
    fn data_dir(&self) -> Result<PathBuf> {
        let dir = self.data_dir.clone().unwrap_or_else(|| {
            std::env::temp_dir().join(format!("sapie-fake-core-{}", std::process::id()))
        });
        std::fs::create_dir_all(&dir)?;
        Ok(dir)
    }
}

impl Keychain for FakePlatform {
    fn get(&self) -> Result<Option<String>> {
        Ok(self.token.lock().unwrap_or_else(|e| e.into_inner()).clone())
    }

    fn set(&self, token: &str) -> Result<()> {
        *self.token.lock().unwrap_or_else(|e| e.into_inner()) = Some(token.to_string());
        Ok(())
    }

    fn clear(&self) -> Result<()> {
        *self.token.lock().unwrap_or_else(|e| e.into_inner()) = None;
        Ok(())
    }
}

impl Process for FakePlatform {
    fn spawn(
        &self,
        exe: &Path,
        dir: &Path,
        args: &[String],
        ld_preload: Option<&str>,
    ) -> Result<u32> {
        self.spawns
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .push(SpawnCall {
                exe: exe.to_path_buf(),
                dir: dir.to_path_buf(),
                args: args.to_vec(),
                ld_preload: ld_preload.map(str::to_string),
            });
        Ok(4242)
    }
}

impl Acl for FakePlatform {
    fn protect_token(&self, _path: &Path) -> Result<()> {
        Ok(())
    }
}

impl Updater for FakePlatform {
    fn latest_version(&self, _channel: &str) -> Result<Option<String>> {
        Ok(None)
    }
}

impl Platform for FakePlatform {
    fn paths(&self) -> &dyn Paths {
        self
    }
    fn keychain(&self) -> &dyn Keychain {
        self
    }
    fn process(&self) -> &dyn Process {
        self
    }
    fn files(&self) -> &dyn FileSystem {
        &self.files
    }
    fn acl(&self) -> &dyn Acl {
        self
    }
    fn updater(&self) -> &dyn Updater {
        self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fake_keychain_and_process_record_calls_without_touching_the_os() {
        let fake = FakePlatform::default();
        fake.keychain().set("token").unwrap();
        assert_eq!(fake.keychain().get().unwrap().as_deref(), Some("token"));
        fake.keychain().clear().unwrap();
        assert_eq!(fake.keychain().get().unwrap(), None);

        let pid = fake
            .process()
            .spawn(
                Path::new("game"),
                Path::new("dir"),
                &["a".into()],
                Some("lib.so"),
            )
            .unwrap();
        assert_eq!(pid, 4242);
        let calls = fake.spawned();
        assert_eq!(calls.len(), 1);
        assert_eq!(calls[0].exe, Path::new("game"));
        assert_eq!(calls[0].args, ["a".to_string()]);
        assert_eq!(calls[0].ld_preload.as_deref(), Some("lib.so"));
        assert!(fake.paths().data_dir().unwrap().is_dir());
        assert_eq!(fake.updater().latest_version("stable").unwrap(), None);
        fake.acl().protect_token(Path::new("token")).unwrap();
    }

    #[test]
    fn fake_filesystem_records_files_without_touching_the_os() {
        let fake = FakePlatform::default();
        let first = Path::new("/state/launcher-state.json");
        let second = Path::new("/state/launcher-state.json.bak");

        fake.files().write(first, b"first").unwrap();
        fake.files().copy(first, second).unwrap();
        assert_eq!(fake.files().read(second).unwrap(), b"first");

        fake.files().write(first, b"second").unwrap();
        fake.files().rename(first, second).unwrap();
        assert!(!fake.files().exists(first));
        assert_eq!(fake.files().read(second).unwrap(), b"second");

        fake.files().remove(second).unwrap();
        assert!(!fake.files().exists(second));
        assert!(fake.files().read(second).is_err());
    }
}
