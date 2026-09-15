import { describe, expect, it } from 'vitest';
import { ELITE_XP_TABLE, STANDARD_XP_TABLE, XP_CAP, levelProgress } from './experience';

describe('levelProgress', () => {
  it('computes the fraction and remaining xp for a mid-level skill', () => {
    // Crafting 99, halfway-ish to 100 on the standard table.
    const base = STANDARD_XP_TABLE[98]; // level 99 requirement
    const next = STANDARD_XP_TABLE[99]; // level 100 requirement
    const xp = base + Math.round((next - base) / 2);

    const progress = levelProgress('Crafting', 99, xp);

    expect(progress.complete).toBe(false);
    expect(progress.fraction).toBeCloseTo(0.5, 2);
    expect(progress.remainingXp).toBe(next - xp);
  });

  it('reads exactly 0 progress right at a level boundary', () => {
    const base = STANDARD_XP_TABLE[49]; // level 50 requirement, exactly

    const progress = levelProgress('Attack', 50, base);

    expect(progress.complete).toBe(false);
    expect(progress.fraction).toBe(0);
    expect(progress.remainingXp).toBe(STANDARD_XP_TABLE[50] - base);
  });

  it('reads Invention off the elite table, not the standard one', () => {
    const eliteBase = ELITE_XP_TABLE[89]; // level 90 requirement (elite)
    const standardBase = STANDARD_XP_TABLE[89]; // level 90 requirement (standard)
    expect(eliteBase).not.toBe(standardBase);

    const progress = levelProgress('Invention', 90, eliteBase);

    expect(progress.complete).toBe(false);
    expect(progress.fraction).toBe(0);
    expect(progress.remainingXp).toBe(ELITE_XP_TABLE[90] - eliteBase);
  });

  it('treats a skill at the 200M experience ceiling as complete', () => {
    const progress = levelProgress('Attack', 120, XP_CAP);

    expect(progress.complete).toBe(true);
    expect(progress.fraction).toBe(1);
    expect(progress.remainingXp).toBe(0);
  });
});
