import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it } from 'vitest';
import type { WebSocketLike } from '../../ws/connection';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { ChatPopup } from './ChatPopup';

class FakeSocket implements WebSocketLike {
  static last: FakeSocket | null = null;

  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(readonly url: string) {
    FakeSocket.last = this;
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    /* nothing to do in the fake */
  }
}

const HISTORY = {
  messages: [
    { id: 1, user_id: 1, username: 'okadishe', body: 'Hey!', created_at: '2026-09-15T07:06:48.728Z' },
  ],
};

function historyHandler() {
  return http.get('http://localhost:8080/chat/history', () => HttpResponse.json(HISTORY));
}

function signedIn() {
  localStorage.setItem('rs3.auth', JSON.stringify({ token: 'jwt-abc', username: 'okadishe' }));
}

async function expandPopup() {
  await userEvent.click(screen.getByRole('button', { name: 'Chat' }));
}

beforeEach(() => {
  localStorage.clear();
  FakeSocket.last = null;
  server.use(historyHandler());
});

it('starts collapsed and expands when clicked', async () => {
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });

  expect(screen.queryByText('Hey!')).not.toBeInTheDocument();
  await expandPopup();

  expect(await screen.findByText('Hey!')).toBeInTheDocument();
});

it('invites the visitor to sign in and hides the composer', async () => {
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });
  await expandPopup();

  expect(await screen.findByText('Hey!')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument();
  expect(screen.queryByLabelText('Message')).not.toBeInTheDocument();
});

it('shows a live message that arrives over the socket', async () => {
  signedIn();
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });
  await expandPopup();

  await screen.findByText('Hey!');
  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onmessage?.({
      data: JSON.stringify({
        type: 'chat',
        payload: { id: 2, user_id: 2, username: 'fe_probe', body: 'Live message', created_at: '2026-09-15T10:29:51.487Z' },
      }),
    });
  });

  expect(await screen.findByText('Live message')).toBeInTheDocument();
  expect(screen.getByText('fe_probe')).toBeInTheDocument();
});

it('sends a typed message as a command frame and clears the field', async () => {
  signedIn();
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });
  await expandPopup();

  await screen.findByText('Hey!');
  act(() => {
    FakeSocket.last?.onopen?.();
  });

  const field = screen.getByLabelText('Message');
  await userEvent.type(field, 'Hello there');
  await userEvent.click(screen.getByRole('button', { name: 'Send' }));

  await waitFor(() =>
    expect(FakeSocket.last?.sent).toContain(JSON.stringify({ action: 'chat', body: 'Hello there' })),
  );
  expect(field).toHaveValue('');
});

it('does not echo the sent message until the server returns it', async () => {
  signedIn();
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });
  await expandPopup();

  await screen.findByText('Hey!');
  act(() => {
    FakeSocket.last?.onopen?.();
  });
  await userEvent.type(screen.getByLabelText('Message'), 'Should not appear');
  await userEvent.click(screen.getByRole('button', { name: 'Send' }));

  expect(screen.queryByText('Should not appear')).not.toBeInTheDocument();
});

it('reports a rejected token with a sign-out action, without signing out automatically', async () => {
  signedIn();
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });
  await expandPopup();

  await screen.findByText('Hey!');
  act(() => {
    FakeSocket.last?.onclose?.({ code: 1008 });
  });

  const warning = await screen.findByText(/Session is no longer valid/);
  expect(warning).toBeInTheDocument();
  expect(within(warning.closest('[role="alert"]') as HTMLElement).getByRole('button', { name: 'Sign out' })).toBeInTheDocument();
});

it('counts messages that arrive while collapsed', async () => {
  signedIn();
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });

  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onmessage?.({
      data: JSON.stringify({
        type: 'chat',
        payload: { id: 2, user_id: 2, username: 'fe_probe', body: 'While away', created_at: '2026-09-15T10:29:51.487Z' },
      }),
    });
  });

  expect(await screen.findByLabelText('Chat, 1 unread message')).toBeInTheDocument();
});

it('clears the unread count once expanded', async () => {
  signedIn();
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });

  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onmessage?.({
      data: JSON.stringify({
        type: 'chat',
        payload: { id: 2, user_id: 2, username: 'fe_probe', body: 'While away', created_at: '2026-09-15T10:29:51.487Z' },
      }),
    });
  });
  await userEvent.click(await screen.findByLabelText('Chat, 1 unread message'));

  expect(screen.getByRole('button', { name: 'Chat' })).toBeInTheDocument();
});
