// One-off download of the 29 skill icons from the RuneScape Wiki into
// public/icons/skills/. The icons are committed so the running app makes
// no third-party requests and offline development keeps its artwork.
//
// Node cannot import a TypeScript file directly, so the skill list is
// duplicated here as a literal array rather than imported from
// ../src/assets/skills.ts. Keep the two lists in sync if skills change.
import { mkdir, writeFile } from 'node:fs/promises';

const SKILLS = [
  'Attack', 'Defence', 'Strength', 'Constitution', 'Ranged', 'Prayer',
  'Magic', 'Cooking', 'Woodcutting', 'Fletching', 'Fishing', 'Firemaking',
  'Crafting', 'Smithing', 'Mining', 'Herblore', 'Agility', 'Thieving',
  'Slayer', 'Farming', 'Runecrafting', 'Hunter', 'Construction', 'Summoning',
  'Dungeoneering', 'Divination', 'Invention', 'Archaeology', 'Necromancy',
];

const OUT = new URL('../public/icons/skills/', import.meta.url);

await mkdir(OUT, { recursive: true });

for (const skill of SKILLS) {
  const file = `${skill}-icon.png`;
  const response = await fetch(`https://runescape.wiki/images/${file}`, {
    headers: { 'User-Agent': 'rs3-market-frontend/1.0 (asset fetch)' },
  });
  if (!response.ok) {
    throw new Error(`${file}: HTTP ${response.status}`);
  }
  await writeFile(new URL(file, OUT), Buffer.from(await response.arrayBuffer()));
  console.log(`saved ${file}`);
}
