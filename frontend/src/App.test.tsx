import { http, HttpResponse } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it } from 'vitest';
import App from './App';
import { server } from './test/msw/server';
import { renderWithProviders } from './test/renderWithProviders';

function healthHandler() {
  return http.get('http://localhost:8080/health/all', () =>
    HttpResponse.json({ gateway: 'ok', all_upstreams_ok: true, upstreams: [] }),
  );
}

it('shows the three sections and opens the character page by default', async () => {
  server.use(healthHandler());

  renderWithProviders(<App />, { route: '/' });

  expect(screen.getByRole('link', { name: 'Персонаж' })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Рецепты' })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Чат' })).toBeInTheDocument();
  expect(await screen.findByRole('heading', { name: 'Персонаж' })).toBeInTheDocument();
});

it('navigates to the recipes section', async () => {
  server.use(healthHandler());

  renderWithProviders(<App />, { route: '/' });
  await userEvent.click(screen.getByRole('link', { name: 'Рецепты' }));

  expect(await screen.findByRole('heading', { name: 'Рецепты' })).toBeInTheDocument();
});

it('reports the failing upstream when the gateway is degraded', async () => {
  server.use(
    http.get('http://localhost:8080/health/all', () =>
      HttpResponse.json({
        gateway: 'ok',
        all_upstreams_ok: false,
        upstreams: [
          { target: 'http://recipe-service:8082', ok: false, status: 0, error: 'connection refused', ms: 1 },
        ],
      }),
    ),
  );

  renderWithProviders(<App />, { route: '/' });

  expect(await screen.findByText(/recipe-service/)).toBeInTheDocument();
});
