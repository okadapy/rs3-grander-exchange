import { useEffect, useState } from 'react';

/**
 * Holds a value back until it has stopped changing for `delayMs`. A search
 * field otherwise fires a request per keystroke, and the answers race each
 * other back — the last one rendered is not necessarily the last one typed.
 */
export function useDebounced<T>(value: T, delayMs: number): T {
  const [settled, setSettled] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return settled;
}
