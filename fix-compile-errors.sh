#!/usr/bin/env bash
# ============================================================
#  fix-compile-errors.sh
#  Fixes three issues:
#    1. recipe-service: unused models import in scraper.go
#    2. realtime-service: unexported `subscribed` field on hub.Client
#    3. calc-service: cache import name collision in main.go
#  Run from project root.
# ============================================================
set -euo pipefail

if [ ! -f go.mod ]; then
  echo "Run from project root (where go.mod lives)." >&2
  exit 1
fi

# ---------- FIX 1: recipe-service scraper.go ----------
# Remove the unused `shared/models` import. The parser.go file still
# uses models; scraper.go itself doesn't reference it directly.
python3 - <<'PY'
import pathlib, re
p = pathlib.Path("recipe-service/internal/scraper/scraper.go")
s = p.read_text()
# Remove the models import line
s = re.sub(r'\n\s*"github\.com/rs3-market/backend/shared/models"\n', '\n', s)
p.write_text(s)
print("  fixed:", p)
PY

# ---------- FIX 2: realtime-service hub.Client ----------
# Add a constructor so handler package can build a Client without
# touching the unexported `subscribed` map. Also export a small helper
# so the handler can't accidentally leave it nil.
cat > realtime-service/internal/hub/client.go <<'GO'
package hub

import "github.com/gorilla/websocket"

// NewClient builds a fully-initialized Client. The handler package
// must use this rather than a struct literal, because `subscribed` is
// unexported.
func NewClient(h *Hub, conn *websocket.Conn, userID uint, username string) *Client {
	return &Client{
		Hub:        h,
		Conn:       conn,
		Send:       make(chan []byte, 64),
		UserID:     userID,
		Username:   username,
		subscribed: map[int64]struct{}{},
	}
}
GO

# Replace the struct literal in ws.go with a call to NewClient.
python3 - <<'PY'
import pathlib, re
p = pathlib.Path("realtime-service/internal/handler/ws.go")
s = p.read_text()

old = '''	client := &hub.Client{
		Hub: h, Conn: conn, Send: make(chan []byte, 64),
		UserID: userID, Username: username,
		subscribed: map[int64]struct{}{},
	}'''
new = '''	client := hub.NewClient(h, conn, userID, username)'''

if old in s:
    s = s.replace(old, new)
    p.write_text(s)
    print("  fixed:", p)
else:
    print("  WARN: pattern not found in", p)
    # Show the offending block so you can inspect
    for i, line in enumerate(s.splitlines(), 1):
        if "hub.Client{" in line or "subscribed:" in line:
            print(f"    line {i}: {line}")
PY

# ---------- FIX 3: calc-service main.go ----------
# The main.go imported both:
#     "github.com/rs3-market/backend/shared/cache"
#     "github.com/rs3-market/backend/calc-service/internal/cache"
# Both package name `cache`, hence the collision. Alias the internal one.
cat > calc-service/main.go <<'GO'
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/logger"
	"github.com/rs3-market/backend/shared/middleware"

	calccache "github.com/rs3-market/backend/calc-service/internal/cache"
	"github.com/rs3-market/backend/calc-service/internal/client"
	"github.com/rs3-market/backend/calc-service/internal/handler"
	"github.com/rs3-market/backend/calc-service/internal/service"
)

func main() {
	cfgPath := getenv("RS3_CONFIG", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel, cfg.Env == "local")
	defer log.Sync()

	c := cache.New(cfg.Redis)
	if err := c.Ping(); err != nil {
		log.Fatal("redis", zap.Error(err))
	}

	rc := client.NewRecipeClient(cfg.Services.RecipeURL)
	pc := client.NewPriceClient(cfg.Services.PriceURL)
	hc := client.NewHiscoreClient(cfg.Services.HiscoreURL)
	cc := calccache.New(c)

	svc := service.New(log, rc, pc, hc, cc)
	h := handler.New(svc, log)

	if cfg.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), middleware.Recovery(log), middleware.RequestIDAndLog(log))
	h.Register(r)

	srv := &http.Server{Addr: ":" + itoa(cfg.Port), Handler: r}
	go func() {
		log.Info("starting", zap.String("service", cfg.ServiceName), zap.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("listen", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func itoa(i int) string {
	if i == 0 {
		return "8080"
	}
	b := [12]byte{}
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}
GO
echo "  rewrote: calc-service/main.go"

# ---------- FORMAT + LOCAL BUILD CHECK ----------
echo ""
echo ">>> gofmt"
gofmt -w recipe-service/internal/scraper/scraper.go \
         realtime-service/internal/hub/client.go \
         realtime-service/internal/handler/ws.go \
         calc-service/main.go

echo ""
echo ">>> verifying each service compiles locally"
for svc in hiscore-service recipe-service ge-price-service calc-service realtime-service gateway; do
  if ( cd "$svc" && go build -o /tmp/_build_check . 2>/tmp/_build_err ); then
    printf "  ✓ %s\n" "$svc"
    rm -f /tmp/_build_check
  else
    printf "  ✗ %s\n" "$svc"
    sed 's/^/      /' /tmp/_build_err
  fi
done

echo ""
echo "============================================================"
echo " Fixes applied. If all six services show ✓ above, run:"
echo "   make dev"
echo "============================================================"
