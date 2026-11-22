use crate::error::{CommandError, ErrorKind, Result};
use crate::platform::Keychain;
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

const TIMEOUT: Duration = Duration::from_secs(8);
// The server issues 24 hour sessions. The expiry is metadata; the token stays in the keychain.
const SESSION_LIFETIME_SECS: u64 = 24 * 60 * 60;

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

fn validate_api_path(path: &str) -> Result<()> {
    if !path.starts_with('/')
        || path.len() < 2
        || path.contains("//")
        || path.contains("..")
        || path.contains('\\')
        || path.contains('?')
        || path.contains('#')
        || path.chars().any(char::is_whitespace)
    {
        return Err(CommandError::new(
            ErrorKind::InvalidArgument,
            "The server path is not valid.",
        ));
    }
    Ok(())
}

fn map_http_error(error: ureq::Error) -> CommandError {
    match error {
        ureq::Error::Status(400, _) => CommandError::new(
            ErrorKind::InvalidArgument,
            "The server rejected the request.",
        ),
        ureq::Error::Status(401, _) => CommandError::new(
            ErrorKind::TokenExpired,
            "The session has ended. Sign in again.",
        ),
        ureq::Error::Status(403, _) => CommandError::new(
            ErrorKind::PermissionDenied,
            "The server refused this account.",
        ),
        ureq::Error::Status(404, _) => CommandError::new(
            ErrorKind::NotFound,
            "The server could not find that account or request.",
        ),
        ureq::Error::Status(405, _) => CommandError::new(
            ErrorKind::ServerTooOld,
            "The server does not provide this route. Update the SAPIE server.",
        ),
        ureq::Error::Status(409, _) => CommandError::new(
            ErrorKind::InvalidArgument,
            "The server says that request already exists.",
        ),
        ureq::Error::Status(410, _) => {
            CommandError::new(ErrorKind::NotFound, "That invite link has expired.")
        }
        ureq::Error::Status(429, _) => CommandError::new(
            ErrorKind::ServerError,
            "The server is limiting requests. Try again later.",
        ),
        ureq::Error::Status(code, _) => CommandError::new(
            ErrorKind::ServerError,
            format!("The server returned HTTP {code}."),
        ),
        ureq::Error::Transport(error) => CommandError::new(
            ErrorKind::Unreachable,
            format!("Could not reach the SAPIE server: {error}"),
        ),
    }
}

fn response_or_error(
    result: std::result::Result<ureq::Response, ureq::Error>,
) -> Result<ureq::Response> {
    result.map_err(map_http_error)
}

// JSON numbers are doubles on the Qt side, which would lose precision on
// 64-bit Steam IDs. Send identifiers as strings across the ABI instead.
fn stringify_ids(value: &mut serde_json::Value) {
    match value {
        serde_json::Value::Object(map) => {
            for (key, child) in map.iter_mut() {
                if matches!(
                    key.as_str(),
                    "steam_id" | "from_steam_id" | "to_steam_id" | "lobby_id" | "owner" | "id"
                ) && child.is_number()
                {
                    let text = child.to_string();
                    *child = serde_json::Value::String(text);
                } else {
                    stringify_ids(child);
                }
            }
        }
        serde_json::Value::Array(items) => {
            for item in items {
                stringify_ids(item);
            }
        }
        _ => {}
    }
}

/// Sends a request to the authenticated user API. The token is read from the
/// keychain here and never crosses the C ABI. Callers supply only fixed paths.
pub fn server_request(
    server: &str,
    method: &str,
    path: &str,
    body: Option<&str>,
    store: &dyn Keychain,
) -> Result<String> {
    let server = validate_server(server)?;
    validate_api_path(path)?;
    let method = method.trim().to_ascii_uppercase();
    if !matches!(method.as_str(), "GET" | "POST" | "DELETE") {
        return Err(CommandError::new(
            ErrorKind::InvalidArgument,
            "The HTTP method is not supported.",
        ));
    }
    let token = store
        .get()?
        .filter(|token| !token.is_empty())
        .ok_or_else(|| {
            CommandError::new(
                ErrorKind::TokenExpired,
                "The session has ended. Sign in again.",
            )
        })?;
    let auth = format!("Bearer {token}");
    let url = format!("http://{server}/api/v1{path}");
    let request = ureq::request(&method, &url)
        .timeout(TIMEOUT)
        .set("Authorization", &auth);
    let response = match body {
        Some(body) => response_or_error(
            request
                .set("Content-Type", "application/json")
                .send_string(body),
        )?,
        None => response_or_error(request.call())?,
    };
    let body = response.into_string().map_err(|e| {
        CommandError::new(
            ErrorKind::ServerError,
            format!("The server returned an unreadable response: {e}"),
        )
    })?;
    if body.trim().is_empty() {
        return Ok("{}".to_string());
    }
    let mut value: serde_json::Value = serde_json::from_str(&body).map_err(|e| {
        CommandError::new(
            ErrorKind::ServerError,
            format!("The server returned invalid JSON: {e}"),
        )
    })?;
    stringify_ids(&mut value);
    serde_json::to_string(&value).map_err(|e| {
        CommandError::new(
            ErrorKind::ServerError,
            format!("The server response could not be encoded: {e}"),
        )
    })
}

/// Revokes the presented token, then clears the local copy even if the server
/// no longer recognises it.
pub fn revoke_token(server: Option<&str>, store: &dyn Keychain) -> Result<()> {
    let token = store.get()?;
    if let (Some(server), Some(token)) = (server, token.filter(|token| !token.is_empty())) {
        let server = validate_server(server)?;
        let url = format!("http://{server}/api/v1/logout");
        let auth = format!("Bearer {token}");
        response_or_error(
            ureq::post(&url)
                .timeout(TIMEOUT)
                .set("Authorization", &auth)
                .call(),
        )?;
    }
    store.clear()?;
    Ok(())
}

pub fn now_secs() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

pub fn is_expired(expires_at: Option<u64>) -> bool {
    match expires_at {
        Some(value) if value > now_secs() => false,
        Some(_) | None => true,
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
    store: &dyn Keychain,
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
    use crate::platform::{FakePlatform, Platform};
    use std::io::{Read, Write};
    use std::net::TcpListener;
    use std::sync::mpsc;
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
        let store = FakePlatform::default();
        let session = login(&server, "alice", "pw", &store).unwrap();
        handle.join().unwrap();
        assert_eq!(session.token, "tok-123");
        assert_eq!(session.steam_id, 76561197960265731);
        assert_eq!(session.persona_name, "Alice");
        assert_eq!(store.keychain().get().unwrap().as_deref(), Some("tok-123"));
        assert!(session.expires_at > now_secs());
    }

    #[test]
    fn login_reports_specific_auth_and_age_errors() {
        let (server, handle) = serve_once("401 Unauthorized", "unauthorized");
        let error = login(&server, "alice", "bad", &FakePlatform::default()).unwrap_err();
        handle.join().unwrap();
        assert_eq!(error.kind, ErrorKind::BadCredentials);
        assert!(!error.message.contains("bad"));

        let (server, handle) = serve_once("404 Not Found", "not found");
        let error = login(&server, "alice", "pw", &FakePlatform::default()).unwrap_err();
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
        let error = login(&server, "alice", "pw", &FakePlatform::default()).unwrap_err();
        handle.join().unwrap();
        assert_eq!(error.kind, ErrorKind::Unreachable);
    }

    #[test]
    fn expiry_needs_a_live_timestamp() {
        assert!(is_expired(None));
        assert!(is_expired(Some(now_secs() - 1)));
        assert!(!is_expired(Some(now_secs() + 3600)));
    }

    fn serve_with_capture(
        status: &str,
        body: &str,
    ) -> (String, mpsc::Receiver<String>, thread::JoinHandle<()>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let response = format!(
            "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
            body.len()
        );
        let (sender, receiver) = mpsc::channel();
        let handle = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            let mut request = [0u8; 8192];
            let size = stream.read(&mut request).unwrap();
            sender
                .send(String::from_utf8_lossy(&request[..size]).into_owned())
                .unwrap();
            stream.write_all(response.as_bytes()).unwrap();
        });
        (format!("127.0.0.1:{}", addr.port()), receiver, handle)
    }

    fn serve_empty_with_capture() -> (String, mpsc::Receiver<String>, thread::JoinHandle<()>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let (sender, receiver) = mpsc::channel();
        let handle = thread::spawn(move || {
            let (mut stream, _) = listener.accept().unwrap();
            let mut request = [0u8; 8192];
            let size = stream.read(&mut request).unwrap();
            sender
                .send(String::from_utf8_lossy(&request[..size]).into_owned())
                .unwrap();
            stream
                .write_all(
                    b"HTTP/1.1 204 No Content\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
                )
                .unwrap();
        });
        (format!("127.0.0.1:{}", addr.port()), receiver, handle)
    }

    #[test]
    fn authenticated_request_uses_the_keychain_and_a_fixed_path() {
        let (server, requests, handle) =
            serve_with_capture("200 OK", r#"{"friends":[],"incoming":[]}"#);
        let store = FakePlatform::default();
        store.keychain().set("tok-123").unwrap();
        let body = server_request(&server, "GET", "/friends", None, &store).unwrap();
        handle.join().unwrap();
        let request = requests.recv().unwrap();
        assert_eq!(body, r#"{"friends":[],"incoming":[]}"#);
        assert!(request.starts_with("GET /api/v1/friends HTTP/1.1"));
        assert!(request.contains("Authorization: Bearer tok-123"));
        assert!(!request.to_ascii_lowercase().contains("password"));
    }

    #[test]
    fn identifiers_cross_the_abi_as_strings() {
        let (server, _requests, handle) =
            serve_with_capture("200 OK", r#"{"friends":[{"steam_id":76561197960265731}]}"#);
        let store = FakePlatform::default();
        store.keychain().set("tok-123").unwrap();
        let body = server_request(&server, "GET", "/friends", None, &store).unwrap();
        handle.join().unwrap();
        assert!(body.contains(r#""steam_id":"76561197960265731""#));
    }

    #[test]
    fn authenticated_request_rejects_missing_token_and_unsafe_paths() {
        let store = FakePlatform::default();
        let error = server_request("127.0.0.1:8081", "GET", "/friends", None, &store).unwrap_err();
        assert_eq!(error.kind, ErrorKind::TokenExpired);
        store.keychain().set("tok-123").unwrap();
        for path in [
            "../logout",
            "/friends?all=1",
            "/friends/../logout",
            "friends",
        ] {
            let error = server_request("127.0.0.1:8081", "GET", path, None, &store).unwrap_err();
            assert_eq!(error.kind, ErrorKind::InvalidArgument);
        }
    }

    #[test]
    fn revoke_clears_the_token_after_logout() {
        let (server, requests, handle) = serve_empty_with_capture();
        let store = FakePlatform::default();
        store.keychain().set("tok-123").unwrap();
        revoke_token(Some(&server), &store).unwrap();
        handle.join().unwrap();
        let request = requests.recv().unwrap();
        assert!(request.starts_with("POST /api/v1/logout HTTP/1.1"));
        assert!(request.contains("Authorization: Bearer tok-123"));
        assert_eq!(store.keychain().get().unwrap(), None);
    }
}
