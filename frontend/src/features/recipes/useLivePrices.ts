import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { queryKeys } from '../../api/queryKeys';
import type { PriceSnapshot } from '../../api/queries/prices';
import { useWs } from '../../ws/WsProvider';

export function useLivePrices(itemIds: number[]) {
  const { subscribe, unsubscribe, onPrice } = useWs();
  const queryClient = useQueryClient();
  const [staleItemIds, setStaleItemIds] = useState<Set<number>>(new Set());

  const key = itemIds.join(',');

  useEffect(() => {
    if (itemIds.length === 0) return;
    const ids = key.split(',').map(Number);
    subscribe(ids);
    return () => unsubscribe(ids);
    // `key` is the stable identity of the id list; `itemIds` is a new array
    // on every render and would resubscribe endlessly.
  }, [key, subscribe, unsubscribe]);

  useEffect(() => {
    const ids = new Set(key.length > 0 ? key.split(',').map(Number) : []);

    return onPrice((snapshot: PriceSnapshot) => {
      if (!ids.has(snapshot.item_id)) return;

      queryClient.setQueryData<Map<number, PriceSnapshot>>(
        queryKeys.latestPrices([...ids]),
        (current) => {
          const next = new Map(current ?? []);
          next.set(snapshot.item_id, snapshot);
          return next;
        },
      );

      setStaleItemIds((current) => new Set(current).add(snapshot.item_id));
    });
  }, [key, onPrice, queryClient]);

  return {
    staleItemIds,
    clearStale: useCallback(() => setStaleItemIds(new Set()), []),
  };
}
