import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type GatewayHealth = components['schemas']['GatewayHealth'];

export function useGatewayHealth() {
  return useQuery({
    queryKey: queryKeys.health(),
    refetchInterval: 60_000,
    queryFn: async (): Promise<GatewayHealth> => {
      const { data, error, response } = await api.GET('/health/all');
      if (error || !data) throw failure(response, error, 'Шлюз недоступен');
      return data;
    },
  });
}
