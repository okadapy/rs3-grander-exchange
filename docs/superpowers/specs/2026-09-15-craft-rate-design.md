# Tick-based craft rates and stats routes

Date: 2026-09-15
Status: approved, ready for planning

## The problem

Every recipe reports a house assumption for actions per hour. In the live
database `aph_source` is `default` on all 6035 fully priced paths. Every
GP/h figure is therefore one craft's profit multiplied by 600, which is
how a godsword ends up listed at 938M GP/h — a number nobody can reach,
sitting in the same column as one they can.

The rate has to come from game mechanics instead, and the three headline
metrics need an endpoint of their own.

## What the wiki provides

`Infobox Recipe` carries a `|ticks =` parameter. Measured over 40 random
recipes: 22 numeric (55%), 9 empty, 4 `varies`, 4 with no field at all.
Usable for the majority, so a fallback path stays mandatory.

An RS3 tick is 0.6s, so the ceiling is `6000 / ticks` actions per hour.

Smelting is level-dependent. From the `Smithing` page — the Smithing
level at which the operation drops to 5, 4 and 3 ticks:

| Bar | 5t | 4t | 3t |
|---|---|---|---|
| Bronze | 1 | 2 | 5 |
| Iron | 10 | 14 | 17 |
| Steel | 20 | 21 | 23 |
| Mithril | 30 | 31 | 37 |
| Adamant | 40 | 46 | 48 |
| Rune | 50 | 53 | 56 |
| Orikalkum | 60 | 61 | 65 |
| Necronium | 70 | 75 | 77 |
| Bane | 80 | 86 | 89 |
| Elder rune | 90 | 91 | 94 |
| Primal | 100 | 101 | 106 |

The table is a constant in code rather than scraped: eleven rows that
change once in a generation.

Note that the infobox holds the best case. `Steel bar` says `|ticks = 3`,
but a level 20 player smelts it in 5. So when the player's level is
known, the table wins over the infobox value, not the other way round.

A recipe counts as smelting when `facility = Furnace` **and** the output
name is in the table. Both conditions are needed — a furnace is used for
more than bars.

## The rate model

The raw tick ceiling overstates throughput, because the inventory runs
out and the player walks to a bank.

```
crafts_per_trip = floor(28 / slots_per_craft)
aph = crafts_per_trip * 6000 / (crafts_per_trip * ticks + bank_trip_ticks)
```

`slots_per_craft` counts non-stackable **inputs** only. Stackables (runes,
coins, energies, shards, feathers, bolts, arrows, charms) take no slot;
the families are listed in code. When every input stacks, the banking
term degenerates and the plain `6000 / ticks` ceiling applies.

The output takes no slot either: it appears in place of the inputs it
consumed, so the inventory never fills before the materials run out.

Calibration: `ticks = 3`, `crafts_per_trip = 28`, `bank_trip_ticks = 21`
gives exactly 1600 crafts per hour, which is the independently timed rate
for smelting a full inventory of bars.

## Components

### `shared/rates` (new package)

Pure functions, no database and no HTTP, so they test directly:

- `SmeltTicks(bar string, smithingLevel int) (int, bool)` — the smelting table.
- `ForgeTicks(bars int, metal string, smithing, firemaking int) (int, bool)`
  — the forging simulation described below.
- `BarMetal(itemName string) (string, bool)` — recognises a bar and its
  metal, shared by both Smithing paths.
- `IsStackable(itemName string) bool` — the stackable families.
- `SlotsPerCraft(inputs []models.RecipeInput) int`.
- `ActionsPerHour(ticks, slotsPerCraft int, cfg Config) int` — the formula.
- `Config` — `InventorySlots` (28), `BankTripTicks` (21), `TickSeconds` (0.6).

The two Smithing tables (ticks-to-smelt by level, progress-per-bar and
reheat milestones by metal) are constants in this package.

### `recipe-service`

The parser starts reading `|ticks =` (numeric values only) and
`|facility =`. `varies`, an empty value and a missing field all yield
`ticks = 0`.

`Recipe` gains `Ticks int` and `Facility string`. Migration 004 adds the
columns; AutoMigrate covers a fresh database.

### `calc-service`

One integration point: `chooseAPH` in
`calc-service/internal/service/calc.go`. New precedence:

1. `ActionsPerHourOverride` from the request → `aph_source: override`
2. smelting, known bar, known player level → `ticks_level`
3. forging, known bar, known Smithing and Firemaking levels → `ticks_forge`
4. numeric `ticks` on the recipe → `ticks`
5. `aph` from the wiki → `wiki`
6. configured default → `default`

Player levels already reach calc through the `player` parameter; no extra
call to hiscore-service is needed.

The banking parameters live under `market:` in config and are echoed in
the response `assumptions` block, beside `spread_pct` — same principle,
an assumption the caller can see.

### `GET /calc/top`

```
/calc/top?metric=xp_per_hour|gp_per_hour|gp_per_xp
         &player=&skill=&limit=&include_incomplete=
```

`limit` defaults to 20, maximum 100. `player` is optional: without it
there is no level filtering and no `ticks_level` branch — the rate comes
from the infobox, and `aph_source` says so.

The response is a sorted list of `item_id`, `name`, `skill`, `level_req`,
`xp_per_hour`, `gp_per_hour`, `gp_per_hour_limited`, `gp_per_xp`,
`actions_per_hour`, `aph_source`, `complete` and `binding_item_name`,
plus one shared `assumptions` block.

For `metric=gp_per_hour` the sort key is `gp_per_hour_limited`, the
buy-limit-bound figure. Sorting on the raw number floats exactly the
items nobody can make 600 times an hour.

The route walks every priceable recipe (~5800), so the cache is not
optional: key on the full parameter set, through the existing
`CalcCache`.

`openapi/combined.yaml` gains the route, then `make openapi`; the
calc-service contract test enforces the match.

## Anvil forging

Forging is not a fixed number of ticks, which is why the infobox says
`varies`. The player fills a progress meter by striking an anvil, and how
fast it fills depends on how hot the item is.

The mechanics, from the `Smithing` page:

- A strike lands every 2 ticks, costs 10 heat, and adds
  `base progress x heat multiplier`.
- Base progress is 10, plus 1 at 99 Smithing.
- Heat multiplier by share of maximum heat: high (67-100%) x2, medium
  (34-66%) x1.6, low (1-33%) x1.3, zero x1.
- `max heat = 300 + 3 x Smithing + 3 x Firemaking`.
- Reheating from empty to full takes 6 ticks, dropping to 4 and then 2 at
  per-metal Smithing milestones (Steel 22 and 26, Rune 54 and 58).

Progress required is the bar count times a per-metal constant, and the
constant is linear in tier:

| Metal | Progress/bar | | Metal | Progress/bar |
|---|---|---|---|---|
| Bronze | 100 | | Orikalkum | 700 |
| Iron | 200 | | Necronium | 800 |
| Steel | 300 | | Bane, Obsidian | 900 |
| Mithril | 400 | | Elder rune | 1000 |
| Adamant | 500 | | Primal | 1100 |
| Rune | 600 | | | |

Checked against three pages: rune platebody 5 bars x 600 = 3000, rune
dagger 2 x 600 = 1200, steel platebody 5 x 300 = 1500. All three match
the prose on those pages.

A recipe counts as forging when `facility = Anvil` and one of its inputs
is a bar in the table. The bar count comes from that input's quantity, so
the `+1 / +2 / +3` upgrade chain needs no special case: each step is its
own recipe consuming the previous item plus its own bars, and the recipe
tree already walks it.

`rates.ForgeTicks(bars, metal string, smithing, firemaking int) int`
simulates one item: start at full heat, strike until progress is met,
reheat whenever heat reaches zero, and return total ticks. Deterministic,
so it tests directly. The result feeds the same banking model as every
other recipe.

### Forging simplifications

Stated because they bound how honest the number is:

- **No perks, gear or consumables.** Rapid, Tinker, Careless, luminite
  injectors, juju potions, Varrock armour and the Smithing cape reheat
  perk together more than double progress per strike. None of it is
  visible in a hiscore lookup, so the model assumes none of it and
  reports a floor rather than a ceiling.
- **No +50/+100 material heat bonus.** It raises maximum heat at certain
  levels, but tier boundaries stay thirds of the *unbonused* maximum,
  which makes the interaction fiddly for a small gain. Excluded, which
  understates high-level rates slightly.
- **No Superheat Item cycling.** Modelling optimal spell play would
  describe a player nobody is, and it needs runes the calculator is not
  costing.

`aph_source` for these recipes is `ticks_forge`, so the frontend can say
where the number came from and that it is a conservative one.

## Out of scope

- **Scraping the `stackable` flag.** The family list in code covers the
  real cases; a pass over ~7000 item pages for one boolean does not pay.
- **Double bar chance** (10% at high levels) — changes output, not speed.
- **Shop prices** (runes bought at a fixed price rather than on the GE) —
  that is a cost basis, not a rate. Separate feature.

## Testing

TDD; tests before code.

`shared/rates` — the smelting table at its boundaries (Steel 20/21/23 →
5/4/3 ticks, below 20 → no value), stackable families, the formula at the
calibration point (3 ticks, 28 slots, 21 tick trip → exactly 1600), and
the degenerate all-stackable case.

`recipe-service` — the parser against `ticks = 3`, `ticks = varies`, an
empty value and a missing field.

Forging — the simulation against hand-computed cases: a steel platebody
(1500 progress) at a level where heat is high throughout, one where the
item cools into the low tier and needs a reheat, and the boundary where a
reheat milestone changes the tick cost.

`calc-service` — the precedence in `chooseAPH` and the `aph_source` each
branch reports.

`/calc/top` — sorting per metric, the skill filter, the use of
`gp_per_hour_limited` for the money metric, and the contract test against
the spec.
