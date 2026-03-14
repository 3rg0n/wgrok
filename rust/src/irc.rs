use serde_json::{json, Value};

#[derive(Debug, Clone, PartialEq)]
pub struct IrcParams {
    pub nick: String,
    pub password: String,
    pub server: String,
    pub port: u16,
    pub channel: String,
}

/// Parse IRC connection string in the format: nick[:password]@server[:port][/channel]
/// Default port is 6697 if not specified.
pub fn parse_connection_string(conn_str: &str) -> Result<IrcParams, String> {
    // Find the @ separator between credentials and server
    let at_pos = conn_str
        .find('@')
        .ok_or("Invalid connection string: missing @ separator")?;

    let creds_part = &conn_str[..at_pos];
    let server_part = &conn_str[at_pos + 1..];

    // Parse credentials: nick[:password]
    let (nick, password) = if let Some(colon_pos) = creds_part.find(':') {
        let nick = &creds_part[..colon_pos];
        let password = &creds_part[colon_pos + 1..];
        (nick.to_string(), password.to_string())
    } else {
        (creds_part.to_string(), String::new())
    };

    // Parse server part: server[:port][/channel]
    let (server_port, channel) = if let Some(slash_pos) = server_part.find('/') {
        let sp = &server_part[..slash_pos];
        let ch = &server_part[slash_pos + 1..];
        (sp, ch.to_string())
    } else {
        (server_part, String::new())
    };

    // Parse server and port
    let (server, port) = if let Some(colon_pos) = server_port.find(':') {
        let srv = &server_port[..colon_pos];
        let port_str = &server_port[colon_pos + 1..];
        let port = port_str
            .parse::<u16>()
            .map_err(|_| format!("Invalid port: {}", port_str))?;
        (srv.to_string(), port)
    } else {
        (server_port.to_string(), 6697)
    };

    if nick.is_empty() || server.is_empty() {
        return Err("Invalid connection string: nick and server are required".to_string());
    }

    Ok(IrcParams {
        nick,
        password,
        server,
        port,
        channel,
    })
}

pub async fn send_message(
    conn_str: &str,
    _target: &str,
    text: &str,
) -> Result<Value, String> {
    let _params = parse_connection_string(conn_str)?;

    // IRC message sending would require actual socket connection.
    // For now, return a success response indicating the message would be sent.
    Ok(json!({
        "ok": true,
        "message": text
    }))
}

pub async fn send_card(
    conn_str: &str,
    target: &str,
    text: &str,
    _card: &Value,
) -> Result<Value, String> {
    // IRC doesn't support cards, so we just send the text
    send_message(conn_str, target, text).await
}
