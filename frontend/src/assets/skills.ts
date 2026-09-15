// Game order, matching what /hiscore/{name} returns after the Overall entry.
export const SKILLS = [
  'Attack', 'Defence', 'Strength', 'Constitution', 'Ranged', 'Prayer',
  'Magic', 'Cooking', 'Woodcutting', 'Fletching', 'Fishing', 'Firemaking',
  'Crafting', 'Smithing', 'Mining', 'Herblore', 'Agility', 'Thieving',
  'Slayer', 'Farming', 'Runecrafting', 'Hunter', 'Construction', 'Summoning',
  'Dungeoneering', 'Divination', 'Invention', 'Archaeology', 'Necromancy',
] as const;

export type SkillName = (typeof SKILLS)[number];

const BY_LOWER = new Map<string, SkillName>(SKILLS.map((s) => [s.toLowerCase(), s]));

// The recipe table stores 'No', 'no' and 'None' where a skill is unknown,
// alongside real names in mixed case. Everything else is not a skill.
export function canonicalSkill(raw: string): SkillName | null {
  return BY_LOWER.get(raw.trim().toLowerCase()) ?? null;
}

export function skillIconUrl(raw: string): string | null {
  const skill = canonicalSkill(raw);
  return skill ? `/icons/skills/${skill}-icon.png` : null;
}

export function itemIconUrl(itemId: number): string {
  return `https://secure.runescape.com/m=itemdb_rs/obj_big.gif?id=${itemId}`;
}
