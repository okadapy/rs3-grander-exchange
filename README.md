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

## Configuration

Each service reads `config.yaml`, overridable by environment
(`RS3_DB_HOST`, `RS3_DB_PASSWORD`, `RS3_REDIS_ADDR`, `RS3_JWT_SECRET`).

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
tables. One schema change needs a manual step, since AutoMigrate adds
columns but never drops them:

```bash
docker compose exec -T mysql mysql -uroot -proot \
  < scripts/migrations/001_price_snapshot_single_price.sql
```

This collapses `buy_price`/`sell_price` into `price`. Both columns only
ever held the same guide price; presenting them as a pair implied a
spread that does not exist. The script is idempotent.
