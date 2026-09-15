import { http, HttpResponse } from 'msw';
import { expect, it } from 'vitest';
import { server } from '../test/msw/server';
import { api } from './client';

it('reads the gateway health endpoint through the generated types', async () => {
  server.use(
    http.get('http://localhost:8080/health', () =>
      HttpResponse.json({ status: 'ok', service: 'gateway' }),
    ),
  );

  const { data, error } = await api.GET('/health');

  expect(error).toBeUndefined();
  expect(data?.status).toBe('ok');
  expect(data?.service).toBe('gateway');
});

it('translates a transport failure into the Russian "no connection" message', async () => {
  server.use(http.get('http://localhost:8080/health', () => HttpResponse.error()));

  await expect(api.GET('/health')).rejects.toThrow('Нет связи со шлюзом');
});
