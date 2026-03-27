import {
  createListener,
  type IncomingMessage,
  WebexListener,
  SlackListener,
  DiscordListener,
  IrcListener,
} from '../src/listener';
import { noopLogger } from '../src/logging';

describe('listener factory', () => {
  it('creates WebexListener for webex platform', () => {
    const listener = createListener('webex', 'test-token', noopLogger);
    expect(listener).toBeInstanceOf(WebexListener);
  });

  it('creates SlackListener for slack platform', () => {
    const listener = createListener('slack', 'xapp-test-token', noopLogger);
    expect(listener).toBeInstanceOf(SlackListener);
  });

  it('creates DiscordListener for discord platform', () => {
    const listener = createListener('discord', 'test-token', noopLogger);
    expect(listener).toBeInstanceOf(DiscordListener);
  });

  it('creates IrcListener for irc platform', () => {
    const listener = createListener('irc', 'nick:pass@server:6697/#channel', noopLogger);
    expect(listener).toBeInstanceOf(IrcListener);
  });

  it('throws for unsupported platform', () => {
    expect(() => createListener('unknown', 'token', noopLogger)).toThrow('Unsupported platform: unknown');
  });
});

describe('IncomingMessage', () => {
  it('has required properties', () => {
    const msg: IncomingMessage = {
      sender: 'user@example.com',
      text: 'Hello world',
      html: '',
      roomId: '',
      roomType: '',
      msgId: 'msg-123',
      platform: 'webex',
      cards: [],
    };

    expect(msg.sender).toBe('user@example.com');
    expect(msg.text).toBe('Hello world');
    expect(msg.msgId).toBe('msg-123');
    expect(msg.platform).toBe('webex');
    expect(msg.cards).toEqual([]);
  });

  it('can hold card data', () => {
    const card = { type: 'AdaptiveCard', body: [] };
    const msg: IncomingMessage = {
      sender: 'user@example.com',
      text: 'Message with card',
      html: '',
      roomId: '',
      roomType: '',
      msgId: 'msg-456',
      platform: 'webex',
      cards: [card],
    };

    expect(msg.cards).toHaveLength(1);
    expect(msg.cards[0]).toBe(card);
  });
});
