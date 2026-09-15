import { useCallback, useState } from 'react';
import type { HiscoreMode } from '../../api/queries/hiscore';

const STORAGE_KEY = 'rs3.player';

interface Prefs {
  name: string;
  mode: HiscoreMode;
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

export function usePlayerPrefs() {
  const [prefs, setPrefs] = useState<Prefs>(read);

  const save = useCallback((next: Prefs) => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
    setPrefs(next);
  }, []);

  return {
    name: prefs.name,
    mode: prefs.mode,
    setName: useCallback((name: string) => save({ ...prefs, name }), [prefs, save]),
    setMode: useCallback((mode: HiscoreMode) => save({ ...prefs, mode }), [prefs, save]),
  };
}
