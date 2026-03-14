import { WebexMessageHandler, type DecryptedMessage, type Logger } from 'webex-message-handler';
import type { ReceiverConfig } from './config.js';
import { Allowlist } from './allowlist.js';
import { getLogger } from './logging.js';
import { parseResponse } from './protocol.js';
import { getMessage, getAttachmentAction, extractCards } from './webex.js';

export type MessageHandler = (slug: string, payload: string, cards: unknown[]) => void | Promise<void>;

export class WgrokReceiver {
  private config: ReceiverConfig;
  private allowlist: Allowlist;
  private messageHandler: MessageHandler;
  private logger: Logger;
  private wsHandler?: WebexMessageHandler;
  private abortController?: AbortController;

  constructor(config: ReceiverConfig, handler: MessageHandler) {
    this.config = config;
    this.allowlist = new Allowlist(config.domains);
    this.messageHandler = handler;
    this.logger = getLogger(config.debug, 'wgrok.receiver');
  }

  async listen(): Promise<void> {
    this.abortController = new AbortController();
    this.wsHandler = new WebexMessageHandler({
      token: this.config.webexToken,
      logger: this.logger,
    });

    this.wsHandler.on('message:created', (msg: DecryptedMessage) => {
      this.onMessage(msg).catch((err) => {
        this.logger.error(`Error handling message: ${err}`);
      });
    });

    this.logger.info(`Receiver listening for slug: ${this.config.slug}`);
    await this.wsHandler.connect();
    this.logger.info('Receiver connected');

    await new Promise<void>((resolve) => {
      this.abortController!.signal.addEventListener('abort', () => resolve());
    });
  }

  async stop(): Promise<void> {
    this.abortController?.abort();
    if (this.wsHandler) {
      await this.wsHandler.disconnect();
      this.wsHandler = undefined;
    }
    this.logger.info('Receiver stopped');
  }

  async fetchAction(actionId: string): Promise<Record<string, unknown>> {
    return getAttachmentAction(this.config.webexToken, actionId);
  }

  /** Exposed for testing with injected cards */
  async onMessageWithCards(msg: DecryptedMessage, cards: unknown[]): Promise<void> {
    const sender = msg.personEmail;
    const text = msg.text.trim();

    if (!this.allowlist.isAllowed(sender)) {
      this.logger.warn(`Rejected message from ${sender}: not in allowlist`);
      return;
    }

    let slug: string, payload: string;
    try {
      ({ slug, payload } = parseResponse(text));
    } catch {
      this.logger.debug(`Ignoring unparseable message from ${sender}`);
      return;
    }

    if (slug !== this.config.slug) {
      this.logger.debug(`Ignoring message with slug "${slug}" (expected "${this.config.slug}")`);
      return;
    }

    if (cards.length > 0) {
      this.logger.info(`Received payload for slug "${slug}" from ${sender} (with ${cards.length} card(s))`);
    } else {
      this.logger.info(`Received payload for slug "${slug}" from ${sender}`);
    }
    await this.messageHandler(slug, payload, cards);
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
