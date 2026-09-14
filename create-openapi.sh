#!/usr/bin/env bash
# ============================================================
#  add-openapi.sh — adds OpenAPI 3.0 specs + Swagger UI to
#  each service. Run from the project root.
# ============================================================
set -euo pipefail

if [ ! -f "go.mod" ] || [ ! -d "hiscore-service" ]; then
  echo "Run this from the project root (where go.mod lives)." >&2
  exit 1
fi

mkdir -p openapi shared/swagger

# ============================================================
#  1. OPENAPI SPECS
# ============================================================

cat > openapi/hiscore-service.yaml <<'YAML'
openapi: 3.0.3
info:
  title: Hiscore Service
  version: 1.0.0
  description: |
    On-demand fetch of RuneScape 3 player hiscores. Results are cached
    with a configurable TTL (default 5 minutes). If the upstream Jagex
    endpoint fails and a stale row exists, the stale row is returned.
servers:
  - url: http://localhost:8081
    description: Local development
tags:
  - name: health
    description: Liveness / readiness
  - name: hiscore
    description: Player hiscore lookups
paths:
  /health:
    get:
      tags: [health]
      summary: Liveness probe
      operationId: getHealth
      responses:
        '200':
          description: Service is up
          content:
            application/json:
              schema:
                type: object
                properties:
                  status: { type: string, example: ok }
  /hiscore/{name}:
    get:
      tags: [hiscore]
      summary: Get hiscores for a player
      operationId: getHiscore
      description: |
        Fetches skill levels, XP and ranks. If the cached copy is older
        than the configured TTL, it is refreshed from the official
        Jagex endpoint.
      parameters:
        - name: name
          in: path
          required: true
          description: RuneScape display name
          schema: { type: string, example: Zezima }
        - name: mode
          in: query
          description: Account type
          schema:
            type: string
            enum: [normal, ironman, hardcore]
            default: normal
      responses:
        '200':
          description: Player hiscores
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Player' }
        '404':
          description: Player not found or upstream error
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Error' }
components:
  schemas:
    Player:
      type: object
      properties:
        id:         { type: integer, format: int64 }
        name:       { type: string }
        mode:       { type: string, enum: [normal, ironman, hardcore] }
        fetched_at: { type: string, format: date-time }
        skills:
          type: array
          items: { $ref: '#/components/schemas/PlayerSkill' }
    PlayerSkill:
      type: object
      properties:
        id:        { type: integer, format: int64 }
        player_id: { type: integer, format: int64 }
        skill:     { type: string, example: Crafting }
        level:     { type: integer }
        xp:        { type: integer, format: int64 }
        rank:      { type: integer, format: int64 }
    Error:
      type: object
      properties:
        error: { type: string }
YAML

cat > openapi/recipe-service.yaml <<'YAML'
openapi: 3.0.3
info:
  title: Recipe Service
  version: 1.0.0
  description: |
    Scrapes, stores and serves RuneScape 3 crafting recipes and their
    dependency trees. Supports filtering by skill and by a character's
    level in that skill. Recipe trees are cached in Redis for 1 hour.
servers:
  - url: http://localhost:8082
    description: Local development
tags:
  - { name: health }
  - { name: recipes }
  - { name: internal, description: Intended only for other services }
paths:
  /health:
    get:
      tags: [health]
      summary: Liveness probe
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  status: { type: string, example: ok }
  /recipes:
    get:
      tags: [recipes]
      summary: List recipes with optional filters
      operationId: listRecipes
      parameters:
        - name: skill
          in: query
          schema: { type: string, example: Crafting }
        - name: level
          in: query
          description: Maximum level requirement (inclusive)
          schema: { type: integer, example: 75 }
      responses:
        '200':
          description: List of recipes
          content:
            application/json:
              schema:
                type: object
                properties:
                  count:   { type: integer }
                  recipes:
                    type: array
                    items: { $ref: '#/components/schemas/Recipe' }
  /recipes/{itemID}:
    get:
      tags: [recipes]
      summary: Full recipe tree for an output item
      operationId: getRecipeTreeAlias
      parameters:
        - name: itemID
          in: path
          required: true
          schema: { type: integer, format: int64 }
      responses:
        '200':
          description: Recipe tree
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Tree' }
        '404':
          description: No recipe found
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Error' }
  /recipes/{itemID}/tree:
    get:
      tags: [recipes]
      summary: Recipe dependency tree (canonical endpoint)
      operationId: getRecipeTree
      parameters:
        - name: itemID
          in: path
          required: true
          schema: { type: integer, format: int64 }
      responses:
        '200':
          description: Recipe tree
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Tree' }
        '404':
          description: No recipe found
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Error' }
  /internal/all-item-ids:
    get:
      tags: [internal]
      summary: All distinct output item IDs
      description: Used by ge-price-service to know which items to poll.
      operationId: getAllItemIDs
      responses:
        '200':
          description: Item IDs
          content:
            application/json:
              schema:
                type: object
                properties:
                  item_ids:
                    type: array
                    items: { type: integer, format: int64 }
components:
  schemas:
    Recipe:
      type: object
      properties:
        id:               { type: integer, format: int64 }
        name:             { type: string }
        output_item_id:   { type: integer, format: int64 }
        output_item_name: { type: string }
        output_qty:       { type: integer }
        skill:            { type: string, example: Crafting }
        level_req:        { type: integer }
        xp_per_action:    { type: number, format: double }
        actions_per_hour: { type: integer }
        members:          { type: boolean }
        source:           { type: string, example: runescape.wiki }
        inputs:
          type: array
          items: { $ref: '#/components/schemas/RecipeInput' }
    RecipeInput:
      type: object
      properties:
        id:              { type: integer, format: int64 }
        recipe_id:       { type: integer, format: int64 }
        item_id:         { type: integer, format: int64 }
        item_name:       { type: string }
        quantity:        { type: integer }
        is_intermediate: { type: boolean }
    Tree:
      type: object
      properties:
        root: { $ref: '#/components/schemas/Node' }
    Node:
      type: object
      properties:
        recipe:   { $ref: '#/components/schemas/Recipe' }
        children:
          type: array
          items: { $ref: '#/components/schemas/Node' }
        depth:    { type: integer }
    Error:
      type: object
      properties:
        error: { type: string }
YAML

cat > openapi/ge-price-service.yaml <<'YAML'
openapi: 3.0.3
info:
  title: GE Price Service
  version: 1.0.0
  description: |
    Polls the Weirdgloop exchange-history API for RuneScape 3 item prices
    and volumes. Publishes per-item updates to Redis on channel
    `prices:{item_id}` for realtime fan-out. Respects the API rate limit
    by throttling between batches.
servers:
  - url: http://localhost:8083
    description: Local development
tags:
  - { name: health }
  - { name: prices }
paths:
  /health:
    get:
      tags: [health]
      summary: Liveness probe
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  status: { type: string, example: ok }
  /prices/latest:
    get:
      tags: [prices]
      summary: Latest snapshot for multiple items
      operationId: getLatestPrices
      parameters:
        - name: ids
          in: query
          required: true
          description: Comma-separated item IDs
          schema: { type: string, example: "1656,2357,1623" }
      responses:
        '200':
          description: Price snapshots
          content:
            application/json:
              schema:
                type: object
                properties:
                  count:  { type: integer }
                  prices:
                    type: array
                    items: { $ref: '#/components/schemas/PriceSnapshot' }
        '400':
          description: Missing ids
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Error' }
  /prices/history/{itemID}:
    get:
      tags: [prices]
      summary: Time-series history for one item
      operationId: getPriceHistory
      parameters:
        - name: itemID
          in: path
          required: true
          schema: { type: integer, format: int64 }
        - name: from
          in: query
          description: RFC3339 start time (default 7 days ago)
          schema: { type: string, format: date-time }
        - name: to
          in: query
          description: RFC3339 end time (default now)
          schema: { type: string, format: date-time }
      responses:
        '200':
          description: History
          content:
            application/json:
              schema:
                type: object
                properties:
                  count: { type: integer }
                  history:
                    type: array
                    items: { $ref: '#/components/schemas/PriceSnapshot' }
  /prices/change/{itemID}:
    get:
      tags: [prices]
      summary: 24-hour price change
      operationId: getPriceChange
      parameters:
        - name: itemID
          in: path
          required: true
          schema: { type: integer, format: int64 }
      responses:
        '200':
          description: Change and both endpoints
          content:
            application/json:
              schema:
                type: object
                properties:
                  item_id:      { type: integer, format: int64 }
                  change_pct:   { type: number, format: double }
                  current:      { $ref: '#/components/schemas/PriceSnapshot' }
                  previous_24h: { $ref: '#/components/schemas/PriceSnapshot' }
components:
  schemas:
    PriceSnapshot:
      type: object
      properties:
        id:         { type: integer, format: int64 }
        item_id:    { type: integer, format: int64 }
        ts:         { type: string, format: date-time }
        buy:        { type: integer, format: int64 }
        sell:       { type: integer, format: int64 }
        volume:     { type: integer, format: int64 }
        normalized: { type: boolean }
    Error:
      type: object
      properties:
        error: { type: string }
YAML

cat > openapi/calc-service.yaml <<'YAML'
openapi: 3.0.3
info:
  title: Calc Service
  version: 1.0.0
  description: |
    Calculates ROI, GP/h, XP/h and GP/XP for **every** possible crafting
    path to produce an item, including intermediate steps. Results are
    sorted by GP/h descending and cached for 10 minutes.
    
    The `aph` query parameter overrides the recipe's actions-per-hour
    for the whole path; if omitted the stored value is used, falling
    back to a per-skill default.
servers:
  - url: http://localhost:8084
    description: Local development
tags:
  - { name: health }
  - { name: calc }
paths:
  /health:
    get:
      tags: [health]
      summary: Liveness probe
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  status: { type: string, example: ok }
  /calc/{itemID}:
    get:
      tags: [calc]
      summary: Profitability paths for one item
      operationId: calculateItem
      parameters:
        - name: itemID
          in: path
          required: true
          schema: { type: integer, format: int64 }
        - name: aph
          in: query
          description: Override actions-per-hour for all steps (0 = use recipe value)
          schema: { type: integer, example: 800 }
        - name: normalized
          in: query
          description: Use averaged prices instead of instant buy/sell
          schema: { type: boolean, default: false }
        - name: player
          in: query
          description: Optional player name to filter by skill level
          schema: { type: string }
        - name: mode
          in: query
          schema: { type: string, enum: [normal, ironman, hardcore], default: normal }
      responses:
        '200':
          description: Calculation result
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Result' }
        '500':
          description: Calculation error
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Error' }
  /calc/batch:
    get:
      tags: [calc]
      summary: Calculate for multiple items
      operationId: calculateBatch
      parameters:
        - name: ids
          in: query
          required: true
          schema: { type: string, example: "1656,2357" }
        - name: aph
          in: query
          schema: { type: integer }
        - name: normalized
          in: query
          schema: { type: boolean }
      responses:
        '200':
          description: Map of item ID to Result or error
          content:
            application/json:
              schema:
                type: object
                additionalProperties: {}
components:
  schemas:
    Result:
      type: object
      properties:
        item_id: { type: integer, format: int64 }
        paths:
          type: array
          items: { $ref: '#/components/schemas/PathResult' }
        cached:  { type: boolean }
    PathResult:
      type: object
      properties:
        path:
          type: array
          items: { type: string }
        steps:
          type: array
          items: { $ref: '#/components/schemas/Step' }
        profit_per_trade: { type: number, format: double }
        gp_per_hour:      { type: number, format: double }
        xp_per_hour:      { type: number, format: double }
        gp_per_xp:        { type: number, format: double }
        roi_pct:          { type: number, format: double }
        total_hours:      { type: number, format: double }
        total_xp:         { type: number, format: double }
        buy_cost:         { type: number, format: double }
        sell_revenue:     { type: number, format: double }
        tax_paid:         { type: number, format: double }
        actions_per_hour: { type: integer }
    Step:
      type: object
      properties:
        recipe:       { type: string }
        skill:        { type: string }
        level_req:    { type: integer }
        xp:           { type: number, format: double }
        aph:          { type: integer }
        buy_cost:     { type: number, format: double }
        sell_revenue: { type: number, format: double }
        profit:       { type: number, format: double }
    Error:
      type: object
      properties:
        error: { type: string }
YAML

cat > openapi/realtime-service.yaml <<'YAML'
openapi: 3.0.3
info:
  title: Realtime Service
  version: 1.0.0
  description: |
    WebSocket gateway for per-item price updates and a single global chat
    room.

    ## WebSocket

    Connect to `/ws?token=<JWT>`. Client frames are JSON:

    - `{"action": "subscribe", "item_ids": [1,2,3]}`
    - `{"action": "unsubscribe", "item_ids": [2]}`
    - `{"action": "chat", "body": "hello"}`

    Server frames are JSON:

    - `{"type": "price", "payload": <PriceSnapshot>}`
    - `{"type": "chat",  "payload": <ChatMessage>}`
    - `{"type": "error", "payload": {"message": "..."}}`

    Chat is rate-limited to one message every 3 seconds per user.
servers:
  - url: http://localhost:8085
    description: Local development
tags:
  - { name: health }
  - { name: auth }
  - { name: chat }
  - { name: ws }
paths:
  /health:
    get:
      tags: [health]
      summary: Liveness probe
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  status: { type: string, example: ok }
  /auth/register:
    post:
      tags: [auth]
      summary: Register a chat user
      operationId: registerUser
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/Credentials' }
      responses:
        '200':
          description: Registered
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:       { type: integer, format: int64 }
                  username: { type: string }
        '400':
          description: Validation error
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Error' }
  /auth/login:
    post:
      tags: [auth]
      summary: Log in and receive a JWT
      operationId: loginUser
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/Credentials' }
      responses:
        '200':
          description: JWT
          content:
            application/json:
              schema:
                type: object
                properties:
                  token: { type: string }
        '401':
          description: Invalid credentials
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Error' }
  /chat/history:
    get:
      tags: [chat]
      summary: Recent chat messages (chronological)
      operationId: getChatHistory
      parameters:
        - name: limit
          in: query
          schema: { type: integer, default: 50 }
        - name: before
          in: query
          description: RFC3339 timestamp — return messages before this
          schema: { type: string, format: date-time }
      responses:
        '200':
          description: Messages
          content:
            application/json:
              schema:
                type: object
                properties:
                  messages:
                    type: array
                    items: { $ref: '#/components/schemas/ChatMessage' }
  /ws:
    get:
      tags: [ws]
      summary: WebSocket upgrade
      operationId: connectWebSocket
      description: |
        Upgrade to WebSocket. Authenticate with `?token=<JWT>` obtained
        from `/auth/login`. All frames are JSON — see service description.
      parameters:
        - name: token
          in: query
          required: true
          schema: { type: string }
      responses:
        '101':
          description: Switching Protocols
        '401':
          description: Invalid or missing token
components:
  schemas:
    Credentials:
      type: object
      required: [username, password]
      properties:
        username: { type: string, minLength: 3, maxLength: 32 }
        password: { type: string, minLength: 6 }
    ChatMessage:
      type: object
      properties:
        id:         { type: integer, format: int64 }
        user_id:    { type: integer, format: int64 }
        username:   { type: string }
        body:       { type: string }
        created_at: { type: string, format: date-time }
    Error:
      type: object
      properties:
        error: { type: string }
YAML

# ============================================================
#  2. SHARED SWAGGER UI PACKAGE
# ============================================================

cat > shared/swagger/swagger.go <<'GO'
// Package swagger serves an OpenAPI spec file plus a Swagger UI page
// that renders it. It has no dependencies beyond gin and the standard
// library, so any service can mount it with a single call.
package swagger

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// htmlTemplate is a self-contained Swagger UI page that loads Swagger UI
// from unpkg and points at /swagger/spec.yaml.
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s — API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    html, body { margin: 0; padding: 0; background: #fafafa; }
    #swagger-ui { font-family: -apple-system, system-ui, sans-serif; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    window.addEventListener('load', function () {
      window.ui = SwaggerUIBundle({
        url: '%s',
        dom_id: '#swagger-ui',
        deepLinking: true,
        displayOperationId: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: 'BaseLayout'
      });
    });
  </script>
</body>
</html>`

// Register mounts:
//
//	GET /swagger                  -> redirect to /swagger/index.html
//	GET /swagger/index.html       -> Swagger UI page
//	GET /swagger/spec.yaml        -> raw OpenAPI spec
//
// specPath is resolved at request time so missing specs produce a clear
// 404 rather than a crash at startup.
func Register(r *gin.Engine, specPath, title string) {
	if title == "" {
		title = "API"
	}
	if specPath == "" {
		specPath = "openapi/openapi.yaml"
	}

	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/index.html", func(c *gin.Context) {
		body := fmt.Sprintf(htmlTemplate, title, "/swagger/spec.yaml")
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(body))
	})
	r.GET("/swagger/spec.yaml", func(c *gin.Context) {
		b, err := os.ReadFile(specPath)
		if err != nil {
			c.String(http.StatusNotFound,
				"openapi spec not found at %q: %v", specPath, err)
			return
		}
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", b)
	})
}
GO

# ============================================================
#  3. PATCH Dockerfiles to copy the openapi directory
# ============================================================

for svc in hiscore-service recipe-service ge-price-service calc-service realtime-service; do
  f="$svc/Dockerfile"
  if ! grep -q '^COPY openapi' "$f"; then
    perl -0777 -pi -e \
      's|(COPY [a-z-]+/config.yaml /app/config.yaml\n)|$1COPY openapi /app/openapi\n|' \
      "$f"
  fi
  echo "  patched $f"
done

# ============================================================
#  4. PATCH each service's main.go:
#     - add swagger import
#     - register /swagger routes
# ============================================================

patch_main() {
  local svc="$1"
  local f="$svc/main.go"

  # 4a. add import after shared/middleware
  if ! grep -q 'shared/swagger' "$f"; then
    perl -0777 -pi -e \
      's|(\t"github.com/rs3-market/backend/shared/middleware"\n)|\1\t"github.com/rs3-market/backend/shared/swagger"\n|' \
      "$f"
  fi

  # 4b. register swagger after the route handler's Register(r) call.
  # Matches both `h.Register(r)` and `hd.Register(r)`.
  if ! grep -q 'swagger.Register(' "$f"; then
    perl -0777 -pi -e \
      's|(\n\th[d]?\.Register\(r\)\n)|\1\tswagger.Register(r, "openapi/"+cfg.ServiceName+".yaml", cfg.ServiceName)\n|' \
      "$f"
  fi

  echo "  patched $f"
}

for svc in hiscore-service recipe-service ge-price-service calc-service realtime-service; do
  patch_main "$svc"
done

# ============================================================
#  5. VERIFY
# ============================================================

echo ""
echo ">>> Formatting modified Go files"
gofmt -w hiscore-service/main.go recipe-service/main.go \
         ge-price-service/main.go calc-service/main.go \
         realtime-service/main.go shared/swagger/swagger.go

echo ""
echo ">>> Checking patches landed"
for svc in hiscore-service recipe-service ge-price-service calc-service realtime-service; do
  printf "  %-20s import:%s register:%s dockerfile:%s\n" \
    "$svc" \
    "$(grep -c 'shared/swagger' "$svc/main.go")" \
    "$(grep -c 'swagger.Register(' "$svc/main.go")" \
    "$(grep -c 'COPY openapi' "$svc/Dockerfile")"
done

echo ""
echo "============================================================"
echo " OpenAPI + Swagger UI installed."
echo ""
echo " Specs:      ./openapi/*.yaml"
echo " UI package: ./shared/swagger/swagger.go"
echo ""
echo " Endpoints per service (replace PORT):"
echo "   http://localhost:PORT/swagger              <- redirect"
echo "   http://localhost:PORT/swagger/index.html   <- UI"
echo "   http://localhost:PORT/swagger/spec.yaml    <- raw spec"
echo ""
echo " Rebuild with: make dev"
echo "============================================================"

