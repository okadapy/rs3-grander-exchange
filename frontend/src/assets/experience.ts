// Experience-to-level tables, index 0 is level 1. Verified against the
// wiki's own anchors: standard level 99 = 13,034,431, level 110 = 38,737,661,
// level 120 = 104,273,167; elite level 99 = 36,073,511, level 120 = 80,618,654.
// Both run to level 150 because the hiscores report virtual levels above a
// skill's nominal cap (e.g. Attack reads 120 at 200,000,000 xp). Do not
// re-derive or shorten these — they are the source of truth for the bar.

// The documented geometric leveling curve every skill except Invention uses.
export const STANDARD_XP_TABLE: readonly number[] = [
  0, 83, 174, 276, 388, 512, 650, 801, 969, 1154,
  1358, 1584, 1833, 2107, 2411, 2746, 3115, 3523, 3973, 4470,
  5018, 5624, 6291, 7028, 7842, 8740, 9730, 10824, 12031, 13363,
  14833, 16456, 18247, 20224, 22406, 24815, 27473, 30408, 33648, 37224,
  41171, 45529, 50339, 55649, 61512, 67983, 75127, 83014, 91721, 101333,
  111945, 123660, 136594, 150872, 166636, 184040, 203254, 224466, 247886, 273742,
  302288, 333804, 368599, 407015, 449428, 496254, 547953, 605032, 668051, 737627,
  814445, 899257, 992895, 1096278, 1210421, 1336443, 1475581, 1629200, 1798808, 1986068,
  2192818, 2421087, 2673114, 2951373, 3258594, 3597792, 3972294, 4385776, 4842295, 5346332,
  5902831, 6517253, 7195629, 7944614, 8771558, 9684577, 10692629, 11805606, 13034431, 14391160,
  15889109, 17542976, 19368992, 21385073, 23611006, 26068632, 28782069, 31777943, 35085654, 38737661,
  42769801, 47221641, 52136869, 57563718, 63555443, 70170840, 77474828, 85539082, 94442737, 104273167,
  115126838, 127110260, 140341028, 154948977, 171077457, 188884740, 208545572, 230252886, 254219702, 280681209,
  309897078, 342154009, 377768545, 417090179, 460504778, 508438379, 561361362, 619793069, 684306901, 755535943,
  834179178, 921008346, 1016875516, 1122721449, 1239584831, 1368612462, 1511070513, 1668356950, 1842015252, 2033749558,
];

// Invention is the only elite skill in the game (Module:Experience/elitedata).
// Archaeology, Dungeoneering and Necromancy are ordinary skills that merely
// go to 120 — they use the standard table above, not this one.
export const ELITE_XP_TABLE: readonly number[] = [
  0, 830, 1861, 2902, 3980, 5126, 6380, 7787, 9400, 11275,
  13605, 16372, 19656, 23546, 28134, 33520, 39809, 47109, 55535, 65209,
  77190, 90811, 106221, 123573, 143025, 164742, 188893, 215651, 245196, 277713,
  316311, 358547, 404634, 454796, 509259, 568254, 632019, 700797, 774834, 854383,
  946227, 1044569, 1149696, 1261903, 1381488, 1508756, 1644015, 1787581, 1939773, 2100917,
  2283490, 2476369, 2679917, 2894505, 3120508, 3358307, 3608290, 3870846, 4146374, 4435275,
  4758122, 5096111, 5449685, 5819299, 6205407, 6608473, 7028964, 7467354, 7924122, 8399751,
  8925664, 9472665, 10041285, 10632061, 11245538, 11882262, 12542789, 13227679, 13937496, 14672812,
  15478994, 16313404, 17176661, 18069395, 18992239, 19945833, 20930821, 21947856, 22997593, 24080695,
  25259906, 26475754, 27728955, 29020233, 30350318, 31719944, 33129852, 34580790, 36073511, 37608773,
  39270442, 40978509, 42733789, 44537107, 46389292, 48291180, 50243611, 52247435, 54303504, 56412678,
  58575824, 60793812, 63067521, 65397835, 67785643, 70231841, 72737330, 75303019, 77929820, 80618654,
  83370445, 86186124, 89066630, 92012904, 95025896, 98106559, 101255855, 104474750, 107764216, 111125230,
  114558777, 118065845, 121647430, 125304532, 129038159, 132849323, 136739041, 140708338, 144758242, 148889790,
  153104021, 157401983, 161784728, 166253312, 170808801, 175452262, 180184770, 185007406, 189921255, 194927409,
];

// The hard experience ceiling for any skill in the live game.
export const XP_CAP = 200_000_000;

export interface LevelProgress {
  /** True once xp has reached the 200M ceiling: the bar is full and there is
   *  no meaningful next-level fraction left to show. */
  complete: boolean;
  /** 0..1 progress from the current level's requirement to the next one's.
   *  Meaningless (and left at 1) when complete is true. */
  fraction: number;
  /** Experience remaining until the next level, 0 when complete. */
  remainingXp: number;
}

function xpTableFor(skill: string): readonly number[] {
  return skill === 'Invention' ? ELITE_XP_TABLE : STANDARD_XP_TABLE;
}

// table[level - 1] is the requirement for the current level (index 0 = level
// 1); table[level] is the requirement for the next one.
export function levelProgress(skill: string, level: number, xp: number): LevelProgress {
  if (xp >= XP_CAP) return { complete: true, fraction: 1, remainingXp: 0 };

  const table = xpTableFor(skill);
  if (level < 1 || level >= table.length) {
    return { complete: true, fraction: 1, remainingXp: 0 };
  }

  const base = table[level - 1];
  const next = table[level];
  if (next <= base) {
    return { complete: true, fraction: 1, remainingXp: 0 };
  }

  const fraction = Math.min(Math.max((xp - base) / (next - base), 0), 1);
  return { complete: false, fraction, remainingXp: Math.max(next - xp, 0) };
}
