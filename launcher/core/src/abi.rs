// The C header is the safety contract for these extern functions; every pointer argument is checked.
#![allow(clippy::missing_safety_doc)]

use crate::auth;
use crate::error::{CommandError, ErrorKind, Result};
use crate::game;
use crate::platform::{FakePlatform, Platform, SystemPlatform};
use crate::state::{self, GameRecord, LauncherState};
use std::ffi::{CStr, CString};
use std::os::raw::{c_char, c_void};
use std::ptr;
use std::sync::{Arc, Mutex};

pub const ABI_VERSION: u32 = 1;

pub type EventCallback = unsafe extern "C" fn(*mut c_void, *const c_char, *const c_char);

#[repr(u32)]
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Status {
    Ok = 0,
    InvalidArgument = 1,
    InvalidPath = 2,
    NotFound = 3,
    PermissionDenied = 4,
    BackupConflict = 5,
    InvalidServer = 6,
    Unreachable = 7,
    BadCredentials = 8,
    ServerTooOld = 9,
    Server = 10,
    TokenExpired = 11,
    Keychain = 12,
    Io = 13,
    Process = 14,
    AbiVersion = 15,
}

#[repr(C)]
pub struct SapieSessionInfo {
    pub session_token: *mut c_char,
    pub steam_id: u64,
    pub persona_name: *mut c_char,
    pub server: *mut c_char,
    pub expires_at: u64,
}

#[repr(C)]
pub struct SapieDetectResult {
    pub exe_path: *mut c_char,
    pub arch: *mut c_char,
    pub era: *mut c_char,
    pub interfaces: *mut *mut c_char,
    pub interface_count: usize,
    pub original_library: *mut c_char,
    pub warning: *mut c_char,
}

#[repr(C)]
pub struct SapieInstallResult {
    pub arch: *mut c_char,
    pub era: *mut c_char,
    pub interfaces: *mut *mut c_char,
    pub interface_count: usize,
    pub original_library: *mut c_char,
    pub library: *mut c_char,
    pub warning: *mut c_char,
}

#[repr(C)]
pub struct SapieConfigureResult {
    pub appid: u32,
    pub game_server: *mut c_char,
    pub note: *mut c_char,
}

#[repr(C)]
pub struct SapieGameRecord {
    pub exe_path: *mut c_char,
    pub arch: *mut c_char,
    pub era: *mut c_char,
    pub interfaces: *mut *mut c_char,
    pub interface_count: usize,
    pub original_library: *mut c_char,
    pub library: *mut c_char,
    pub appid: u32,
    pub configured: i32,
    pub launch_args: *mut *mut c_char,
    pub launch_arg_count: usize,
    pub ld_preload: *mut c_char,
    pub last_played: u64,
    pub favorite: i32,
}

struct Core {
    platform: Arc<dyn Platform>,
    state: Mutex<LauncherState>,
}

impl Core {
    fn new(platform: Arc<dyn Platform>) -> Result<Self> {
        Ok(Self {
            state: Mutex::new(state::load(platform.as_ref())?),
            platform,
        })
    }

    fn snapshot(&self) -> Result<LauncherState> {
        Ok(self
            .state
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .clone())
    }

    fn update<T>(&self, apply: impl FnOnce(&mut LauncherState) -> Result<T>) -> Result<T> {
        let mut data = self.state.lock().unwrap_or_else(|e| e.into_inner());
        let value = apply(&mut data)?;
        state::save(self.platform.as_ref(), &data)?;
        Ok(value)
    }
}

pub struct SapieCore {
    inner: Core,
    callback: Option<EventCallback>,
    user_data: *mut c_void,
}

impl SapieCore {
    fn emit(&self, step: &str, message: &str) {
        if let Some(callback) = self.callback {
            let step_c = CString::new(step).unwrap_or_default();
            let message_c = CString::new(message).unwrap_or_default();
            unsafe { callback(self.user_data, step_c.as_ptr(), message_c.as_ptr()) };
        }
    }
}

fn status_of(error: &CommandError) -> Status {
    match error.kind {
        ErrorKind::InvalidPath => Status::InvalidPath,
        ErrorKind::NotFound => Status::NotFound,
        ErrorKind::PermissionDenied => Status::PermissionDenied,
        ErrorKind::BackupConflict => Status::BackupConflict,
        ErrorKind::InvalidServer => Status::InvalidServer,
        ErrorKind::Unreachable => Status::Unreachable,
        ErrorKind::BadCredentials => Status::BadCredentials,
        ErrorKind::ServerTooOld => Status::ServerTooOld,
        ErrorKind::ServerError => Status::Server,
        ErrorKind::TokenExpired => Status::TokenExpired,
        ErrorKind::KeychainError => Status::Keychain,
        ErrorKind::Io => Status::Io,
        ErrorKind::Process => Status::Process,
    }
}

unsafe fn as_core<'a>(ptr: *mut SapieCore) -> Option<&'a SapieCore> {
    ptr.as_ref()
}

unsafe fn as_str<'a>(ptr: *const c_char) -> Option<&'a str> {
    if ptr.is_null() {
        return None;
    }
    CStr::from_ptr(ptr).to_str().ok()
}

unsafe fn str_array(ptr: *const *const c_char, count: usize) -> Option<Vec<String>> {
    if count == 0 {
        return Some(Vec::new());
    }
    if ptr.is_null() {
        return None;
    }
    let mut values = Vec::with_capacity(count);
    for index in 0..count {
        {
            let value = as_str(*ptr.add(index))?;
            values.push(value.to_string());
        }
    }
    Some(values)
}

fn join_lobby_id(url: &str) -> Option<&str> {
    let id = url.strip_prefix("sapie://join/")?.split(['?', '#', '/']).next()?;
    (!id.is_empty() && id.bytes().all(|byte| byte.is_ascii_digit())).then_some(id)
}

fn c_string(value: &str) -> *mut c_char {
    CString::new(value)
        .unwrap_or_default()
        .into_raw()
}

fn opt_string(value: Option<&str>) -> *mut c_char {
    value.map(c_string).unwrap_or(ptr::null_mut())
}

fn c_string_array(values: &[String]) -> (*mut *mut c_char, usize) {
    if values.is_empty() {
        return (ptr::null_mut(), 0);
    }
    let raw: Vec<*mut c_char> = values.iter().map(|value| c_string(value)).collect();
    let boxed = raw.into_boxed_slice();
    let count = boxed.len();
    (Box::into_raw(boxed) as *mut *mut c_char, count)
}

unsafe fn free_string(value: *mut c_char) {
    if !value.is_null() {
        drop(CString::from_raw(value));
    }
}

unsafe fn free_string_array(values: *mut *mut c_char, count: usize) {
    if values.is_null() || count == 0 {
        return;
    }
    let raw = Vec::from_raw_parts(values, count, count);
    for value in raw {
        free_string(value);
    }
}

unsafe fn write_out<T>(out: *mut T, value: T) -> Status {
    if out.is_null() {
        return Status::InvalidArgument;
    }
    ptr::write(out, value);
    Status::Ok
}

impl Drop for SapieSessionInfo {
    fn drop(&mut self) {
        unsafe {
            free_string(self.session_token);
            free_string(self.persona_name);
            free_string(self.server);
        }
    }
}

impl Drop for SapieDetectResult {
    fn drop(&mut self) {
        unsafe {
            free_string(self.exe_path);
            free_string(self.arch);
            free_string(self.era);
            free_string_array(self.interfaces, self.interface_count);
            free_string(self.original_library);
            free_string(self.warning);
        }
    }
}

impl Drop for SapieInstallResult {
    fn drop(&mut self) {
        unsafe {
            free_string(self.arch);
            free_string(self.era);
            free_string_array(self.interfaces, self.interface_count);
            free_string(self.original_library);
            free_string(self.library);
            free_string(self.warning);
        }
    }
}

impl Drop for SapieConfigureResult {
    fn drop(&mut self) {
        unsafe {
            free_string(self.game_server);
            free_string(self.note);
        }
    }
}

impl Drop for SapieGameRecord {
    fn drop(&mut self) {
        unsafe {
            free_string(self.exe_path);
            free_string(self.arch);
            free_string(self.era);
            free_string_array(self.interfaces, self.interface_count);
            free_string(self.original_library);
            free_string(self.library);
            free_string_array(self.launch_args, self.launch_arg_count);
            free_string(self.ld_preload);
        }
    }
}

impl From<&auth::SessionInfo> for SapieSessionInfo {
    fn from(value: &auth::SessionInfo) -> Self {
        Self {
            session_token: c_string(&value.token),
            steam_id: value.steam_id,
            persona_name: c_string(&value.persona_name),
            server: c_string(&value.server),
            expires_at: value.expires_at,
        }
    }
}

impl From<&game::DetectResult> for SapieDetectResult {
    fn from(value: &game::DetectResult) -> Self {
        let (interfaces, interface_count) = c_string_array(&value.interfaces);
        Self {
            exe_path: c_string(&value.exe_path),
            arch: c_string(&value.arch),
            era: c_string(&value.era),
            interfaces,
            interface_count,
            original_library: opt_string(value.original_library.as_deref()),
            warning: opt_string(value.warning.as_deref()),
        }
    }
}

impl From<&game::InstallResult> for SapieInstallResult {
    fn from(value: &game::InstallResult) -> Self {
        let (interfaces, interface_count) = c_string_array(&value.interfaces);
        Self {
            arch: c_string(&value.arch),
            era: c_string(&value.era),
            interfaces,
            interface_count,
            original_library: opt_string(value.original_library.as_deref()),
            library: c_string(&value.library),
            warning: opt_string(value.warning.as_deref()),
        }
    }
}

impl From<&game::ConfigureResult> for SapieConfigureResult {
    fn from(value: &game::ConfigureResult) -> Self {
        Self {
            appid: value.appid.unwrap_or(0),
            game_server: c_string(&value.game_server),
            note: c_string(&value.note),
        }
    }
}

impl From<&GameRecord> for SapieGameRecord {
    fn from(value: &GameRecord) -> Self {
        let (interfaces, interface_count) = c_string_array(&value.interfaces);
        let (launch_args, launch_arg_count) = c_string_array(&value.launch_args);
        Self {
            exe_path: c_string(&value.exe_path),
            arch: c_string(&value.arch),
            era: c_string(&value.era),
            interfaces,
            interface_count,
            original_library: opt_string(value.original_library.as_deref()),
            library: opt_string(value.library.as_deref()),
            appid: value.appid.unwrap_or(0),
            configured: i32::from(value.configured),
            launch_args,
            launch_arg_count,
            ld_preload: opt_string(value.ld_preload.as_deref()),
            last_played: value.last_played.unwrap_or(0),
            favorite: i32::from(value.favorite),
        }
    }
}

fn add_record(state: &mut LauncherState, mut record: GameRecord) -> GameRecord {
    let key = record.exe_path.clone();
    if let Some(existing) = state.games.get_mut(&key) {
        existing.arch = record.arch;
        existing.era = record.era;
        existing.interfaces = std::mem::take(&mut record.interfaces);
        existing.original_library = record.original_library;
        return existing.clone();
    }
    state.games.insert(key, record.clone());
    record
}

unsafe fn create_with(
    platform: Arc<dyn Platform>,
    callback: Option<EventCallback>,
    user_data: *mut c_void,
    out_core: *mut *mut SapieCore,
) -> Status {
    if out_core.is_null() {
        return Status::InvalidArgument;
    }
    ptr::write(out_core, ptr::null_mut());
    match Core::new(platform) {
        Ok(inner) => write_out(
            out_core,
            Box::into_raw(Box::new(SapieCore {
                inner,
                callback,
                user_data,
            })),
        ),
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub extern "C" fn sapie_core_abi_version() -> u32 {
    ABI_VERSION
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_create(
    version: u32,
    callback: Option<EventCallback>,
    user_data: *mut c_void,
    out_core: *mut *mut SapieCore,
) -> Status {
    if out_core.is_null() {
        return Status::InvalidArgument;
    }
    ptr::write(out_core, ptr::null_mut());
    if version != ABI_VERSION {
        return Status::AbiVersion;
    }
    create_with(Arc::new(SystemPlatform), callback, user_data, out_core)
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_create_fake(
    version: u32,
    callback: Option<EventCallback>,
    user_data: *mut c_void,
    out_core: *mut *mut SapieCore,
) -> Status {
    if out_core.is_null() {
        return Status::InvalidArgument;
    }
    ptr::write(out_core, ptr::null_mut());
    if version != ABI_VERSION {
        return Status::AbiVersion;
    }
    create_with(Arc::new(FakePlatform::default()), callback, user_data, out_core)
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_destroy(core: *mut SapieCore) {
    if !core.is_null() {
        drop(Box::from_raw(core));
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_string_free(value: *mut c_char) {
    free_string(value);
}

#[no_mangle]
pub unsafe extern "C" fn sapie_string_array_free(values: *mut *mut c_char, count: usize) {
    free_string_array(values, count);
}

#[no_mangle]
pub unsafe extern "C" fn sapie_session_info_free(value: *mut SapieSessionInfo) {
    if !value.is_null() {
        drop(Box::from_raw(value));
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_detect_result_free(value: *mut SapieDetectResult) {
    if !value.is_null() {
        drop(Box::from_raw(value));
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_install_result_free(value: *mut SapieInstallResult) {
    if !value.is_null() {
        drop(Box::from_raw(value));
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_configure_result_free(value: *mut SapieConfigureResult) {
    if !value.is_null() {
        drop(Box::from_raw(value));
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_game_record_free(value: *mut SapieGameRecord) {
    if !value.is_null() {
        drop(Box::from_raw(value));
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_game_record_array_free(values: *mut SapieGameRecord, count: usize) {
    if values.is_null() || count == 0 {
        return;
    }
    let slice = std::ptr::slice_from_raw_parts_mut(values, count);
    drop(Box::from_raw(slice));
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_test_connection(
    core: *mut SapieCore,
    server: *const c_char,
) -> Status {
    if as_core(core).is_none() {
        return Status::InvalidArgument;
    }
    let server = match as_str(server) {
        Some(server) => server,
        None => return Status::InvalidArgument,
    };
    match auth::test_connection(server) {
        Ok(()) => Status::Ok,
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_login(
    core: *mut SapieCore,
    server: *const c_char,
    username: *const c_char,
    password: *const c_char,
    out_session: *mut *mut SapieSessionInfo,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let (server, username, password) = match (as_str(server), as_str(username), as_str(password)) {
        (Some(server), Some(username), Some(password)) => (server, username, password),
        _ => return Status::InvalidArgument,
    };
    let session = match auth::login(
        server,
        username,
        password,
        core.inner.platform.keychain(),
    ) {
        Ok(session) => session,
        Err(error) => return status_of(&error),
    };
    if let Err(error) = core.inner.update(|state| {
        state.server = Some(session.server.clone());
        state.username = Some(username.to_string());
        state.steam_id = Some(session.steam_id);
        state.persona_name = Some(session.persona_name.clone());
        state.token_expires_at = Some(session.expires_at);
        Ok(())
    }) {
        return status_of(&error);
    }
    write_out(out_session, Box::into_raw(Box::new(SapieSessionInfo::from(&session))))
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_sign_out(core: *mut SapieCore) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    if let Err(error) = core.inner.platform.keychain().clear() {
        return status_of(&error);
    }
    match core.inner.update(|state| {
        state.steam_id = None;
        state.persona_name = None;
        state.token_expires_at = None;
        Ok(())
    }) {
        Ok(()) => Status::Ok,
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_detect_game(
    core: *mut SapieCore,
    path: *const c_char,
    out_result: *mut *mut SapieDetectResult,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    let report = |step: &str, message: &str| core.emit(step, message);
    match game::detect(path, Some(&report)) {
        Ok(result) => write_out(
            out_result,
            Box::into_raw(Box::new(SapieDetectResult::from(&result))),
        ),
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_install_library(
    core: *mut SapieCore,
    path: *const c_char,
    out_result: *mut *mut SapieInstallResult,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    let record = match core.inner.snapshot() {
        Ok(state) => state.games.get(path).cloned(),
        Err(error) => return status_of(&error),
    };
    let report = |step: &str, message: &str| core.emit(step, message);
    let installed = match game::install(path, None, record.as_ref(), Some(&report)) {
        Ok(installed) => installed,
        Err(error) => return status_of(&error),
    };
    if let Err(error) = core.inner.update(|state| {
        let record = state
            .games
            .get_mut(path)
            .ok_or_else(|| CommandError::new(ErrorKind::NotFound, "Game is not in the library."))?;
        record.arch = installed.arch.clone();
        record.era = installed.era.clone();
        record.interfaces = installed.interfaces.clone();
        record.original_library = installed.original_library.clone();
        record.library = Some(installed.library.clone());
        Ok(())
    }) {
        return status_of(&error);
    }
    write_out(
        out_result,
        Box::into_raw(Box::new(SapieInstallResult::from(&installed))),
    )
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_restore_library(
    core: *mut SapieCore,
    path: *const c_char,
    out_restored: *mut *mut c_char,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    let record = match core.inner.snapshot() {
        Ok(state) => state.games.get(path).cloned(),
        Err(error) => return status_of(&error),
    };
    let report = |step: &str, message: &str| core.emit(step, message);
    let restored = match game::restore(path, record.as_ref(), Some(&report)) {
        Ok(restored) => restored,
        Err(error) => return status_of(&error),
    };
    if let Err(error) = core.inner.update(|state| {
        if let Some(record) = state.games.get_mut(path) {
            record.original_library = None;
            record.library = None;
        }
        Ok(())
    }) {
        return status_of(&error);
    }
    write_out(out_restored, opt_string(restored.as_deref()))
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_configure_game(
    core: *mut SapieCore,
    path: *const c_char,
    appid: u32,
    out_result: *mut *mut SapieConfigureResult,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    let server = match core.inner.snapshot() {
        Ok(state) => match state.server {
            Some(server) => server,
            None => {
                return status_of(&CommandError::new(
                    ErrorKind::InvalidServer,
                    "Sign in before configuring a game.",
                ))
            }
        },
        Err(error) => return status_of(&error),
    };
    let report = |step: &str, message: &str| core.emit(step, message);
    let configured = match game::configure(path, appid, &server, Some(&report)) {
        Ok(configured) => configured,
        Err(error) => return status_of(&error),
    };
    if let Err(error) = core.inner.update(|state| {
        let record = state
            .games
            .get_mut(path)
            .ok_or_else(|| CommandError::new(ErrorKind::NotFound, "Game is not in the library."))?;
        record.appid = configured.appid;
        record.configured = true;
        Ok(())
    }) {
        return status_of(&error);
    }
    write_out(
        out_result,
        Box::into_raw(Box::new(SapieConfigureResult::from(&configured))),
    )
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_launch_game(
    core: *mut SapieCore,
    path: *const c_char,
    out_pid: *mut u32,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    let state = match core.inner.snapshot() {
        Ok(state) => state,
        Err(error) => return status_of(&error),
    };
    let record = match state.games.get(path).cloned() {
        Some(record) => record,
        None => {
            return status_of(&CommandError::new(
                ErrorKind::NotFound,
                "Game is not in the library.",
            ))
        }
    };
    let token = match core.inner.platform.keychain().get() {
        Ok(Some(token)) => token,
        Ok(None) => {
            return status_of(&CommandError::new(
                ErrorKind::TokenExpired,
                "The session has ended. Sign in again.",
            ))
        }
        Err(error) => return status_of(&error),
    };
    if auth::is_expired(state.token_expires_at) {
        return Status::TokenExpired;
    }
    let pid = match game::launch(path, &record, &token, core.inner.platform.as_ref()) {
        Ok(pid) => pid,
        Err(error) => return status_of(&error),
    };
    if let Err(error) = core.inner.update(|state| {
        if let Some(record) = state.games.get_mut(path) {
            record.last_played = Some(auth::now_secs());
        }
        Ok(())
    }) {
        return status_of(&error);
    }
    write_out(out_pid, pid)
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_unpack_steamstub(
    core: *mut SapieCore,
    path: *const c_char,
    out_unpacked: *mut *mut c_char,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    let report = |step: &str, message: &str| core.emit(step, message);
    match game::unpack_steamstub(path, Some(&report)) {
        Ok(unpacked) => write_out(out_unpacked, opt_string(unpacked.as_deref())),
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_library_list(
    core: *mut SapieCore,
    out_records: *mut *mut SapieGameRecord,
    out_count: *mut usize,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    if out_records.is_null() || out_count.is_null() {
        return Status::InvalidArgument;
    }
    let state = match core.inner.snapshot() {
        Ok(state) => state,
        Err(error) => return status_of(&error),
    };
    let records: Vec<SapieGameRecord> = state.games.values().map(SapieGameRecord::from).collect();
    let count = records.len();
    let ptr = if count == 0 {
        ptr::null_mut()
    } else {
        Box::into_raw(records.into_boxed_slice()) as *mut SapieGameRecord
    };
    ptr::write(out_records, ptr);
    ptr::write(out_count, count);
    Status::Ok
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_library_add(
    core: *mut SapieCore,
    path: *const c_char,
    out_record: *mut *mut SapieGameRecord,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    let report = |step: &str, message: &str| core.emit(step, message);
    let detected = match game::detect(path, Some(&report)) {
        Ok(detected) => detected,
        Err(error) => return status_of(&error),
    };
    let mut record = GameRecord::new(&detected.exe_path);
    record.arch = detected.arch;
    record.era = detected.era;
    record.interfaces = detected.interfaces;
    record.original_library = detected.original_library;
    let record = match core.inner.update(|state| Ok(add_record(state, record))) {
        Ok(record) => record,
        Err(error) => return status_of(&error),
    };
    write_out(
        out_record,
        Box::into_raw(Box::new(SapieGameRecord::from(&record))),
    )
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_library_remove(
    core: *mut SapieCore,
    path: *const c_char,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    let record = match core.inner.snapshot() {
        Ok(state) => match state.games.get(path).cloned() {
            Some(record) => record,
            None => return Status::NotFound,
        },
        Err(error) => return status_of(&error),
    };
    let report = |step: &str, message: &str| core.emit(step, message);
    let outcome = match game::restore(path, Some(&record), Some(&report)) {
        Ok(_) => Ok(()),
        Err(error) if error.kind == ErrorKind::NotFound => game::remove_library(path, &record),
        Err(error) => Err(error),
    };
    if let Err(error) = outcome {
        return status_of(&error);
    }
    match core.inner.update(|state| {
        state.games.remove(path);
        Ok(())
    }) {
        Ok(()) => Status::Ok,
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_set_favorite(
    core: *mut SapieCore,
    path: *const c_char,
    favorite: i32,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let path = match as_str(path) {
        Some(path) => path,
        None => return Status::InvalidArgument,
    };
    match core.inner.update(|state| {
        let record = state
            .games
            .get_mut(path)
            .ok_or_else(|| CommandError::new(ErrorKind::NotFound, "Game is not in the library."))?;
        record.favorite = favorite != 0;
        Ok(())
    }) {
        Ok(()) => Status::Ok,
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_save_game_options(
    core: *mut SapieCore,
    path: *const c_char,
    args: *const *const c_char,
    arg_count: usize,
    ld_preload: *const c_char,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let (path, args, preload) = match (as_str(path), str_array(args, arg_count)) {
        (Some(path), Some(args)) => (path, args, as_str(ld_preload)),
        _ => return Status::InvalidArgument,
    };
    let ld_preload = preload.filter(|value| !value.is_empty()).map(str::to_string);
    match core.inner.update(|state| {
        let record = state
            .games
            .get_mut(path)
            .ok_or_else(|| CommandError::new(ErrorKind::NotFound, "Game is not in the library."))?;
        record.launch_args = args;
        record.ld_preload = ld_preload;
        Ok(())
    }) {
        Ok(()) => Status::Ok,
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_get_preference(
    core: *mut SapieCore,
    key: *const c_char,
    fallback: *const c_char,
    out_value: *mut *mut c_char,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let (key, fallback) = match (as_str(key), as_str(fallback)) {
        (Some(key), Some(fallback)) => (key, fallback),
        _ => return Status::InvalidArgument,
    };
    let value = match core.inner.snapshot() {
        Ok(state) => state
            .preferences
            .get(key)
            .map_or(fallback, String::as_str)
            .to_string(),
        Err(error) => return status_of(&error),
    };
    write_out(out_value, c_string(&value))
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_set_preference(
    core: *mut SapieCore,
    key: *const c_char,
    value: *const c_char,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let (key, value) = match (as_str(key), as_str(value)) {
        (Some(key), Some(value)) => (key, value),
        _ => return Status::InvalidArgument,
    };
    match core.inner.update(|state| {
        state.preferences.insert(key.to_string(), value.to_string());
        Ok(())
    }) {
        Ok(()) => Status::Ok,
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_data_folder(
    core: *mut SapieCore,
    out_path: *mut *mut c_char,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    match core.inner.platform.paths().data_dir() {
        Ok(path) => write_out(out_path, c_string(&path.to_string_lossy())),
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_launcher_version(out_version: *mut *mut c_char) -> Status {
    write_out(out_version, c_string(env!("CARGO_PKG_VERSION")))
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_check_updates(
    core: *mut SapieCore,
    channel: *const c_char,
    out_version: *mut *mut c_char,
) -> Status {
    let core = match as_core(core) {
        Some(core) => core,
        None => return Status::InvalidArgument,
    };
    let channel = match as_str(channel) {
        Some(channel) => channel,
        None => return Status::InvalidArgument,
    };
    match core.inner.platform.updater().latest_version(channel) {
        Ok(version) => write_out(out_version, c_string(version.as_deref().unwrap_or(""))),
        Err(error) => status_of(&error),
    }
}

#[no_mangle]
pub unsafe extern "C" fn sapie_core_parse_deep_link(
    url: *const c_char,
    out_lobby_id: *mut *mut c_char,
) -> Status {
    match as_str(url).and_then(join_lobby_id) {
        Some(id) => write_out(out_lobby_id, c_string(id)),
        None => Status::InvalidArgument,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn free_functions_accept_null_and_zeroed_structs() {
        unsafe {
            sapie_string_free(ptr::null_mut());
            sapie_string_array_free(ptr::null_mut(), 0);
            sapie_session_info_free(ptr::null_mut());
            sapie_detect_result_free(ptr::null_mut());
            sapie_install_result_free(ptr::null_mut());
            sapie_configure_result_free(ptr::null_mut());
            sapie_game_record_free(ptr::null_mut());
            sapie_game_record_array_free(ptr::null_mut(), 0);
        }
    }

    #[test]
    fn create_rejects_unknown_abi_versions() {
        let mut core: *mut SapieCore = ptr::dangling_mut();
        let status = unsafe { sapie_core_create(ABI_VERSION + 1, None, ptr::null_mut(), &mut core) };
        assert_eq!(status, Status::AbiVersion);
        assert!(core.is_null());
    }

    #[test]
    fn create_and_destroy_fake_core() {
        let mut core: *mut SapieCore = ptr::null_mut();
        let status = unsafe { sapie_core_create_fake(ABI_VERSION, None, ptr::null_mut(), &mut core) };
        assert_eq!(status, Status::Ok);
        assert!(!core.is_null());
        unsafe { sapie_core_destroy(core) };
    }

    #[test]
    fn library_list_roundtrips_owned_record_memory() {
        let mut core: *mut SapieCore = ptr::null_mut();
        assert_eq!(
            unsafe { sapie_core_create_fake(ABI_VERSION, None, ptr::null_mut(), &mut core) },
            Status::Ok
        );
        let mut records: *mut SapieGameRecord = ptr::null_mut();
        let mut count: usize = 99;
        assert_eq!(
            unsafe { sapie_core_library_list(core, &mut records, &mut count) },
            Status::Ok
        );
        assert_eq!(count, 0);
        assert!(records.is_null());
        unsafe { sapie_core_destroy(core) };
    }

    fn isolated_core(name: &str) -> (*mut SapieCore, std::path::PathBuf) {
        let dir = std::env::temp_dir().join(format!("sapie-abi-{}-{name}", std::process::id()));
        let _ = std::fs::remove_dir_all(&dir);
        let platform = FakePlatform::with_data_dir(dir.clone());
        let mut core = ptr::null_mut();
        assert_eq!(
            unsafe { create_with(Arc::new(platform), None, ptr::null_mut(), &mut core) },
            Status::Ok
        );
        (core, dir)
    }

    #[test]
    fn preference_version_folder_and_deep_link_roundtrip() {
        let (core, dir) = isolated_core("prefs");
        let key = CString::new("librarySort").unwrap();
        let fallback = CString::new("Name").unwrap();
        let value = CString::new("Last played").unwrap();
        let mut out = ptr::null_mut();
        assert_eq!(
            unsafe { sapie_core_get_preference(core, key.as_ptr(), fallback.as_ptr(), &mut out) },
            Status::Ok
        );
        assert_eq!(unsafe { as_str(out) }, Some("Name"));
        unsafe { free_string(out) };

        assert_eq!(
            unsafe { sapie_core_set_preference(core, key.as_ptr(), value.as_ptr()) },
            Status::Ok
        );
        out = ptr::null_mut();
        assert_eq!(
            unsafe { sapie_core_get_preference(core, key.as_ptr(), fallback.as_ptr(), &mut out) },
            Status::Ok
        );
        assert_eq!(unsafe { as_str(out) }, Some("Last played"));
        unsafe { free_string(out) };

        out = ptr::null_mut();
        assert_eq!(unsafe { sapie_core_data_folder(core, &mut out) }, Status::Ok);
        assert_eq!(unsafe { as_str(out) }, Some(dir.to_str().unwrap()));
        unsafe { free_string(out) };

        out = ptr::null_mut();
        assert_eq!(unsafe { sapie_core_launcher_version(&mut out) }, Status::Ok);
        assert!(!unsafe { as_str(out) }.unwrap_or_default().is_empty());
        unsafe { free_string(out) };

        let link = CString::new("sapie://join/123?exp=1&sig=abc").unwrap();
        out = ptr::null_mut();
        assert_eq!(
            unsafe { sapie_core_parse_deep_link(link.as_ptr(), &mut out) },
            Status::Ok
        );
        assert_eq!(unsafe { as_str(out) }, Some("123"));
        unsafe { free_string(out) };

        let bad = CString::new("sapie://join/not-a-number").unwrap();
        assert_eq!(
            unsafe { sapie_core_parse_deep_link(bad.as_ptr(), &mut out) },
            Status::InvalidArgument
        );
        unsafe { sapie_core_destroy(core) };
        let _ = std::fs::remove_dir_all(&dir);
    }

    #[test]
    fn favorite_and_launch_options_roundtrip_through_owned_strings() {
        let (core, dir) = isolated_core("options");
        assert!(unsafe {
            (*core).inner.update(|state| {
                state
                    .games
                    .insert("game.exe".into(), GameRecord::new("game.exe"));
                Ok(())
            })
        }
        .is_ok());
        let path = CString::new("game.exe").unwrap();
        assert_eq!(
            unsafe { sapie_core_set_favorite(core, path.as_ptr(), 1) },
            Status::Ok
        );

        let args = [
            CString::new("-windowed").unwrap(),
            CString::new("-nopause").unwrap(),
        ];
        let ptrs: [*const c_char; 2] = [args[0].as_ptr(), args[1].as_ptr()];
        let preload = CString::new("/lib/fake.so").unwrap();
        assert_eq!(
            unsafe {
                sapie_core_save_game_options(
                    core,
                    path.as_ptr(),
                    ptrs.as_ptr(),
                    ptrs.len(),
                    preload.as_ptr(),
                )
            },
            Status::Ok
        );
        let record = unsafe { (*core).inner.snapshot() }
            .unwrap()
            .games
            .get("game.exe")
            .cloned()
            .unwrap();
        assert!(record.favorite);
        assert_eq!(record.launch_args, ["-windowed", "-nopause"]);
        assert_eq!(record.ld_preload.as_deref(), Some("/lib/fake.so"));

        let mut out = ptr::null_mut();
        let channel = CString::new("stable").unwrap();
        assert_eq!(
            unsafe { sapie_core_check_updates(core, channel.as_ptr(), &mut out) },
            Status::Ok
        );
        assert_eq!(unsafe { as_str(out) }, Some(""));
        unsafe { free_string(out) };

        unsafe { sapie_core_destroy(core) };
        let _ = std::fs::remove_dir_all(&dir);
    }
}
