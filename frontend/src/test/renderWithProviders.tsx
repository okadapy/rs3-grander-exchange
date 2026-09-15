import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import { useMemo } from 'react';
import type { ReactElement, ReactNode } from 'react';
import { AuthProvider } from '../auth/AuthProvider';
import { PlayerPrefsProvider } from '../features/character/usePlayerPrefs';
import { theme } from '../theme/theme';
import type { WebSocketLike } from '../ws/connection';
import { WsProvider } from '../ws/WsProvider';

export function renderWithProviders(
  ui: ReactElement,
  opts?: { socketFactory?: (url: string) => WebSocketLike },
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });

  function Wrapper({ children }: { children: ReactNode }) {
    // A fresh closure on every render would tear the socket down and
    // reconnect on every render; memoising keeps the factory reference
    // stable across the test's lifetime. opts is a per-call constant in
    // every test, so it is intentionally left out of the deps array.
    const factory = useMemo(() => opts?.socketFactory, []);
    return (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider theme={theme}>
          <AuthProvider>
            <WsProvider socketFactory={factory}>
              <PlayerPrefsProvider>{children}</PlayerPrefsProvider>
            </WsProvider>
          </AuthProvider>
        </ThemeProvider>
      </QueryClientProvider>
    );
  }

  const view = render(<Wrapper>{ui}</Wrapper>);

  return {
    ...view,
    // Re-renders through the same Wrapper element so a test can simulate a
    // prop change coming from a parent (e.g. the shell handing down a new
    // selected skill) without tearing down the QueryClient/auth/websocket/
    // prefs context, which a bare `rerender(<NextUi />)` would otherwise
    // replace.
    rerender: (next: ReactElement) => view.rerender(<Wrapper>{next}</Wrapper>),
  };
}
