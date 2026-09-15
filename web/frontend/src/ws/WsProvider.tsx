import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { API_URL } from '../api/client';
import { useAuth } from '../auth/AuthProvider';
import { createWsConnection } from './connection';
import type { ChatMessage, PriceSnapshot, WebSocketLike, WsConnection, WsStatus } from './connection';

interface WsContextValue {
  status: WsStatus;
  messages: ChatMessage[];
  serverError: string | null;
  sendChat(body: string): void;
  subscribe(itemIds: number[]): void;
  unsubscribe(itemIds: number[]): void;
  onPrice(listener: (snapshot: PriceSnapshot) => void): () => void;
}

const WsContext = createContext<WsContextValue | null>(null);

interface Props {
  children: ReactNode;
  socketFactory?: (url: string) => WebSocketLike;
}

function socketUrl(): string {
  return `${API_URL.replace(/^http/, 'ws')}/ws`;
}

export function WsProvider({ children, socketFactory }: Props) {
  const { token } = useAuth();
  const [status, setStatus] = useState<WsStatus>('closed');
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [serverError, setServerError] = useState<string | null>(null);

  const connectionRef = useRef<WsConnection | null>(null);
  const priceListeners = useRef(new Set<(snapshot: PriceSnapshot) => void>());

  useEffect(() => {
    if (!token) {
      setStatus('closed');
      return;
    }

    const connection = createWsConnection({
      url: socketUrl(),
      token,
      socketFactory,
      handlers: {
        onChat: (message) =>
          setMessages((current) =>
            current.some((existing) => existing.id === message.id) ? current : [...current, message],
          ),
        onPrice: (snapshot) => {
          for (const listener of priceListeners.current) listener(snapshot);
        },
        onStatus: setStatus,
        onServerError: setServerError,
      },
    });

    connectionRef.current = connection;
    return () => {
      connection.close();
      connectionRef.current = null;
    };
  }, [token, socketFactory]);

  // Stable across renders (deps are empty; both read from refs), so that
  // useLivePrices' effects - which depend on these three functions - do
  // not re-run, and resubscribe over the wire, on every chat message.
  const subscribe = useCallback((itemIds: number[]) => {
    connectionRef.current?.subscribe(itemIds);
  }, []);

  const unsubscribe = useCallback((itemIds: number[]) => {
    connectionRef.current?.unsubscribe(itemIds);
  }, []);

  const onPrice = useCallback((listener: (snapshot: PriceSnapshot) => void) => {
    priceListeners.current.add(listener);
    return () => {
      priceListeners.current.delete(listener);
    };
  }, []);

  const value = useMemo<WsContextValue>(
    () => ({
      status,
      messages,
      serverError,
      sendChat: (body) => connectionRef.current?.sendChat(body),
      subscribe,
      unsubscribe,
      onPrice,
    }),
    [status, messages, serverError, subscribe, unsubscribe, onPrice],
  );

  return <WsContext.Provider value={value}>{children}</WsContext.Provider>;
}

export function useWs(): WsContextValue {
  const value = useContext(WsContext);
  if (!value) throw new Error('useWs used outside WsProvider');
  return value;
}

export type { PriceSnapshot };
