import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type Player = components['schemas']['Player'];
export type PlayerSkill = components['schemas']['PlayerSkill'];
export type HiscoreMode = components['schemas']['HiscoreMode'];

export function usePlayer(name: string, mode: HiscoreMode) {
  return useQuery({
    queryKey: queryKeys.player(name, mode),
    enabled: name.trim().length > 0,
    staleTime: 5 * 60_000,
    queryFn: async (): Promise<Player> => {
      const { data, error, response } = await api.GET('/hiscore/{name}', {
        params: { path: { name }, query: { mode } },
      });
      if (response.status === 404) {
        throw new Error('Игрок не найден или хайскоры недоступны');
      }
      if (error || !data) throw failure(response, error, 'Не удалось прочитать хайскоры');
      return data;
    },
  });
}
