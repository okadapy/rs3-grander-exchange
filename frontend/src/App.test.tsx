import { http, HttpResponse } from 'msw';
import { screen } from '@testing-library/react';
import { beforeEach, expect, it } from 'vitest';
import App from './App';
import { server } from './test/msw/server';
import { renderWithProviders } from './test/renderWithProviders';

function healthHandler() {
  return http.get('http://localhost:8080/health/all', () =>
    HttpResponse.json({ gateway: 'ok', all_upstreams_ok: true, upstreams: [] }),
  );
}

// App always mounts ChatPopup (Task 8), which fetches chat history on mount
// regardless of whether the popup is expanded.
beforeEach(() => {
  server.use(http.get('http://localhost:8080/chat/history', () => HttpResponse.json({ messages: [] })));
});

it('renders the character column and the recipe area on one screen', async () => {
  server.use(healthHandler());
  renderWithProviders(<App />);

  expect(await screen.findByRole('heading', { name: 'Character' })).toBeInTheDocument();
  expect(screen.getByRole('heading', { name: 'Recipes' })).toBeInTheDocument();
  expect(screen.queryByRole('link', { name: 'Персонаж' })).not.toBeInTheDocument();
  expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
});

it('shows a green status dot when every upstream is healthy', async () => {
  server.use(healthHandler());
  renderWithProviders(<App />);

  expect(await screen.findByLabelText('All services operational')).toBeInTheDocument();
  expect(screen.queryByText(/Все сервисы/)).not.toBeInTheDocument();
});

it('shows an amber dot when some upstreams are down', async () => {
  server.use(
    http.get('http://localhost:8080/health/all', () =>
      HttpResponse.json({
        gateway: 'ok',
        all_upstreams_ok: false,
        upstreams: [{ target: 'http://recipe-service:8082', ok: false, status: 0, error: 'refused', ms: 1 }],
      }),
    ),
  );
  renderWithProviders(<App />);

  expect(
    await screen.findByLabelText(/Some services are not responding:.*recipe-service/),
  ).toBeInTheDocument();
});

it('shows a red dot when the gateway itself cannot be reached', async () => {
  server.use(http.get('http://localhost:8080/health/all', () => HttpResponse.error()));
  renderWithProviders(<App />);

  expect(await screen.findByLabelText('Services unavailable')).toBeInTheDocument();
});
