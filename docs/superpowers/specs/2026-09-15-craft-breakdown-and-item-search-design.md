# Craft breakdown on hover, and a separate item search

## Why

Two gaps in the recipe table as it stands.

The money columns answer *how much*, never *where*. A row says a craft nets
85.7k, but not which stage of the chain earned it and which one destroyed
value. `/calc` already returns that breakdown — `CalcPath.steps`, one entry
per stage with its own buy cost, revenue and profit — and the frontend was
throwing it away.

The table is also reachable only by skill. There was no way to ask about one
named item, which is the first thing anyone wants to do with a market tool.

## Scope

1. Regenerate `schema.d.ts` from the updated `combined.yaml`.
2. A hover breakdown of the winning path, per stage, on the recipe table.
3. An Items tab: search the catalogue by name, pick one, see the same
   breakdown.

`/calc/top` arrived in the same spec revision. It is not used here — a
catalogue-wide ranking is a feature of its own, not a detail of these two.

## 1. The spec

`npm run gen:api` regenerates the types. The gateway serves Swagger UI but no
raw document, so `combined.yaml` in the repository root remains the source.

The regeneration makes `Recipe.ticks`, `CalcAssumptions.inventory_slots` and
`CalcAssumptions.bank_trip_ticks` required, which the test fixtures had to
grow. `AssumptionsBar` now states the last two: the spec introduced
`aph_source` values (`ticks`, `ticks_level`, `ticks_forge`) whose rates are
derived from an assumed inventory size and bank-trip cost, so a GP/h figure
read without them is read without its basis.

## 2. Craft breakdown

`CraftBreakdown` takes an item name and one `CalcPath`, and renders a row per
step: ordinal, recipe, runs per finished item, skill and level, XP, buy, sell
and profit — the server's own figures for that stage, priced on its own.

Two rules it holds to:

- **The stages are never summed.** The first draft of this component carried a
  running profit accumulated down the column, on the assumption that the
  stages compose. Driving the real gateway disproved it: for item 2363 the
  three stages read +856 in total while `profit_per_craft` is −3463, and for
  1673 the stages read +1182 against −1163. An intermediate is consumed by the
  next stage rather than sold, so its revenue never reaches the path. A
  running total over this column is a number that is true of nothing, and it
  would contradict the Margin column fed by the same response. The table says
  so in a line under itself rather than leaving the reader to add the rows up
  by eye.
- The totals row therefore takes `buy_cost`, `sell_revenue` and
  `profit_per_craft` from the path, never from the steps.
- A path with no steps says so instead of drawing an empty table.

It hangs off the item-name cell as a `Tooltip` rather than a popover: the
content is read-only, so nothing in it needs to survive the pointer leaving,
and a tooltip opens on keyboard focus as well as hover.

**No new request.** `/calc/batch` already carries the steps for all 25 items on
the page; `buildRows` kept only `path` (the recipe names, which nothing read)
and now carries the whole `CalcPath` as `bestPath`.

## 3. Items tab

The shell gets MUI `Tabs` — Recipes | Items. React Router was removed in
`eb02c91` and is not coming back for this; the open tab is shell state. The
inactive tab is unmounted rather than hidden, because the recipe table holds a
live price subscription and a page of calculations that must not keep running
behind an invisible tab.

`ItemsPage` is a name field, a `source` select (`all` / `output` / `input`,
matching `ItemSource`), and a result grid over `/search/items`, paged 25 at a
time against the response's `total`. The query is debounced 300 ms and held
back below two characters: shorter queries match most of the catalogue, so the
server would page through thousands of rows to answer a keystroke.

Picking a row calls `/calc/{itemID}` — the batch endpoint is keyed on the IDs
the recipe table is showing, and a searched item is rarely among them — and
renders the same `CraftBreakdown`. That reuse is why the component takes a
`CalcPath` rather than a `RecipeRow`.

Each failure mode gets its own answer rather than an empty grid: too short a
query, nothing matched, a failed search (message plus Retry, via `QueryState`),
and an item with no priceable path.

## Testing

RTL and MSW, no mocking of internals. `CraftBreakdown` is covered directly
(step order, per-stage figures with no derived running total, server totals,
empty steps, incomplete paths) on fixtures copied from real `/calc/1673`
responses, so a future running total would fail rather than look plausible;
the hover is covered through `RecipesPage`, the tab switch
through `App`, and the search through `ItemsPage` including the debounce
threshold, the source filter reaching the query string, and every failure mode
above.
