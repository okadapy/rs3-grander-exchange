import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type ChatMessage = components['schemas']['ChatMessage'];

export function useChatHistory(opts?: { enabled?: boolean }) {
  return useQuery({
    queryKey: queryKeys.chatHistory(),
    enabled: opts?.enabled ?? true,
    queryFn: async (): Promise<ChatMessage[]> => {
      const { data, error, response } = await api.GET('/chat/history', {
        params: { query: { limit: 50 } },
      });
      if (error || !data) throw failure(response, error, 'Failed to load chat history');
      return data.messages;
    },
  });
}
