import type { Logger } from 'webex-message-handler';
import type { ReceiverConfig } from './config.js';
import { decompress as codecDecompress, decrypt as codecDecrypt } from './codec.js';
import { Allowlist } from './allowlist.js';
import { getLogger } from './logging.js';
import { parseResponse, parseFlags, isPause, isResume, stripBotMention } from './protocol.js';
import { getMessage, getAttachmentAction, extractCards } from './webex.js';
import { createListener, type IncomingMessage, type PlatformListener } from './listener.js';

export type MessageHandler = (slug: string, payload: string, cards: unknown[], fromSlug: string) => void | Promise<void>;
export type ControlHandler = (cmd: string) => void | Promise<void>;

export class WgrokReceiver {
  private config: ReceiverConfig;
  private allowlist: Allowlist;
  private messageHandler: MessageHandler;
  private controlHandler?: ControlHandler;
  private logger: Logger;
  private listener?: PlatformListener;
  private abortController?: AbortController;
  private chunkBuffer: Map<string, Map<number, string>> = new Map();

  constructor(config: ReceiverConfig, handler: MessageHandler, controlHandler?: ControlHandler) {
    this.config = config;
    this.allowlist = new Allowlist(config.domains);
    this.messageHandler = handler;
    this.controlHandler = controlHandler;
    this.logger = getLogger(config.debug, 'wgrok.receiver');
  }

  async listen(): Promise<void> {
    this.abortController = new AbortController();
    this.listener = createListener(this.config.platform, this.config.webexToken, this.logger);

    this.listener.onMessage(async (msg: IncomingMessage) => {
      await this.onMessage(msg).catch((err) => {
        this.logger.error(`Error handling message: ${err}`);
      });
    });

    this.logger.info(`Receiver listening for slug: ${this.config.slug}`);
    await this.listener.connect();
    this.logger.info('Receiver connected');

    await new Promise<void>((resolve) => {
      this.abortController!.signal.addEventListener('abort', () => resolve());
    });
  }

  async stop(): Promise<void> {
    this.abortController?.abort();
    if (this.listener) {
      await this.listener.disconnect();
      this.listener = undefined;
    }
    this.logger.info('Receiver stopped');
  }

  async fetchAction(actionId: string): Promise<Record<string, unknown>> {
    return getAttachmentAction(this.config.webexToken, actionId);
  }

  /** Exposed for testing with injected cards */
  async onMessageWithCards(msg: IncomingMessage, cards: unknown[]): Promise<void> {
    const sender = msg.sender;
    const text = stripBotMention(msg.text, msg.html);

    if (!this.allowlist.isAllowed(sender)) {
      this.logger.warn(`Rejected message from ${sender}: not in allowlist`);
      return;
    }

    // Handle control commands before parsing as response
    if (isPause(text)) {
      this.logger.info(`Received pause command from ${sender}`);
      if (this.controlHandler) {
        await this.controlHandler('pause');
      }
      return;
    }

    if (isResume(text)) {
      this.logger.info(`Received resume command from ${sender}`);
      if (this.controlHandler) {
        await this.controlHandler('resume');
      }
      return;
    }

    let to: string, from: string, flags: string, payload: string;
    try {
      ({ to, from, flags, payload } = parseResponse(text));
    } catch {
      this.logger.debug(`Ignoring unparseable message from ${sender}`);
      return;
    }

    if (to !== this.config.slug) {
      this.logger.debug(`Ignoring message with to "${to}" (expected "${this.config.slug}")`);
      return;
    }

    let compressed = false;
    let encrypted = false;
    let chunkSeq: number | null = null;
    let chunkTotal: number | null = null;

    try {
      ({ compressed, encrypted, chunkSeq, chunkTotal } = parseFlags(flags));
    } catch {
      this.logger.debug(`Ignoring message with invalid flags "${flags}"`);
      return;
    }

    // Handle chunked payload
    if (chunkSeq !== null && chunkTotal !== null) {
      if (chunkTotal > 999 || chunkSeq > chunkTotal || chunkSeq < 1) {
        this.logger.warn(`Invalid chunk ${chunkSeq}/${chunkTotal} from ${sender}`);
        return;
      }
      const key = `${sender}:${to}`;
      if (!this.chunkBuffer.has(key)) {
        this.chunkBuffer.set(key, new Map());
      }
      this.chunkBuffer.get(key)!.set(chunkSeq, payload);
      if (this.chunkBuffer.get(key)!.size < chunkTotal) {
        this.logger.debug(`Buffered chunk ${chunkSeq}/${chunkTotal} for to "${to}" from ${sender}`);
        return;
      }
      // All chunks received — reassemble
      const parts: string[] = [];
      for (let i = 1; i <= chunkTotal; i++) {
        parts.push(this.chunkBuffer.get(key)!.get(i)!);
      }
      payload = parts.join('');
      this.chunkBuffer.delete(key);
      this.logger.debug(`Reassembled ${chunkTotal} chunks for to "${to}" from ${sender}`);
    }

    // Decrypt if encrypted
    if (encrypted) {
      if (!this.config.encryptKey) {
        this.logger.warn(`Received encrypted message but no WGROK_ENCRYPT_KEY is set`);
        return;
      }
      try {
        payload = codecDecrypt(payload, this.config.encryptKey);
      } catch (err) {
        this.logger.warn(`Failed to decrypt message: ${err}`);
        return;
      }
    }

    // Decompress if compressed
    if (compressed) {
      try {
        payload = codecDecompress(payload);
      } catch (err) {
        this.logger.warn(`Failed to decompress message: ${err}`);
        return;
      }
    }

    if (cards.length > 0) {
      this.logger.info(`Received payload for to "${to}" from ${sender} (with ${cards.length} card(s))`);
    } else {
      this.logger.info(`Received payload for to "${to}" from ${sender}`);
    }
    await this.messageHandler(to, payload, cards, from);
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
}
