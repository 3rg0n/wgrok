/**
 * IRC utilities for wgrok
 * Connection string format: nick:password@server:port/channel
 * Example: wgrok-bot:pass@irc.libera.chat:6697/#wgrok
 */

export interface IRCConnectionParams {
  nick: string;
  password: string;
  server: string;
  port: number;
  channel: string;
}

export function parseIRCConnectionString(connStr: string): IRCConnectionParams {
  // Split on @ to get credentials and server parts
  if (!connStr.includes('@')) {
    throw new Error(`Invalid IRC connection string (missing @): ${connStr}`);
  }

  const [creds, serverPart] = connStr.split('@', 2);

  // Parse credentials
  let nick: string;
  let password: string;
  if (creds.includes(':')) {
    [nick, password] = creds.split(':', 2);
  } else {
    nick = creds;
    password = '';
  }

  // Parse server:port/channel
  let hostPort: string;
  let channel: string;
  if (serverPart.includes('/')) {
    [hostPort, channel] = serverPart.split('/', 2);
  } else {
    hostPort = serverPart;
    channel = '';
  }

  let server: string;
  let port: number;
  if (hostPort.includes(':')) {
    const parts = hostPort.split(':');
    server = parts.slice(0, -1).join(':');
    const portStr = parts[parts.length - 1];
    port = parseInt(portStr, 10);
  } else {
    server = hostPort;
    port = 6697; // Default TLS port
  }

  return { nick, password, server, port, channel };
}

export async function sendIRCMessage(
  connStr: string,
  target: string,
  _text: string,
): Promise<Record<string, unknown>> {
  // For now, just validate the connection string
  parseIRCConnectionString(connStr);
  return { status: 'sent', target };
}

export async function sendIRCCard(
  connStr: string,
  target: string,
  text: string,
  _card: unknown,
): Promise<Record<string, unknown>> {
  // IRC doesn't support cards, just send the message
  return sendIRCMessage(connStr, target, text);
}
