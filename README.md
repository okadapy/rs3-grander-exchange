# RS3 Market Backend

Grand Exchange market analysis for RuneScape 3: scraped crafting
recipes, GE price history, tradeability signals, and a profitability
calculator that walks a recipe tree and prices every way to produce an
item.

```
docker compose up --build
```

| Service            | Port | Role                                              |
|--------------------|------|---------------------------------------------------|
| `gateway`          | 8080 | Single public origin; routing, CORS, rate limiting |
| `hiscore-service`  | 8081 | On-demand RS3 hiscores                            |
| `recipe-service`   | 8082 | Wiki recipe scraper, item IDs, GE buy limits      |
| `ge-price-service` | 8083 | Weirdgloop price poller, history, liquidity       |
| `calc-service`     | 8084 | ROI / GP-h / XP-h / GP-XP across production paths |
| `realtime-service` | 8085 | WebSocket price push and chat                     |

- API docs: <http://localhost:8080/swagger>
- Generate a frontend client from **`openapi/combined.yaml`**.

## Reading the data

Three things decide whether a number from this API is actionable. All
three are exposed rather than hidden, because each of them can turn a
large headline margin into nothing.

**The Grand Exchange publishes one price, not two.** RS3 has no public
instant-buy / instant-sell feed, so `PriceSnapshot` carries a single
`price` — the guide price. There is no bid/ask spread anywhere in this
data. `calc-service` applies an explicit spread assumption
(`market.spread_pct`, default 2%) to model the real cost of a round
trip, and reports it back in `CalcResult.assumptions`. A GP/h figure
from this API is not meaningful without that block beside it.

**Buy limits usually bind before clicking speed does.** Every item has
a 4-hour GE buy limit. A craft netting 500 GP on an item limited to 100
per 4 hours yields at most 50k GP per 4 hours however fast you click.
`CalcPath.throughput` names the binding input and the GP/h that limit
actually permits — frequently an order of magnitude under
`gp_per_hour`. Prefer it when present.

**A margin is not a trading signal.** A 5M GP/h margin on an item that
trades twice a day looks identical, in a plain profit listing, to the
same margin on one moving millions of units. `GET /prices/stats/{itemID}`
returns a liquidity `score` and `tier` built from traded volume, how
recently the price moved, and how often it moves. Check `observations`
to see how much data the score rests on.

**Ranking the catalogue is a route, not a client-side loop.**
`GET /calc/top?metric=gp_per_hour` walks every priceable recipe —
around 5800 of them — and returns the best path for each, sorted by
one metric. `metric` is required (`xp_per_hour`, `gp_per_hour` or
`gp_per_xp`) and has no default: asking for a ranking without saying of
what is a mistake worth a 400, not a guess. `metric=gp_per_hour` sorts
on `gp_per_hour_limited`, the same throughput-bound rate described
above — sorting on the raw `gp_per_hour` would put a godsword nobody
forges six hundred times an hour above the things people actually
craft.

Two smaller honesty flags:

- `CalcPath.complete` — `false` means an input had no price and was
  costed at zero, so the money figures are upper bounds rather than
  estimates. Excluded by default; pass `include_incomplete=true` to see
  them.
- `aph_source` — `default` means actions-per-hour is a house assumption,
  not a measured rate. The wiki rarely publishes one, so this is the
  common case. Good for ranking recipes against each other; not a
  literal throughput.

## Data sources

| Source | Used for |
|--------|----------|
| `api.weirdgloop.org/exchange/history/rs` | Guide prices and traded volume |
| `runescape.wiki` MediaWiki API | Recipe pages (via `embeddedin` on the recipe infobox) |
| `Module:GEIDs/data.json` | Item name to GE item ID |
| `Module:GELimits/data.json` | 4-hour buy limits |
| `secure.runescape.com` hiscores | Player skill levels |

Reference data refreshes daily. Item IDs are fetched before buy limits,
because limits are keyed by name and need the ID map to become useful.

### Item IDs

A recipe names its inputs; prices are keyed by GE item ID. Resolving one
to the other runs in three passes, in this order:

1. recipe outputs, from the wiki item ID map
2. inputs, from the outputs of recipes that produce them — a bar is an
   input to one recipe and the output of another
3. whatever inputs are left, from the item ID map — the gathered
   materials no recipe produces

Order matters: pass 2 matches input names against output names, so it
only sees what pass 1 has already resolved.

Resolution runs after every scrape, not on a schedule of its own.
Upserting a recipe deletes its inputs and inserts them fresh with no
item ID, so a scrape invalidates exactly what resolution produces; the
two are one cycle in `scraper.RunScrapeCycle`.

About 3.4k of 14.9k inputs never resolve, and that is correct rather
than a gap to close. They are untradeable by design — Daemonheim items
(`Thread (Dungeoneering)`, and the Daemonheim-only hides and tree
branches that carry no suffix), Summoning charms, minigame currencies
(`Sacred clay`, `* fragments`) and `Coins`. They have no GE ID because
they have no GE price, which is what `CalcPath.complete` is reporting
when it comes back `false`.

## Configuration

Each service reads `config.yaml`, overridable by environment
(`RS3_DB_HOST`, `RS3_DB_PASSWORD`, `RS3_REDIS_ADDR`, `RS3_JWT_SECRET`).

SQL logging is configured separately from the service's own
`log_level`, under `mysql.log_level` (`silent`, `error`, `warn`,
`info`; default `warn`). Tying the two together meant that asking a
service for debug logs also asked GORM to print every statement, and a
scrape upserting thousands of recipes then buried its own progress under
hundreds of thousands of query lines. At `warn` you still get errors and
any statement slower than 200ms.

The trading assumptions live under `market:` in
`calc-service/config.yaml`. They are modelling choices rather than
scraped facts, which is why they are configurable and echoed in every
response:

```yaml
market:
  spread_pct: 2.0          # assumed round-trip buy/sell gap
  tax_pct: 2.0             # GE sales tax on the whole sale
  tax_cap_per_item: 5000000
  tax_exempt_below: 0
  default_actions_per_hour: 600
```

Before deploying anywhere real, change `jwt.secret` in
`realtime-service/config.yaml` and narrow `cors.allowed_origins` in
`gateway/config.yaml`.

## OpenAPI

`openapi/combined.yaml` (OpenAPI 3.1) describes the whole gateway
surface and is the file to generate clients from:

```bash
npx openapi-typescript openapi/combined.yaml -o src/api.ts
```

The per-service specs are **generated** from it — run `make openapi`
after editing the combined file, never edit them by hand. Each service
has a contract test asserting its routes and its spec agree, so drift
fails the build rather than surfacing as a 404 in the frontend.

## Development

```bash
make test          # unit tests
make race          # tests under the race detector
make vet
make build         # binaries into bin/
make openapi       # regenerate per-service specs
make lint-openapi  # validate every spec (needs network)
```

## Migrations

`scripts/init.sql` creates the databases; GORM AutoMigrate handles
tables. Anything AutoMigrate cannot do — it adds columns and tables but
never drops them — is a numbered script in `scripts/migrations/`, to be
applied in order against an existing database:

```bash
for f in scripts/migrations/*.sql; do
  docker compose exec -T mysql mysql -uroot -proot < "$f"
done
```

Every script is idempotent, so re-running the set is safe.

- **001** collapses `buy_price`/`sell_price` into `price`. Both columns
  only ever held the same guide price; presenting them as a pair implied
  a spread that does not exist. Also drops `daily_changes`, a table
  AutoMigrate created and nothing ever wrote to.
- **002** drops `ge_id_maps`. GORM derived that name from the `GEIDMap`
  struct before a `TableName` method pinned it to `geid_maps`, and left
  the original behind holding a stale copy of the item ID map.
- **003** indexes `recipes.output_item_name`. ID resolution joins every
  input name against it, unindexed on both sides, which took ~13s with
  nothing to write and ~44s on a run that resolved rows. It used to run
  once a day where nobody noticed; it now runs after every scrape.
- **004** adds `recipes.ticks` and `recipes.facility`. The tick cost
  drives the craft-rate model; the facility separates smelting at a
  furnace from forging at an anvil, which share a skill but not a rate.

## Local operations

Each service exposes dev-only endpoints on its own port. They are not
routed through the gateway and are not in `combined.yaml`.

```bash
curl -X POST    localhost:8082/internal/scrape          # scrape + resolve now
curl            localhost:8082/internal/input-id-stats  # resolution coverage
curl -X POST    localhost:8082/internal/backfill-inputs
curl            "localhost:8082/internal/dump?page=Sapphire%20necklace"
curl -X POST    localhost:8083/internal/poll            # price cycle now
curl -X DELETE  localhost:8083/internal/poll/lock       # after a crashed poller
```

A full scrape takes about three minutes and a price cycle about half a
minute, so neither is instant — watch the logs rather than the response, which
returns 202 immediately. The poller takes a Redis lock so two instances
cannot double-poll; `/internal/poll/lock` exists for when a crash leaves
that lock behind.
