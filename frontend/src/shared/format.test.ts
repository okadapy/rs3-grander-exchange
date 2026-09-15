import { describe, expect, it } from 'vitest';
import { formatCompact, formatInt, formatPct } from './format';

describe('formatCompact', () => {
  it('renders absent values as an em dash', () => {
    expect(formatCompact(null)).toBe('—');
    expect(formatCompact(undefined)).toBe('—');
    expect(formatCompact(Number.NaN)).toBe('—');
  });

  it('renders small values as whole numbers', () => {
    expect(formatCompact(0)).toBe('0');
    expect(formatCompact(307)).toBe('307');
    expect(formatCompact(999.6)).toBe('1000');
  });

  it('abbreviates thousands, millions and billions', () => {
    expect(formatCompact(1000)).toBe('1K');
    expect(formatCompact(12_345)).toBe('12.3K');
    expect(formatCompact(1_234_567)).toBe('1.23M');
    expect(formatCompact(343_000_000)).toBe('343M');
    expect(formatCompact(5_709_998_811)).toBe('5.71B');
  });

  it('keeps the sign on negative values', () => {
    expect(formatCompact(-250)).toBe('-250');
    expect(formatCompact(-12_345)).toBe('-12.3K');
  });
});

describe('formatInt', () => {
  it('groups thousands with a narrow space and handles absent values', () => {
    expect(formatInt(null)).toBe('—');
    expect(formatInt(99)).toBe('99');
    expect(formatInt(171_851)).toBe('171 851');
  });
});

describe('formatPct', () => {
  it('renders one decimal place', () => {
    expect(formatPct(null)).toBe('—');
    expect(formatPct(0)).toBe('0.0%');
    expect(formatPct(12.34)).toBe('12.3%');
    expect(formatPct(-4.5)).toBe('-4.5%');
  });

  it('adds an explicit plus sign when asked', () => {
    expect(formatPct(12.34, { sign: true })).toBe('+12.3%');
    expect(formatPct(-4.5, { sign: true })).toBe('-4.5%');
    expect(formatPct(0, { sign: true })).toBe('0.0%');
  });
});
