import { beforeEach, describe, expect, it } from 'vitest';
import { createWsConnection } from './connection';
import type { WebSocketLike, WsHandlers, WsStatus } from './connection';

class FakeSocket implements WebSocketLike {
  static instances: FakeSocket[] = [];

  sent: string[] = [];
  closedWith: number | undefined;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(readonly url: string) {
    FakeSocket.instances.push(this);
  }

  send(data: string) { this.sent.push(data); }
  close(code?: number) { this.closedWith = code; }

  open() { this.onopen?.(); }
  deliver(frame: unknown) { this.onmessage?.({ data: JSON.stringify(frame) }); }
  drop(code: number) { this.onclose?.({ code }); }

  get parsed(): unknown[] { return this.sent.map((raw) => JSON.parse(raw)); }
}

function harness(overrides: Partial<WsHandlers> = {}) {
  const chats: unknown[] = [];
  const prices: unknown[] = [];
  const statuses: WsStatus[] = [];
  const errors: string[] = [];
  const pending: (() => void)[] = [];

  const connection = createWsConnection({
    url: 'ws://localhost:8080/ws',
    token: 'jwt-abc',
    socketFactory: (url) => new FakeSocket(url),
    reconnectDelays: [10, 20],
    scheduleReconnect: (run) => { pending.push(run); },
    handlers: {
      onChat: (m) => chats.push(m),
      onPrice: (p) => prices.push(p),
      onStatus: (s) => statuses.push(s),
      onServerError: (m) => errors.push(m),
      ...overrides,
    },
  });

  return {
    connection,
    chats,
    prices,
    statuses,
    errors,
    runReconnect: () => pending.shift()?.(),
    socket: (index = 0) => FakeSocket.instances[index],
    socketCount: () => FakeSocket.instances.length,
  };
}

beforeEach(() => { FakeSocket.instances = []; });

describe('createWsConnection', () => {
  it('puts the token in the query string', () => {
    const h = harness();
    expect(h.socket().url).toBe('ws://localhost:8080/ws?token=jwt-abc');
  });

  it('buffers commands issued before the socket opens', () => {
    const h = harness();
    h.connection.subscribe([1603, 1605]);
    expect(h.socket().sent).toEqual([]);

    h.socket().open();

    expect(h.socket().parsed).toEqual([{ action: 'subscribe', item_ids: [1603, 1605] }]);
  });

  it('routes chat, price and error frames to their handlers', () => {
    const h = harness();
    h.socket().open();

    h.socket().deliver({ type: 'chat', payload: { id: 1, user_id: 2, username: 'okadishe', body: 'Hey', created_at: '2026-09-15T07:06:48.728Z' } });
    h.socket().deliver({ type: 'price', payload: { id: 9, item_id: 1603, ts: '2026-09-15T09:53:33.001Z', price: 307, volume: 171851 } });
    h.socket().deliver({ type: 'heartbeat', payload: { ts: '2026-09-15T09:54:00.000Z' } });
    h.socket().deliver({ type: 'error', payload: { message: 'unknown action' } });

    expect(h.chats).toHaveLength(1);
    expect(h.prices).toEqual([
      { id: 9, item_id: 1603, ts: '2026-09-15T09:53:33.001Z', price: 307, volume: 171851 },
    ]);
    expect(h.errors).toEqual(['unknown action']);
  });

  it('ignores malformed frames instead of throwing', () => {
    const h = harness();
    h.socket().open();

    h.socket().onmessage?.({ data: 'not json' });
    h.socket().deliver({ type: 'price' });

    expect(h.prices).toEqual([]);
    expect(h.errors).toEqual([]);
  });

  it('restores subscriptions after a reconnect without duplicating them', () => {
    const h = harness();
    h.socket().open();
    h.connection.subscribe([1603]);
    h.connection.subscribe([1605, 1603]);
    h.connection.unsubscribe([1603]);

    h.socket().drop(1006);
    h.runReconnect();
    h.socket(1).open();

    expect(h.socketCount()).toBe(2);
    expect(h.socket(1).parsed).toEqual([{ action: 'subscribe', item_ids: [1605] }]);
  });

  it('stops after the reconnect delays are exhausted', () => {
    const h = harness();
    h.socket().open();

    h.socket().drop(1006);
    h.runReconnect();
    h.socket(1).drop(1006);
    h.runReconnect();
    h.socket(2).drop(1006);

    expect(h.socketCount()).toBe(3);
    expect(h.statuses.at(-1)).toBe('closed');
  });

  it('treats a policy close as a rejected token and does not reconnect', () => {
    const h = harness();
    h.socket().open();

    h.socket().drop(1008);
    h.runReconnect();

    expect(h.socketCount()).toBe(1);
    expect(h.statuses.at(-1)).toBe('unauthorized');
  });

  it('does not reconnect after a deliberate close', () => {
    const h = harness();
    h.socket().open();

    h.connection.close();
    h.socket().drop(1000);
    h.runReconnect();

    expect(h.socketCount()).toBe(1);
  });

  it('sends chat messages as command frames', () => {
    const h = harness();
    h.socket().open();

    h.connection.sendChat('Привет');

    expect(h.socket().parsed).toEqual([{ action: 'chat', body: 'Привет' }]);
  });

  it('reports the open status once the socket connects', () => {
    const h = harness();
    expect(h.statuses).toEqual(['connecting']);
    h.socket().open();
    expect(h.statuses).toEqual(['connecting', 'open']);
  });
});
