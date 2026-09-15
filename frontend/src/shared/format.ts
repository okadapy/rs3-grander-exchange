const ABSENT = '—';

function isAbsent(value: number | null | undefined): value is null | undefined {
  return value === null || value === undefined || Number.isNaN(value);
}

function trim(value: number, digits: number): string {
  return Number(value.toFixed(digits)).toString();
}

export function formatCompact(value: number | null | undefined): string {
  if (isAbsent(value)) return ABSENT;

  const abs = Math.abs(value);
  if (abs < 1_000) return Math.round(value).toString();
  if (abs < 1_000_000) return `${trim(value / 1_000, 1)}K`;
  if (abs < 1_000_000_000) return `${trim(value / 1_000_000, 2)}M`;
  return `${trim(value / 1_000_000_000, 2)}B`;
}

const integerFormat = new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 });

export function formatInt(value: number | null | undefined): string {
  if (isAbsent(value)) return ABSENT;
  return integerFormat.format(value);
}

export function formatPct(
  value: number | null | undefined,
  opts?: { sign?: boolean },
): string {
  if (isAbsent(value)) return ABSENT;
  const body = `${value.toFixed(1)}%`;
  return opts?.sign && value > 0 ? `+${body}` : body;
}
