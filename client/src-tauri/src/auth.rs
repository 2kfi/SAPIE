use crate::error::{CommandError, ErrorKind, Result};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::sync::Arc;
#[cfg(test)]
use std::sync::Mutex;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const SERVICE: &str = "com.sapie.launcher";
const ACCOUNT: &str = "session-token";
const TIMEOUT: Duration = Duration::from_secs(8);
// The server issues 24 hour sessions. The expiry is metadata; the token stays in the keychain.
const SESSION_LIFETIME_SECS: u64 = 24 * 60 * 60;

pub trait TokenStore: Send + Sync {
    fn get(&self) -> Result<Option<String>>;
    fn set(&self, token: &str) -> Result<()>;
    fn clear(&self) -> Result<()>;
}

pub struct KeychainTokenStore;

impl TokenStore for KeychainTokenStore {
    fn get(&self) -> Result<Option<String>> {
        let entry = keyring::Entry::new(SERVICE, ACCOUNT).map_err(|e| {
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
        let entry = keyring::Entry::new(SERVICE, ACCOUNT).map_err(|e| {
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
        let entry = keyring::Entry::new(SERVICE, ACCOUNT).map_err(|e| {
            CommandError::new(
                ErrorKind::KeychainError,
                format!("Could not open the OS keychain: {e}"),
            )
        })?;
        match entry.delete_credential() {
            Ok(()) => Ok(()),
            Err(keyring::Error::NoEntry) => Ok(()),
            Err(e) => Err(CommandError::new(
                ErrorKind::KeychainError,
                format!("Could not remove the session token: {e}"),
            )),
        }
    }
}

#[cfg(test)]
#[derive(Default)]
pub struct MemoryTokenStore {
    token: Mutex<Option<String>>,
}

#[cfg(test)]
impl TokenStore for MemoryTokenStore {
    fn get(&self) -> Result<Option<String>> {
        Ok(self.token.lock().unwrap().clone())
    }

    fn set(&self, token: &str) -> Result<()> {
        *self.token.lock().unwrap() = Some(token.to_string());
        Ok(())
    }

    fn clear(&self) -> Result<()> {
        *self.token.lock().unwrap() = None;
        Ok(())
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct SessionInfo {
    pub token: String,
    pub steam_id: u64,
    pub persona_name: String,
    pub server: String,
    pub expires_at: u64,
}

#[derive(Deserialize)]
struct LoginPayload {
    session_token: String,
    steam_id: u64,
    persona_name: String,
}

pub fn validate_server(server: &str) -> Result<String> {
    let server = server.trim();
    let (host, port) = server.rsplit_once(':').ok_or_else(|| {
        CommandError::new(
            ErrorKind::InvalidServer,
            "Server must be host:port, for example 192.168.1.10:8081.",
        )
    })?;
    let host = host.trim().trim_matches(['[', ']']);
    if host.is_empty() || host.chars().any(char::is_whitespace) {
        return Err(CommandError::new(
            ErrorKind::InvalidServer,
            "Server host is empty or contains spaces.",
        ));
    }
    let port: u16 = port.parse().map_err(|_| {
        CommandError::new(
            ErrorKind::InvalidServer,
            format!("Invalid server port: {port}"),
        )
    })?;
    if port == 0 {
        return Err(CommandError::new(
            ErrorKind::InvalidServer,
            "Server port must be between 1 and 65535.",
        ));
    }
    let host = if host.contains(':') {
        format!("[{host}]")
    } else {
        host.to_string()
    };
    Ok(format!("{host}:{port}"))
}

pub(crate) fn now_secs() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

pub fn is_expired(expires_at: Option<u64>) -> bool {
    match expires_at {
        Some(value) if value > now_secs() => false,
        Some(_) => true,
        None => true,
    }
}

pub fn test_connection(server: &str) -> Result<()> {
    let server = validate_server(server)?;
    let url = format!("http://{server}/api/v1/config");
    match ureq::get(&url).timeout(TIMEOUT).call() {
        Ok(_) => Ok(()),
        Err(ureq::Error::Status(404 | 405, _)) => Err(CommandError::new(
            ErrorKind::ServerTooOld,
            "The server does not provide the 0.4 API. Update the SAPIE server.",
        )),
        Err(ureq::Error::Status(code, _)) => Err(CommandError::new(
            ErrorKind::ServerError,
            format!("The server returned HTTP {code} for the connection test."),
        )),
        Err(e) => Err(CommandError::new(
            ErrorKind::Unreachable,
            format!("Could not reach the SAPIE server at {server}: {e}"),
        )),
    }
}

pub fn login(
    server: &str,
    username: &str,
    password: &str,
    store: &Arc<dyn TokenStore>,
) -> Result<SessionInfo> {
    let server = validate_server(server)?;
    if username.trim().is_empty() || password.is_empty() {
        return Err(CommandError::new(
            ErrorKind::BadCredentials,
            "Username and password are required.",
        ));
    }
    let url = format!("http://{server}/api/v1/login");
    let response = match ureq::post(&url)
        .timeout(TIMEOUT)
        .set("Content-Type", "application/json")
        .send_json(json!({ "username": username, "password": password }))
    {
        Ok(response) => response,
        Err(ureq::Error::Status(401 | 403, _)) => {
            return Err(CommandError::new(
                ErrorKind::BadCredentials,
                "Wrong username or password.",
            ))
        }
        Err(ureq::Error::Status(404 | 405, _)) => {
            return Err(CommandError::new(
                ErrorKind::ServerTooOld,
                "The server does not provide the 0.4 login API. Update the SAPIE server.",
            ))
        }
        Err(ureq::Error::Status(code, _)) => {
            return Err(CommandError::new(
                ErrorKind::ServerError,
                format!("The server returned HTTP {code} for the login request."),
            ))
        }
        Err(e) => {
            return Err(CommandError::new(
                ErrorKind::Unreachable,
                format!("Could not reach the SAPIE server at {server}: {e}"),
            ))
        }
    };
    let payload: LoginPayload = response.into_json().map_err(|e| {
        CommandError::new(
            ErrorKind::ServerError,
            format!("The server returned an unreadable login response: {e}"),
        )
    })?;
    if payload.session_token.is_empty() || payload.steam_id == 0 {
        return Err(CommandError::new(
            ErrorKind::ServerError,
            "The server returned an incomplete login response.",
        ));
    }
    store.set(&payload.session_token)?;
    Ok(SessionInfo {
        token: payload.session_token,
        steam_id: payload.steam_id,
        persona_name: payload.persona_name,
        server,
        expires_at: now_secs() + SESSION_LIFETIME_SECS,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Read, Write};
    use std::net::TcpListener;
    use std::thread;

    fn serve_once(status: &str, body: &str) -> (String, thread::JoinHandle<()>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let response = format!(
            "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
            body.len()
        );
        let handle = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            let mut request = [0u8; 4096];
            let _ = stream.read(&mut request);
            stream.write_all(response.as_bytes()).unwrap();
        });
        (format!("127.0.0.1:{}", addr.port()), handle)
    }

    fn store() -> Arc<dyn TokenStore> {
        Arc::new(MemoryTokenStore::default())
    }

    #[test]
    fn server_addresses_are_validated() {
        assert_eq!(
            validate_server(" 127.0.0.1:8081 ").unwrap(),
            "127.0.0.1:8081"
        );
        assert_eq!(validate_server("[::1]:8081").unwrap(), "[::1]:8081");
        assert_eq!(
            validate_server("host:0").unwrap_err().kind,
            ErrorKind::InvalidServer
        );
        assert_eq!(
            validate_server("host").unwrap_err().kind,
            ErrorKind::InvalidServer
        );
        assert_eq!(
            validate_server("ho st:80").unwrap_err().kind,
            ErrorKind::InvalidServer
        );
    }

    #[test]
    fn login_stores_token_in_the_injected_store_only() {
        let (server, handle) = serve_once(
            "200 OK",
            r#"{"session_token":"tok-123","steam_id":76561197960265731,"persona_name":"Alice"}"#,
        );
        let store = store();
        let session = login(&server, "alice", "pw", &store).unwrap();
        handle.join().unwrap();
        assert_eq!(session.token, "tok-123");
        assert_eq!(session.steam_id, 76561197960265731);
        assert_eq!(session.persona_name, "Alice");
        assert_eq!(store.get().unwrap().as_deref(), Some("tok-123"));
        assert!(session.expires_at > now_secs());
    }

    #[test]
    fn login_reports_specific_auth_and_age_errors() {
        let (server, handle) = serve_once("401 Unauthorized", "unauthorized");
        let error = login(&server, "alice", "bad", &store()).unwrap_err();
        handle.join().unwrap();
        assert_eq!(error.kind, ErrorKind::BadCredentials);
        assert!(!error.message.contains("bad"));

        let (server, handle) = serve_once("404 Not Found", "not found");
        let error = login(&server, "alice", "pw", &store()).unwrap_err();
        handle.join().unwrap();
        assert_eq!(error.kind, ErrorKind::ServerTooOld);
    }

    #[test]
    fn unreachable_server_is_specific() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let server = format!("127.0.0.1:{}", listener.local_addr().unwrap().port());
        let handle = thread::spawn(move || {
            let (_stream, _) = listener.accept().unwrap();
        });
        let error = login(&server, "alice", "pw", &store()).unwrap_err();
        handle.join().unwrap();
        assert_eq!(error.kind, ErrorKind::Unreachable);
    }

    #[test]
    fn clearing_the_store_removes_the_token() {
        let store = store();
        store.set("tok").unwrap();
        store.clear().unwrap();
        assert_eq!(store.get().unwrap(), None);
    }
}
