import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import { useMemo } from 'react';
import type { ReactElement } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider } from '../auth/AuthProvider';
import { theme } from '../theme/theme';
import type { WebSocketLike } from '../ws/connection';
import { WsProvider } from '../ws/WsProvider';

export function renderWithProviders(
  ui: ReactElement,
  opts?: { route?: string; socketFactory?: (url: string) => WebSocketLike },
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });

  function Wrapper() {
    // A fresh closure on every render would tear the socket down and
    // reconnect on every render; memoising keeps the factory reference
    // stable across the test's lifetime. opts is a per-call constant in
    // every test, so it is intentionally left out of the deps array.
    const factory = useMemo(() => opts?.socketFactory, []);
    return (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider theme={theme}>
          <MemoryRouter initialEntries={[opts?.route ?? '/']}>
            <AuthProvider>
              <WsProvider socketFactory={factory}>{ui}</WsProvider>
            </AuthProvider>
          </MemoryRouter>
        </ThemeProvider>
      </QueryClientProvider>
    );
  }

  return render(<Wrapper />);
}
