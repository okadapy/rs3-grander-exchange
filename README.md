# RS3 Grander Exchange

A Grand Exchange profitability calculator for RuneScape 3: it walks a
recipe tree, prices every way of making an item, and ranks the catalogue
by experience per hour, coins per hour and coins per experience.

This repository is the whole product. The Go backend and the React
frontend were developed separately and are merged here with their
history intact, with an nginx in front so the browser sees one origin.

## Quick start

```sh
make
```

That builds everything and waits until it answers, then prints the URL.
Nothing else is needed — no `npm install`, no database setup, no
migrations to run by hand. First run takes a few minutes because it
compiles six Go services and a Vite bundle; afterwards it is seconds.

```sh
make down      # stop, keep the database
make destroy   # stop and delete the database and cache
make logs      # follow everything
make ps        # what is running
```

Port 80 is the default. If something already owns it:

```sh
PUBLIC_PORT=8000 PUBLIC_ORIGIN=http://localhost:8000 make
```

Both variables are needed together. Vite inlines the API origin into the
bundle at build time, so the frontend has to be told the address the
browser will actually use — the port alone is not enough.

## What runs

```
                        ┌─────────────┐
   browser ──── :80 ────│    nginx    │
                        └──────┬──────┘
                    /          │         everything else
                    │          │
            ┌───────▼──────┐   │   ┌─────────────┐
            │   frontend   │   └───│   gateway   │
            │ (static SPA) │       └──────┬──────┘
            └──────────────┘              │
             ┌──────────────┬─────────────┼──────────────┬───────────────┐
             │              │             │              │               │
      hiscore-service recipe-service ge-price-service calc-service realtime-service
             │              │             │              │               │
             └──────────────┴──────┬──────┴──────────────┴───────────────┘
                                   │
                            MySQL 8 + Redis 7
```

The browser only ever talks to nginx. The API is therefore same-origin
with the app, and the CORS layer behind it never fires in normal use.

nginx forwards the paths the gateway owns — `/calc`, `/recipes`,
`/prices`, `/hiscore`, `/search`, `/items`, `/auth`, `/chat`, `/ws`,
`/health`, `/swagger` — and serves the single-page app for everything
else. Those prefixes are listed one by one rather than hidden behind an
`/api` prefix, because they are the paths the published OpenAPI contract
names; rewriting them here would make the spec a lie.

Two timeouts are deliberately not the defaults. A cold ranking walks the
whole catalogue and is measured in seconds, so the proxy read timeout is
180s rather than 60s. Chat is a WebSocket that stays open, so `/ws` gets
an hour and no response buffering.

The backend services keep their own published ports (8080–8085, 3306,
6379) so they stay directly reachable while developing. Nothing in the
browser path depends on that.

## Layout

| Path | What it is |
|---|---|
| `backend/` | Go monorepo: six services, shared packages, migrations, OpenAPI |
| `web/` | Vite + React + MUI frontend, plus its copy of the API spec |
| `nginx/` | The front door config |
| `docker-compose.yml` | Includes the backend's compose file and adds the frontend and nginx |
| `Makefile` | The commands above |

`backend/` and `web/` were imported with `git subtree`, so their commit
history is present and they can still be pushed back upstream.

## The API contract

`backend/openapi/combined.yaml` is the single source of truth. The
per-service files under `backend/openapi/` are generated from it, and
the frontend consumes `web/combined.yaml`.

`make spec` regenerates the per-service files and copies the spec to the
frontend. `make up` runs it first, so a stale spec cannot reach a build.
Edit the combined file, never the generated ones.

## Development

```sh
make test                          # backend test suite
make rebuild SERVICE=calc-service  # rebuild one service after an edit
make -C backend fmt vet            # formatting and vet
```

The frontend image is a production build. For hot reload, run Vite
directly against the stack:

```sh
cd web && npm --prefix frontend install
VITE_API_URL=http://localhost npm --prefix frontend run dev
```

## Data

The recipe corpus is scraped from runescape.wiki and the prices come
from the Grand Exchange. The scrape runs on a schedule inside
`recipe-service`; a fresh database fills itself on first run, which
takes a while.

Rates are derived from game mechanics rather than a house constant
wherever the data allows: tick costs from the recipe infobox, smelting
speed from the level thresholds, and anvil forging from a progress and
heat simulation. `aph_source` on every result says which of those
produced the number, and `default` means it is a house assumption rather
than a measured rate.

## Known gaps

These are real and worth knowing before trusting a number:

- **Gathering is not modelled.** No recipe produces ore, logs or raw
  fish, so any chain bottoms out at buying its raw materials. A bronze
  bar shows copper and tin as bought inputs; it cannot show mining them.
- **Deep chains are truncated.** The buy-versus-craft enumeration is
  bounded at 64 combinations per node. Provably worse branches are
  pruned first, but on a long upgrade chain the bound still bites and
  the answer is "no worse than buying everything" rather than "the
  best there is". The response does not currently say when this
  happened.
- **One ranking is expensive.** A cold `GET /calc/top` makes on the
  order of 17,000 upstream requests and takes about ten seconds. It is
  cached for ten minutes, but a request with an unresolvable player is
  never cached and never refused.
- **A named player the hiscores cannot resolve degrades silently.** The
  ranking comes back unfiltered, which is not what the route promises.
