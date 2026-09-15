import { describe, expect, it } from 'vitest';
import { SKILLS, canonicalSkill, itemIconUrl, skillIconUrl } from './skills';

describe('SKILLS', () => {
  it('lists the 29 skills without Overall', () => {
    expect(SKILLS).toHaveLength(29);
    expect(SKILLS).toContain('Crafting');
    expect(SKILLS).toContain('Necromancy');
    expect(SKILLS).not.toContain('Overall');
  });
});

describe('canonicalSkill', () => {
  it('repairs the casing the recipe data ships with', () => {
    expect(canonicalSkill('Crafting')).toBe('Crafting');
    expect(canonicalSkill('crafting')).toBe('Crafting');
    expect(canonicalSkill('SMITHING')).toBe('Smithing');
  });

  it('rejects the placeholder values in the recipe data', () => {
    expect(canonicalSkill('No')).toBeNull();
    expect(canonicalSkill('no')).toBeNull();
    expect(canonicalSkill('None')).toBeNull();
    expect(canonicalSkill('')).toBeNull();
  });
});

describe('skillIconUrl', () => {
  it('points at the committed icon file', () => {
    expect(skillIconUrl('crafting')).toBe('/icons/skills/Crafting-icon.png');
  });

  it('returns null for a non-skill', () => {
    expect(skillIconUrl('No')).toBeNull();
  });
});

describe('itemIconUrl', () => {
  it('builds the official item database url', () => {
    expect(itemIconUrl(1603)).toBe(
      'https://secure.runescape.com/m=itemdb_rs/obj_big.gif?id=1603',
    );
  });
});
