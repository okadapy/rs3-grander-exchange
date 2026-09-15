import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it } from 'vitest';
import { server } from '../test/msw/server';
import { AuthProvider, useAuth } from './AuthProvider';

function Probe() {
  const auth = useAuth();
  return (
    <div>
      <span data-testid="token">{auth.token ?? 'no token'}</span>
      <span data-testid="username">{auth.username ?? 'anonymous'}</span>
      <span data-testid="error">{auth.error ?? ''}</span>
      <button onClick={() => void auth.signIn({ username: 'okadishe', password: 'secret123' })}>
        Sign in
      </button>
      <button onClick={() => auth.signOut()}>Sign out</button>
    </div>
  );
}

beforeEach(() => localStorage.clear());

it('stores the token after a successful login and restores it on remount', async () => {
  server.use(
    http.post('http://localhost:8080/auth/login', () =>
      HttpResponse.json({ token: 'jwt-abc' }),
    ),
  );

  const { unmount } = render(<AuthProvider><Probe /></AuthProvider>);
  await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

  await waitFor(() => expect(screen.getByTestId('token')).toHaveTextContent('jwt-abc'));
  expect(screen.getByTestId('username')).toHaveTextContent('okadishe');

  unmount();
  render(<AuthProvider><Probe /></AuthProvider>);
  expect(screen.getByTestId('token')).toHaveTextContent('jwt-abc');
});

it('surfaces invalid credentials without storing anything', async () => {
  server.use(
    http.post('http://localhost:8080/auth/login', () =>
      HttpResponse.json({ error: 'invalid credentials' }, { status: 401 }),
    ),
  );

  render(<AuthProvider><Probe /></AuthProvider>);
  await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

  await waitFor(() => expect(screen.getByTestId('error')).toHaveTextContent('invalid credentials'));
  expect(screen.getByTestId('token')).toHaveTextContent('no token');
  expect(localStorage.getItem('rs3.auth')).toBeNull();
});

it('clears the stored session on sign out', async () => {
  server.use(
    http.post('http://localhost:8080/auth/login', () => HttpResponse.json({ token: 'jwt-abc' })),
  );

  render(<AuthProvider><Probe /></AuthProvider>);
  await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
  await waitFor(() => expect(screen.getByTestId('token')).toHaveTextContent('jwt-abc'));

  await userEvent.click(screen.getByRole('button', { name: 'Sign out' }));

  expect(screen.getByTestId('token')).toHaveTextContent('no token');
  expect(localStorage.getItem('rs3.auth')).toBeNull();
});
