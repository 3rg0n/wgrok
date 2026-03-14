import type { Logger } from 'webex-message-handler';
import type { SenderConfig } from './config.js';
import { getLogger } from './logging.js';
import { formatEcho } from './protocol.js';
import { sendCard, sendMessage } from './webex.js';

export class WgrokSender {
  private config: SenderConfig;
  private logger: Logger;

  constructor(config: SenderConfig) {
    this.config = config;
    this.logger = getLogger(config.debug, 'wgrok.sender');
  }

  async send(payload: string, card?: unknown): Promise<Record<string, unknown>> {
    const text = formatEcho(this.config.slug, payload);
    this.logger.info(`Sending to ${this.config.target}: ${text}`);
    if (card) {
      this.logger.info('Including adaptive card attachment');
      return sendCard(this.config.webexToken, this.config.target, text, card);
    }
    return sendMessage(this.config.webexToken, this.config.target, text);
  }
}
