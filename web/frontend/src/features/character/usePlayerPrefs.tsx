import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import type { HiscoreMode } from '../../api/queries/hiscore';

const STORAGE_KEY = 'rs3.player';

interface Prefs {
  name: string;
  mode: HiscoreMode;
}

export interface PlayerPrefs extends Prefs {
  setName(name: string): void;
  setMode(mode: HiscoreMode): void;
}

const EMPTY: Prefs = { name: '', mode: 'normal' };

function read(): Prefs {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return EMPTY;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (parsed && typeof parsed === 'object' && typeof (parsed as Prefs).name === 'string') {
      return { name: (parsed as Prefs).name, mode: (parsed as Prefs).mode ?? 'normal' };
    }
  } catch {
    // Fall through to the empty preferences below.
  }
  return EMPTY;
}

const PlayerPrefsContext = createContext<PlayerPrefs | null>(null);

// The character column and the recipe table both read the player name, and
// the name is what decides whether /calc/batch can report level eligibility.
// Held as plain component state it existed twice, so typing a name in the
// column left the table's copy empty until a full reload; one provider is
// what keeps the two in step.
export function PlayerPrefsProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefs] = useState<Prefs>(read);

  const save = useCallback((next: Prefs) => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
    setPrefs(next);
  }, []);

  const value = useMemo<PlayerPrefs>(
    () => ({
      name: prefs.name,
      mode: prefs.mode,
      setName: (name: string) => save({ ...prefs, name }),
      setMode: (mode: HiscoreMode) => save({ ...prefs, mode }),
    }),
    [prefs, save],
  );

  return <PlayerPrefsContext.Provider value={value}>{children}</PlayerPrefsContext.Provider>;
}

export function usePlayerPrefs(): PlayerPrefs {
  const value = useContext(PlayerPrefsContext);
  if (!value) throw new Error('usePlayerPrefs used outside PlayerPrefsProvider');
  return value;
}
