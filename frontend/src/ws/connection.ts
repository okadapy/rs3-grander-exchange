import type { components } from '../api/schema';

export type ChatMessage = components['schemas']['ChatMessage'];
export type PriceSnapshot = components['schemas']['PriceSnapshot'];
export type WsCommand = components['schemas']['WsCommand'];

export type WsStatus = 'connecting' | 'open' | 'closed' | 'unauthorized';

export interface WebSocketLike {
  send(data: string): void;
  close(code?: number): void;
  onopen: (() => void) | null;
  onmessage: ((event: { data: string }) => void) | null;
  onclose: ((event: { code: number }) => void) | null;
  onerror: (() => void) | null;
}

export interface WsHandlers {
  onChat(message: ChatMessage): void;
  onPrice(snapshot: PriceSnapshot): void;
  onStatus(status: WsStatus): void;
  onServerError(message: string): void;
}

export interface WsConnectionOptions {
  url: string;
  token: string;
  handlers: WsHandlers;
  socketFactory?: (url: string) => WebSocketLike;
  reconnectDelays?: number[];
  scheduleReconnect?: (run: () => void, ms: number) => void;
}

export interface WsConnection {
  subscribe(itemIds: number[]): void;
  unsubscribe(itemIds: number[]): void;
  sendChat(body: string): void;
  close(): void;
}

// The gateway rejects a bad token by refusing the upgrade, which browsers
// report as an abnormal close rather than a status code. A finite delay
// list is what keeps that from becoming an endless retry loop.
const DEFAULT_DELAYS = [1_000, 2_000, 5_000, 10_000, 30_000];
const POLICY_VIOLATION = 1008;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

export function createWsConnection(opts: WsConnectionOptions): WsConnection {
  const {
    url,
    token,
    handlers,
    socketFactory = (target) => new WebSocket(target) as unknown as WebSocketLike,
    reconnectDelays = DEFAULT_DELAYS,
    scheduleReconnect = (run, ms) => { setTimeout(run, ms); },
  } = opts;

  const subscriptions = new Set<number>();
  let socket: WebSocketLike | null = null;
  // Holds only non-subscription commands (e.g. chat) issued before the
  // socket is open; flushed once in `onopen`, after the restored subscriptions.
  let outbox: WsCommand[] = [];
  let attempt = 0;
  let opened = false;
  let disposed = false;

  function transmit(command: WsCommand) {
    if (socket && opened) {
      socket.send(JSON.stringify(command));
    } else {
      outbox.push(command);
    }
  }

  // Subscribe/unsubscribe never go through `transmit`/`outbox`: the full
  // current `subscriptions` set is always replayed as one frame in `onopen`
  // (first connect or reconnect alike). Buffering the diff as well would
  // double-send it once the socket comes up, so before open this only
  // updates the set and lets `onopen` carry it.
  function sendSubscriptionDiff(command: WsCommand) {
    if (socket && opened) {
      socket.send(JSON.stringify(command));
    }
  }

  function handleFrame(raw: string) {
    let frame: unknown;
    try {
      frame = JSON.parse(raw);
    } catch {
      return;
    }
    if (!isRecord(frame) || !isRecord(frame.payload)) return;

    switch (frame.type) {
      case 'chat':
        handlers.onChat(frame.payload as ChatMessage);
        return;
      case 'price':
        handlers.onPrice(frame.payload as PriceSnapshot);
        return;
      case 'error': {
        const message = frame.payload.message;
        if (typeof message === 'string') handlers.onServerError(message);
        return;
      }
      default:
        // heartbeat and anything the server adds later need no action.
        return;
    }
  }

  function connect() {
    opened = false;
    handlers.onStatus('connecting');

    const next = socketFactory(`${url}?token=${encodeURIComponent(token)}`);
    socket = next;

    next.onopen = () => {
      opened = true;
      attempt = 0;
      handlers.onStatus('open');

      if (subscriptions.size > 0) {
        next.send(JSON.stringify({ action: 'subscribe', item_ids: [...subscriptions] }));
      }
      const queued = outbox;
      outbox = [];
      for (const command of queued) next.send(JSON.stringify(command));
    };

    next.onmessage = (event) => handleFrame(event.data);

    next.onclose = (event) => {
      opened = false;
      if (disposed) return;

      if (event.code === POLICY_VIOLATION) {
        handlers.onStatus('unauthorized');
        return;
      }
      const delay = reconnectDelays[attempt];
      if (delay === undefined) {
        handlers.onStatus('closed');
        return;
      }
      attempt += 1;
      scheduleReconnect(connect, delay);
    };

    next.onerror = () => { /* onclose always follows; the status moves there. */ };
  }

  connect();

  return {
    subscribe(itemIds) {
      const added = itemIds.filter((id) => !subscriptions.has(id));
      for (const id of added) subscriptions.add(id);
      if (added.length > 0) sendSubscriptionDiff({ action: 'subscribe', item_ids: added });
    },

    unsubscribe(itemIds) {
      const removed = itemIds.filter((id) => subscriptions.delete(id));
      if (removed.length > 0) sendSubscriptionDiff({ action: 'unsubscribe', item_ids: removed });
    },

    sendChat(body) {
      transmit({ action: 'chat', body });
    },

    close() {
      disposed = true;
      socket?.close(1000);
      socket = null;
    },
  };
}
