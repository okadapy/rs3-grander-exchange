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

- `SmeltTicks(bar string, smithingLevel int) (int, bool)` — the table above.
- `IsStackable(itemName string) bool` — the stackable families.
- `SlotsPerCraft(inputs []models.RecipeInput) int`.
- `ActionsPerHour(ticks, slotsPerCraft int, cfg Config) int` — the formula.
- `Config` — `InventorySlots` (28), `BankTripTicks` (21), `TickSeconds` (0.6).

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
3. numeric `ticks` on the recipe → `ticks`
4. `aph` from the wiki → `wiki`
5. configured default → `default`

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

## Out of scope

- **Anvil forging.** Progress and heat
  (`max heat = 300 + 3×Smithing + 3×Firemaking`, three multiplier tiers,
  trips back to the forge) is a subsystem of its own. Until it exists,
  those recipes stay honestly marked `aph_source: default`.
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

`calc-service` — the precedence in `chooseAPH` and the `aph_source` each
branch reports.

`/calc/top` — sorting per metric, the skill filter, the use of
`gp_per_hour_limited` for the money metric, and the contract test against
the spec.
