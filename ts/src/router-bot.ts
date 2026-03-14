import { WebexMessageHandler, type DecryptedMessage, type Logger } from 'webex-message-handler';
import type { BotConfig } from './config.js';
import { Allowlist } from './allowlist.js';
import { getLogger } from './logging.js';
import { isEcho, parseEcho, formatResponse } from './protocol.js';
import { getMessage, extractCards } from './webex.js';
import { platformSendMessage, platformSendCard } from './platform.js';

export class WgrokRouterBot {
  private config: BotConfig;
  private allowlist: Allowlist;
  private logger: Logger;
  private handler?: WebexMessageHandler;
  private abortController?: AbortController;
  private routes: Record<string, string>;

  constructor(config: BotConfig) {
    this.config = config;
    this.allowlist = new Allowlist(config.domains);
    this.logger = getLogger(config.debug, 'wgrok.router_bot');
    this.routes = config.routes;
  }

  async run(): Promise<void> {
    this.abortController = new AbortController();
    this.handler = new WebexMessageHandler({
      token: this.config.webexToken,
      logger: this.logger,
    });

    this.handler.on('message:created', (msg: DecryptedMessage) => {
      this.onMessage(msg).catch((err) => {
        this.logger.error(`Error handling message: ${err}`);
      });
    });

    this.logger.info('Router bot starting');
    await this.handler.connect();
    this.logger.info('Router bot connected');

    await new Promise<void>((resolve) => {
      this.abortController!.signal.addEventListener('abort', () => resolve());
    });
  }

  async stop(): Promise<void> {
    this.abortController?.abort();
    if (this.handler) {
      await this.handler.disconnect();
      this.handler = undefined;
    }
    this.logger.info('Router bot stopped');
  }

  /** Resolve target address based on slug and routing rules */
  private resolveTarget(slug: string, sender: string): string {
    if (this.routes[slug]) {
      return this.routes[slug];
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
  async onMessageWithCards(msg: DecryptedMessage, cards: unknown[]): Promise<void> {
    const sender = msg.personEmail;
    const text = msg.text.trim();

    if (!this.allowlist.isAllowed(sender)) {
      this.logger.warn(`Rejected message from ${sender}: not in allowlist`);
      return;
    }

    if (!isEcho(text)) {
      this.logger.debug(`Ignoring non-echo message from ${sender}`);
      return;
    }

    let slug: string, payload: string;
    try {
      ({ slug, payload } = parseEcho(text));
    } catch {
      this.logger.error(`Failed to parse echo message`);
      return;
    }

    const target = this.resolveTarget(slug, sender);
    const response = formatResponse(slug, payload);
    const [platform, token] = this.getSendPlatformToken();

    if (cards.length > 0) {
      this.logger.info(`Relaying to ${target}: ${response} (with ${cards.length} card(s))`);
      await platformSendCard(platform, token, target, response, cards[0]);
    } else {
      this.logger.info(`Relaying to ${target}: ${response}`);
      await platformSendMessage(platform, token, target, response);
    }
  }

  private async onMessage(msg: DecryptedMessage): Promise<void> {
    const cards = await this.fetchCards(msg.id);
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
}
