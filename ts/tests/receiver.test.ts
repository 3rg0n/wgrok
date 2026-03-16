import { loadCases } from './helpers';
import { WgrokReceiver } from '../src/receiver';
import type { IncomingMessage } from '../src/listener';

interface ReceiverCases {
  config: { slug: string; domains: string[] };
  cases: Array<{
    name: string;
    sender: string;
    text: string;
    cards: unknown[];
    expect_handler: boolean;
    expected_slug?: string;
    expected_payload?: string;
    expected_cards?: unknown[];
  }>;
}

const CASES = loadCases<ReceiverCases>('receiver_cases.json');

function fakeMsg(sender: string, text: string): IncomingMessage {
  return {
    sender,
    text,
    msgId: 'test-msg-id',
    platform: 'webex',
    cards: [],
  };
}

describe('WgrokReceiver', () => {
  it.each(CASES.cases)('$name', async (tc) => {
    let handlerCalled = false;
    let gotSlug = '';
    let gotPayload = '';
    let gotCards: unknown[] = [];

    const handler = (slug: string, payload: string, cards: unknown[]) => {
      handlerCalled = true;
      gotSlug = slug;
      gotPayload = payload;
      gotCards = cards;
    };

    const receiver = new WgrokReceiver(
      {
        webexToken: 'fake-token',
        slug: CASES.config.slug,
        domains: CASES.config.domains,
        debug: false,
        platform: 'webex',
      },
      handler,
    );

    const msg = fakeMsg(tc.sender, tc.text);
    await receiver.onMessageWithCards(msg, tc.cards);

    if (tc.expect_handler) {
      expect(handlerCalled).toBe(true);
      expect(gotSlug).toBe(tc.expected_slug);
      expect(gotPayload).toBe(tc.expected_payload);
      expect(gotCards).toEqual(tc.expected_cards);
    } else {
      expect(handlerCalled).toBe(false);
    }
  });
});
