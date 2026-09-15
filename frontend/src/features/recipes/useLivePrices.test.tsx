import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';
import { queryKeys } from '../../api/queryKeys';
import { snapshot } from '../../test/fixtures';
import { useLivePrices } from './useLivePrices';

const subscribe = vi.fn();
const unsubscribe = vi.fn();
let emitPrice: ((s: ReturnType<typeof snapshot>) => void) | null = null;

vi.mock('../../ws/WsProvider', () => ({
  useWs: () => ({
    status: 'open',
    messages: [],
    serverError: null,
    sendChat: vi.fn(),
    subscribe,
    unsubscribe,
    onPrice: (listener: (s: ReturnType<typeof snapshot>) => void) => {
      emitPrice = listener;
      return () => { emitPrice = null; };
    },
  }),
}));

function makeWrapper(client: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
}

it('subscribes to the page item ids and unsubscribes on unmount', () => {
  const client = new QueryClient();
  const { unmount } = renderHook(() => useLivePrices([1603, 1605]), {
    wrapper: makeWrapper(client),
  });

  expect(subscribe).toHaveBeenCalledWith([1603, 1605]);
  unmount();
  expect(unsubscribe).toHaveBeenCalledWith([1603, 1605]);
});

it('patches the cached price and marks the row stale', async () => {
  const client = new QueryClient();
  client.setQueryData(
    queryKeys.latestPrices([1603]),
    new Map([[1603, snapshot({ price: 2707 })]]),
  );

  const { result } = renderHook(() => useLivePrices([1603]), { wrapper: makeWrapper(client) });

  act(() => { emitPrice?.(snapshot({ price: 3100 })); });

  await waitFor(() => expect(result.current.staleItemIds.has(1603)).toBe(true));
  const cached = client.getQueryData<Map<number, ReturnType<typeof snapshot>>>(
    queryKeys.latestPrices([1603]),
  );
  expect(cached?.get(1603)?.price).toBe(3100);
});

it('ignores a price frame for an item outside the current page', async () => {
  const client = new QueryClient();
  const { result } = renderHook(() => useLivePrices([1603]), { wrapper: makeWrapper(client) });

  act(() => { emitPrice?.(snapshot({ item_id: 9999, price: 10 })); });

  await waitFor(() => expect(result.current.staleItemIds.size).toBe(0));
});
