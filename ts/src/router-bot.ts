import type { Logger } from 'webex-message-handler';
import type { BotConfig } from './config.js';
import { Allowlist } from './allowlist.js';
import { getLogger } from './logging.js';
import { isEcho, parseEcho, formatResponse, isPause, isResume } from './protocol.js';
import { getMessage, extractCards } from './webex.js';
import { platformSendMessage, platformSendCard } from './platform.js';
import { createListener, type IncomingMessage, type PlatformListener } from './listener.js';

interface BufferedMessage {
  response: string;
  target: string;
  cards: unknown[];
}

export class WgrokRouterBot {
  private config: BotConfig;
  private allowlist: Allowlist;
  private logger: Logger;
  private listeners: PlatformListener[] = [];
  private abortController?: AbortController;
  private routes: Record<string, string>;
  private pausedTargets: Set<string> = new Set();
  private pauseBuffer: Map<string, BufferedMessage[]> = new Map();

  constructor(config: BotConfig) {
    this.config = config;
    this.allowlist = new Allowlist(config.domains);
    this.logger = getLogger(config.debug, 'wgrok.router_bot');
    this.routes = config.routes;
  }

  async run(): Promise<void> {
    this.abortController = new AbortController();

    // Create listeners for all configured platforms
    const platformTokens = this.config.platformTokens;
    for (const [platform, tokens] of Object.entries(platformTokens)) {
      for (const token of tokens) {
        const listener = createListener(platform, token, this.logger);
        listener.onMessage(async (msg: IncomingMessage) => {
          await this.onMessage(msg).catch((err) => {
            this.logger.error(`Error handling message: ${err}`);
          });
        });
        this.listeners.push(listener);
      }
    }

    this.logger.info('Router bot starting');
    await Promise.all(this.listeners.map((l) => l.connect()));
    this.logger.info('Router bot connected');

    await new Promise<void>((resolve) => {
      this.abortController!.signal.addEventListener('abort', () => resolve());
    });
  }

  async stop(): Promise<void> {
    this.abortController?.abort();
    await Promise.all(this.listeners.map((l) => l.disconnect()));
    this.listeners = [];
    this.logger.info('Router bot stopped');
  }

  /** Resolve target address based on to and routing rules */
  private resolveTarget(to: string, sender: string): string {
    if (this.routes[to]) {
      return this.routes[to];
    }
    return sender;
  }

  /** Get the send platform and token, preferring webex if available */
  private getSendPlatformToken(): [string, string] {
    // Prefer webex if available
    if (this.config.platformTokens.webex && this.config.platformTokens.webex.length > 0) {
      return ['webex', this.config.platformTokens.webex[0]];
    }

    // Otherwise get the first available platform
    for (const [platform, tokens] of Object.entries(this.config.platformTokens)) {
      if (tokens && tokens.length > 0) {
        return [platform, tokens[0]];
      }
    }

    // Fallback to webex token if no platform tokens are configured
    return ['webex', this.config.webexToken];
  }

  /** Exposed for testing with injected cards */
  async onMessageWithCards(msg: IncomingMessage, cards: unknown[]): Promise<void> {
    const sender = msg.sender;
    const text = msg.text;

    if (!this.allowlist.isAllowed(sender)) {
      this.logger.warn(`Rejected message from ${sender}: not in allowlist`);
      return;
    }

    // Handle pause/resume commands before checking for echo
    if (isPause(text)) {
      this.logger.info(`Received pause command from ${sender}`);
      this.pausedTargets.add(sender);
      if (!this.pauseBuffer.has(sender)) {
        this.pauseBuffer.set(sender, []);
      }
      return;
    }

    if (isResume(text)) {
      this.logger.info(`Received resume command from ${sender}`);
      await this.flushBuffer(sender);
      this.pausedTargets.delete(sender);
      return;
    }

    if (!isEcho(text)) {
      this.logger.debug(`Ignoring non-echo message from ${sender}`);
      return;
    }

    let to: string, from: string, flags: string, payload: string;
    try {
      ({ to, from, flags, payload } = parseEcho(text));
    } catch {
      this.logger.error(`Failed to parse echo message`);
      return;
    }

    const target = this.resolveTarget(to, sender);
    const response = formatResponse(to, from, flags, payload);
    const [platform, token] = this.getSendPlatformToken();

    // Check if target is paused — if so, buffer instead of sending
    if (this.pausedTargets.has(target)) {
      this.logger.info(`Target ${target} is paused, buffering message`);
      if (!this.pauseBuffer.has(target)) {
        this.pauseBuffer.set(target, []);
      }
      const buf = this.pauseBuffer.get(target)!;
      if (buf.length >= 1000) {
        this.logger.warn(`Pause buffer full for ${target}, dropping oldest message`);
        buf.shift();
      }
      buf.push({ response, target, cards });
      return;
    }

    if (cards.length > 0) {
      this.logger.info(`Relaying to ${target}: ${response} (with ${cards.length} card(s))`);
      await platformSendCard(platform, token, target, response, cards[0]);
    } else {
      this.logger.info(`Relaying to ${target}: ${response}`);
      await platformSendMessage(platform, token, target, response);
    }
  }

  private async onMessage(msg: IncomingMessage): Promise<void> {
    // For webex, fetch additional card data. For other platforms, cards come with the message
    let cards = msg.cards;
    if (msg.platform === 'webex' && msg.msgId) {
      cards = await this.fetchCards(msg.msgId);
    }
    await this.onMessageWithCards(msg, cards);
  }

  private async fetchCards(messageId: string): Promise<unknown[]> {
    if (!messageId) return [];
    try {
      const fullMsg = await getMessage(this.config.webexToken, messageId);
      return extractCards(fullMsg);
    } catch (err) {
      this.logger.debug(`Could not fetch message attachments: ${err}`);
      return [];
    }
  }

  async pause(): Promise<void> {
    const [platform, token] = this.getSendPlatformToken();
    for (const target of Object.values(this.routes)) {
      this.logger.info(`Sending pause command to Mode C route target: ${target}`);
      this.pausedTargets.add(target);
      if (!this.pauseBuffer.has(target)) {
        this.pauseBuffer.set(target, []);
      }
      await platformSendMessage(platform, token, target, './pause');
    }
  }

  async resume(): Promise<void> {
    const [platform, token] = this.getSendPlatformToken();
    for (const target of Object.values(this.routes)) {
      this.logger.info(`Sending resume command to Mode C route target: ${target}`);
      await platformSendMessage(platform, token, target, './resume');
      await this.flushBuffer(target);
      this.pausedTargets.delete(target);
    }
  }

  private async flushBuffer(target: string): Promise<void> {
    const buffered = this.pauseBuffer.get(target);
    if (!buffered || buffered.length === 0) {
      return;
    }

    const [platform, token] = this.getSendPlatformToken();
    this.logger.info(`Flushing ${buffered.length} buffered messages for ${target}`);

    for (const msg of buffered) {
      if (msg.cards.length > 0) {
        await platformSendCard(platform, token, msg.target, msg.response, msg.cards[0]);
      } else {
        await platformSendMessage(platform, token, msg.target, msg.response);
      }
    }

    this.pauseBuffer.delete(target);
  }
}
