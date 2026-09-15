import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { login, register } from '../api/queries/auth';
import type { Credentials } from '../api/queries/auth';

const STORAGE_KEY = 'rs3.auth';

interface Session {
  token: string;
  username: string;
}

export interface AuthContextValue {
  token: string | null;
  username: string | null;
  pending: boolean;
  error: string | null;
  signIn(credentials: Credentials): Promise<void>;
  signUp(credentials: Credentials): Promise<void>;
  signOut(): void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function readSession(): Session | null {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (
      parsed && typeof parsed === 'object' &&
      typeof (parsed as Session).token === 'string' &&
      typeof (parsed as Session).username === 'string'
    ) {
      return parsed as Session;
    }
  } catch {
    // A corrupted entry is indistinguishable from no session.
  }
  localStorage.removeItem(STORAGE_KEY);
  return null;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(readSession);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = useCallback(async (work: () => Promise<Session>) => {
    setPending(true);
    setError(null);
    try {
      const next = await work();
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
      setSession(next);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Неизвестная ошибка');
    } finally {
      setPending(false);
    }
  }, []);

  const signIn = useCallback(
    (credentials: Credentials) =>
      run(async () => ({
        token: await login(credentials),
        username: credentials.username,
      })),
    [run],
  );

  const signUp = useCallback(
    (credentials: Credentials) =>
      run(async () => {
        await register(credentials);
        return { token: await login(credentials), username: credentials.username };
      }),
    [run],
  );

  const signOut = useCallback(() => {
    localStorage.removeItem(STORAGE_KEY);
    setSession(null);
    setError(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      token: session?.token ?? null,
      username: session?.username ?? null,
      pending,
      error,
      signIn,
      signUp,
      signOut,
    }),
    [session, pending, error, signIn, signUp, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth используется вне AuthProvider');
  return value;
}
