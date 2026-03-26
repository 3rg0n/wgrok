/**
 * Platform listener abstraction — normalized message receiving across transports.
 *
 * Each platform listener connects via its native mechanism (WebSocket, TCP, etc.)
 * and emits messages through a common callback interface.
 */

import { WebexMessageHandler, type Logger } from 'webex-message-handler';
import type { TLSSocket } from 'tls';
import * as tls from 'tls';

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type DecryptedMessage = any;
type SlackEnvelope = Record<string, unknown>;
type SlackPayload = Record<string, unknown>;
type SlackEvent = Record<string, unknown>;
type DiscordMessage = Record<string, unknown>;
type DiscordData = Record<string, unknown>;

export interface IncomingMessage {
  sender: string;
  text: string;
  html: string;
  msgId: string;
  platform: string;
  cards: unknown[];
}

export type MessageCallback = (msg: IncomingMessage) => Promise<void> | void;

export interface PlatformListener {
  onMessage(callback: MessageCallback): void;
  connect(): Promise<void>;
  disconnect(): Promise<void>;
}

/**
 * WebexListener wraps webex-message-handler and normalizes into IncomingMessage
 */
export class WebexListener implements PlatformListener {
  private token: string;
  private logger: Logger;
  private callback: MessageCallback | null = null;
  private handler: WebexMessageHandler | null = null;

  constructor(token: string, logger: Logger) {
    this.token = token;
    this.logger = logger;
  }

  onMessage(callback: MessageCallback): void {
    this.callback = callback;
  }

  async connect(): Promise<void> {
    this.handler = new WebexMessageHandler({
      token: this.token,
      logger: this.logger,
    });

    this.handler.on('message:created', (msg) => {
      void this.handleWebexMessage(msg);
    });

    await this.handler.connect();
  }

  private async handleWebexMessage(msg: DecryptedMessage): Promise<void> {
    if (this.callback) {
      const incoming: IncomingMessage = {
        sender: msg.personEmail as string,
        text: ((msg.text as string) || '').trim(),
        html: ((msg.html as string) || ''),
        msgId: msg.id as string,
        platform: 'webex',
        cards: [],
      };
      await this.callback(incoming);
    }
  }

  async disconnect(): Promise<void> {
    if (this.handler) {
      await this.handler.disconnect();
      this.handler = null;
    }
  }
}

const SLACK_SOCKET_MODE_URL = 'https://slack.com/api/apps.connections.open';

/**
 * SlackListener uses Socket Mode WebSocket
 * Requires an app-level token (xapp-*)
 */
export class SlackListener implements PlatformListener {
  private token: string;
  private logger: Logger;
  private callback: MessageCallback | null = null;
  private ws: WebSocket | null = null;
  private running = false;

  constructor(token: string, logger: Logger) {
    this.token = token;
    this.logger = logger;
  }

  onMessage(callback: MessageCallback): void {
    this.callback = callback;
  }

  async connect(): Promise<void> {
    // Request a WebSocket URL via apps.connections.open
    const response = await fetch(SLACK_SOCKET_MODE_URL, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${this.token}`,
      },
    });

    if (!response.ok) {
      throw new Error(`Slack apps.connections.open failed: HTTP ${response.status}`);
    }

    const data = (await response.json()) as Record<string, unknown>;
    if (!data.ok) {
      throw new Error(`Slack apps.connections.open failed: ${(data.error as string | undefined) || 'unknown'}`);
    }

    const wsUrl = data.url as string;
    this.ws = new WebSocket(wsUrl);
    this.running = true;

    this.ws.addEventListener('message', (event) => {
      void this.handleSlackEvent(event.data as string);
    });

    this.ws.addEventListener('close', () => {
      this.running = false;
    });

    this.ws.addEventListener('error', () => {
      this.running = false;
    });

    this.logger.info('Slack Socket Mode connected');

    // Wait for the WebSocket to be open before returning
    await new Promise<void>((resolve) => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        resolve();
      } else {
        const onOpen = () => {
          this.ws?.removeEventListener('open', onOpen);
          resolve();
        };
        this.ws?.addEventListener('open', onOpen);
      }
    });
  }

  private async handleSlackEvent(raw: string): Promise<void> {
    let envelope: SlackEnvelope;
    try {
      envelope = JSON.parse(raw) as SlackEnvelope;
    } catch {
      return;
    }

    // Acknowledge the envelope
    const envelopeId = envelope.envelope_id as string | undefined;
    if (envelopeId && this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ envelope_id: envelopeId }));
    }

    const eventType = envelope.type as string | undefined;
    if (eventType !== 'events_api') {
      return;
    }

    const payload = envelope.payload as SlackPayload | undefined;
    if (!payload) {
      return;
    }

    const event = payload.event as SlackEvent | undefined;
    if (!event || (event.type as string) !== 'message') {
      return;
    }

    // Skip bot messages to avoid loops
    if (event.bot_id) {
      return;
    }

    if (this.callback) {
      const incoming: IncomingMessage = {
        sender: (event.user as string) || '',
        text: (((event.text as string) || '').trim()),
        html: '',
        msgId: (event.ts as string) || '',
        platform: 'slack',
        cards: [],
      };
      await this.callback(incoming);
    }
  }

  async disconnect(): Promise<void> {
    this.running = false;
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.logger.info('Slack listener disconnected');
  }
}

const DISCORD_GATEWAY_API = 'https://discord.com/api/v10/gateway';

// Discord Gateway opcodes
const OP_DISPATCH = 0;
const OP_HEARTBEAT = 1;
const OP_IDENTIFY = 2;
const OP_HELLO = 10;

// Intents: GUILD_MESSAGES (1 << 9) + MESSAGE_CONTENT (1 << 15)
const INTENTS = (1 << 9) | (1 << 15);

/**
 * DiscordListener uses Gateway WebSocket
 */
export class DiscordListener implements PlatformListener {
  private token: string;
  private logger: Logger;
  private callback: MessageCallback | null = null;
  private ws: WebSocket | null = null;
  private running = false;
  private heartbeatInterval: NodeJS.Timeout | null = null;
  private sequence: number | null = null;

  constructor(token: string, logger: Logger) {
    this.token = token;
    this.logger = logger;
  }

  onMessage(callback: MessageCallback): void {
    this.callback = callback;
  }

  async connect(): Promise<void> {
    // Get gateway URL
    const response = await fetch(DISCORD_GATEWAY_API);
    if (!response.ok) {
      throw new Error(`Failed to get Discord gateway: HTTP ${response.status}`);
    }

    const data = (await response.json()) as Record<string, unknown>;
    const gwUrl = (data.url as string) || 'wss://gateway.discord.gg';
    const wsUrl = `${gwUrl}/?v=10&encoding=json`;

    this.ws = new WebSocket(wsUrl);
    this.running = true;

    this.ws.addEventListener('close', () => {
      this.running = false;
      if (this.heartbeatInterval) {
        clearInterval(this.heartbeatInterval);
        this.heartbeatInterval = null;
      }
    });

    this.ws.addEventListener('error', () => {
      this.running = false;
    });

    // Wait for Hello (opcode 10), then register the main message handler
    await new Promise<void>((resolve, reject) => {
      const onMessage = async (event: Event) => {
        if (event instanceof MessageEvent) {
          try {
            const helloMsg = JSON.parse(event.data as string) as Record<string, unknown>;
            if ((helloMsg.op as number) === OP_HELLO) {
              this.ws?.removeEventListener('message', onMessage);
              this.ws?.removeEventListener('error', onError);
              const heartbeatInterval = ((helloMsg.d as Record<string, unknown>).heartbeat_interval as number) / 1000;
              this.startHeartbeat(heartbeatInterval);
              resolve();
            }
          } catch {
            reject(new Error('Failed to parse Discord Hello message'));
          }
        }
      };

      const onError = () => {
        this.ws?.removeEventListener('message', onMessage);
        reject(new Error('WebSocket error during Hello'));
      };

      this.ws?.addEventListener('message', onMessage);
      this.ws?.addEventListener('error', onError);
    });

    // Register the main dispatch handler after Hello is complete
    this.ws?.addEventListener('message', (event) => {
      void this.handleDiscordMessage(event.data as string);
    });

    // Send Identify
    this.ws?.send(
      JSON.stringify({
        op: OP_IDENTIFY,
        d: {
          token: this.token,
          intents: INTENTS,
          properties: {
            os: 'linux',
            browser: 'wgrok',
            device: 'wgrok',
          },
        },
      })
    );

    this.logger.info('Discord Gateway connected');
  }

  private startHeartbeat(interval: number): void {
    this.heartbeatInterval = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ op: OP_HEARTBEAT, d: this.sequence }));
      }
    }, interval * 1000);
  }

  private async handleDiscordMessage(raw: string): Promise<void> {
    let data: DiscordMessage;
    try {
      data = JSON.parse(raw) as DiscordMessage;
    } catch {
      return;
    }

    // Track sequence number for heartbeat
    if ((data.s as number | null) !== null && data.s !== undefined) {
      this.sequence = data.s as number;
    }

    const op = data.op as number | undefined;
    if (op === OP_DISPATCH && (data.t as string) === 'MESSAGE_CREATE') {
      await this.handleDiscordMessageCreate(data.d as DiscordData);
    }
  }

  private async handleDiscordMessageCreate(event: DiscordData): Promise<void> {
    // Skip bot messages
    const author = event.author as Record<string, unknown> | undefined;
    if (!author || (author.bot as boolean | undefined)) {
      return;
    }

    if (this.callback) {
      const embeds = event.embeds as unknown[] | undefined;
      const incoming: IncomingMessage = {
        sender: (author.id as string) || '',
        text: (((event.content as string) || '').trim()),
        html: '',
        msgId: (event.id as string) || '',
        platform: 'discord',
        cards: embeds || [],
      };
      await this.callback(incoming);
    }
  }

  async disconnect(): Promise<void> {
    this.running = false;
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.logger.info('Discord listener disconnected');
  }
}

const PRIVMSG_RE = /^:([^!]+)![^ ]+ PRIVMSG ([^ ]+) :(.+)$/;

/**
 * IrcListener uses persistent TCP/TLS connection
 */
export class IrcListener implements PlatformListener {
  private connStr: string;
  private logger: Logger;
  private callback: MessageCallback | null = null;
  private socket: TLSSocket | null = null;
  private running = false;
  private buffer = '';

  constructor(connStr: string, logger: Logger) {
    this.connStr = connStr;
    this.logger = logger;
  }

  onMessage(callback: MessageCallback): void {
    this.callback = callback;
  }

  async connect(): Promise<void> {
    const params = this.parseConnectionString(this.connStr);

    // Create TLS connection
    this.socket = tls.connect(
      {
        host: params.server,
        port: params.port,
        rejectUnauthorized: false,
      },
      () => {
        void this.sendRaw(`NICK ${params.nick}`);
        if (params.password) {
          void this.sendRaw(`PASS ${params.password}`);
        }
        void this.sendRaw(`USER ${params.nick} 0 * :${params.nick}`);
        void this.waitForWelcome(params.nick, params.channel);
      }
    );

    this.socket.on('data', (data: Buffer | string) => {
      this.handleIrcData(data);
    });

    this.socket.on('close', () => {
      this.running = false;
    });

    this.socket.on('error', () => {
      this.running = false;
    });

    this.running = true;
    this.logger.info(`IRC connected to ${params.nick}@${params.server}`);
  }

  private parseConnectionString(connStr: string): {
    nick: string;
    password: string;
    server: string;
    port: number;
    channel: string;
  } {
    if (!connStr.includes('@')) {
      throw new Error(`Invalid IRC connection string (missing @): ${connStr}`);
    }

    const [creds, serverPart] = connStr.split('@', 2);

    let nick: string;
    let password: string;
    if (creds.includes(':')) {
      [nick, password] = creds.split(':', 2);
    } else {
      nick = creds;
      password = '';
    }

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

  private async sendRaw(line: string): Promise<void> {
    if (!this.socket) {
      throw new Error('Not connected to IRC server');
    }
    // Sanitize to prevent IRC protocol injection
    const sanitized = line.replace(/\r/g, '').replace(/\n/g, '');
    return new Promise((resolve, reject) => {
      this.socket!.write(`${sanitized}\r\n`, (err) => {
        if (err) reject(err);
        else resolve();
      });
    });
  }

  private async waitForWelcome(nick: string, channel: string): Promise<void> {
    // Welcome is async, just wait a bit then join the channel
    await new Promise((resolve) => setTimeout(resolve, 1000));
    if (channel && this.socket) {
      void this.sendRaw(`JOIN ${channel}`);
    }
  }

  private handleIrcData(data: Buffer | string): void {
    this.buffer += typeof data === 'string' ? data : data.toString('utf-8');

    let lines = this.buffer.split('\r\n');
    this.buffer = lines[lines.length - 1];
    lines = lines.slice(0, -1);

    for (const line of lines) {
      const decoded = line.trim();
      if (!decoded) {
        continue;
      }

      // Handle server PING
      if (decoded.startsWith('PING')) {
        const pongArg = decoded.substring(5).trim();
        void this.sendRaw(`PONG ${pongArg}`);
        continue;
      }

      // Parse PRIVMSG
      const match = PRIVMSG_RE.exec(decoded);
      if (match && this.callback) {
        const [, nick, , text] = match;
        void (async () => {
          const incoming: IncomingMessage = {
            sender: nick,
            text: text.trim(),
            html: '',
            msgId: '',
            platform: 'irc',
            cards: [],
          };
          await this.callback!(incoming);
        })();
      }
    }
  }

  async disconnect(): Promise<void> {
    this.running = false;
    if (this.socket) {
      this.socket.end();
      this.socket = null;
    }
    this.logger.info('IRC listener disconnected');
  }
}

export function createListener(platform: string, token: string, logger: Logger): PlatformListener {
  if (platform === 'webex') {
    return new WebexListener(token, logger);
  }
  if (platform === 'slack') {
    return new SlackListener(token, logger);
  }
  if (platform === 'discord') {
    return new DiscordListener(token, logger);
  }
  if (platform === 'irc') {
    return new IrcListener(token, logger);
  }
  throw new Error(`Unsupported platform: ${platform}`);
}
