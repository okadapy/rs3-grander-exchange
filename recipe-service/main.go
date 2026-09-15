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
	"github.com/rs3-market/backend/shared/db"
	"github.com/rs3-market/backend/shared/logger"
	"github.com/rs3-market/backend/shared/middleware"

	"github.com/rs3-market/backend/recipe-service/internal/handler"
	"github.com/rs3-market/backend/recipe-service/internal/repository"
	"github.com/rs3-market/backend/recipe-service/internal/scraper"
	"github.com/rs3-market/backend/recipe-service/internal/service"
)

func main() {
	cfgPath := getenv("RS3_CONFIG", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		panic(err)
	}

	log := logger.New(cfg.LogLevel, cfg.Env == "local")
	defer log.Sync()

	gdb, err := db.Open(cfg.MySQL, cfg.LogLevel)
	if err != nil {
		log.Fatal("db open", zap.Error(err))
	}

	c := cache.New(cfg.Redis)
	if err := c.Ping(); err != nil {
		log.Fatal("redis", zap.Error(err))
	}

	repo := repository.New(gdb)
	if err := repo.Migrate(); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}

	// Backfill input item IDs from matching output names.
	// Anything craftable (bars, planks, leathers) gets its ID
	// resolved this way so the price poller can fetch its cost.
	if n, err := repo.BackfillInputItemIDs(); err != nil {
		log.Warn("backfill input ids", zap.Error(err))
	} else if n > 0 {
		log.Info("backfilled input item ids", zap.Int64("rows", n))
	}
	// Fetch the wiki GE item ID map and resolve raw-material
	// inputs that BackfillInputItemIDs couldn't handle. Runs once
	// at startup; the scraper loop refreshes it daily.
	// Reference data refresh: GE item IDs first, then buy limits (which
	// are keyed by name and need the ID map to become useful). Both are
	// cheap single-file downloads, so this reruns daily rather than only
	// at startup — the game adds and renames tradeable items.
	go func() {
		time.Sleep(3 * time.Second)
		for {
			if err := scraper.FetchGEIDs(repo, log); err != nil {
				log.Warn("GEIDs fetch failed", zap.Error(err))
			} else if total, resolved, err := repo.InputIDStats(); err == nil {
				log.Info("input id coverage after GEIDs",
					zap.Int64("total", total),
					zap.Int64("resolved", resolved),
					zap.Int64("missing", total-resolved))
			}
			if err := scraper.FetchBuyLimits(repo, log); err != nil {
				log.Warn("buy limits fetch failed", zap.Error(err))
			}
			time.Sleep(24 * time.Hour)
		}
	}()

	if total, resolved, err := repo.InputIDStats(); err == nil {
		log.Info("input id coverage",
			zap.Int64("total", total),
			zap.Int64("resolved", resolved),
			zap.Int64("missing", total-resolved))
	}

	// ---- build the scraper BEFORE the HTTP server so the background
	// loop and the admin handler share one instance.
	sc := scraper.New(repo, log)

	// ---- background scrape loop ----
	// Retries on failure with exponential backoff (30s -> 30m),
	// sleeps 24h after a successful run.
	go func() {
		time.Sleep(5 * time.Second)
		backoff := 30 * time.Second
		for {
			n, err := sc.ScrapeAll()
			if err == nil && n > 0 {
				log.Info("scrape successful, sleeping 24h",
					zap.Int("upserted", n))
				backoff = 24 * time.Hour
			} else {
				log.Warn("scrape unsuccessful, retrying",
					zap.Int("upserted", n), zap.Error(err))
				if backoff < 30*time.Minute {
					backoff *= 2
				}
			}
			time.Sleep(backoff)
		}
	}()

	// ---- HTTP server ----
	svc := service.New(repo, log, c)
	h := handler.New(svc, log)

	if cfg.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(
		gin.Recovery(),
		middleware.Recovery(log),
		middleware.CORSDefault(),
		middleware.RequestIDAndLog(log),
	)

	h.Register(r)

	// Admin routes live on the same engine, registered after the
	// public handler so their /internal/* paths don't collide.
	admin := handler.NewAdmin(sc, log)
	admin.Register(r)

	srv := &http.Server{Addr: ":" + itoa(cfg.Port), Handler: r}
	go func() {
		log.Info("starting",
			zap.String("service", cfg.ServiceName),
			zap.Int("port", cfg.Port))
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
	log.Info("stopped")
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
