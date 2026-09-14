package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/db"
	"github.com/rs3-market/backend/shared/logger"
	"github.com/rs3-market/backend/shared/middleware"
	"github.com/rs3-market/backend/realtime-service/internal/auth"
	"github.com/rs3-market/backend/realtime-service/internal/chat"
	"github.com/rs3-market/backend/realtime-service/internal/handler"
	"github.com/rs3-market/backend/realtime-service/internal/hub"
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
		log.Fatal("db", zap.Error(err))
	}
	c := cache.New(cfg.Redis)
	if err := c.Ping(); err != nil {
		log.Fatal("redis", zap.Error(err))
	}

	a := auth.New(gdb, cfg.JWT.Secret, cfg.JWT.Expiry)
	if err := a.Migrate(); err != nil {
		log.Fatal("migrate auth", zap.Error(err))
	}
	chatSvc := chat.New(gdb, c, log, cfg.Chat.RateLimitWindow, cfg.Chat.HistoryLimit)

	h := hub.New(log)
	go h.Run()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// subscribe to Redis prices:* and fan out
	go chatSvc.RunPriceSubscriber(ctx, func(channel, payload string) {
		idStr := strings.TrimPrefix(channel, "prices:")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return
		}
		h.BroadcastPrice(id, []byte(payload))
	})

	// health debug: ping test event
	go func() {
		t := time.NewTicker(60 * time.Second)
		defer t.Stop()
		for range t.C {
			env := map[string]any{"ts": time.Now().UTC()}
			b, _ := json.Marshal(env)
			h.BroadcastChat(b) // no-op-ish heartbeat
		}
	}()

	hd := handler.New(h, a, chatSvc, log)
	if cfg.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), middleware.Recovery(log), middleware.RequestIDAndLog(log))
	hd.Register(r)

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
	cancel()
	sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer scancel()
	_ = srv.Shutdown(sctx)
}

func getenv(k, d string) string { if v := os.Getenv(k); v != "" { return v }; return d }

func itoa(i int) string {
	if i == 0 { return "8080" }
	b := [12]byte{}
	n := len(b)
	for i > 0 { n--; b[n] = byte('0' + i%10); i /= 10 }
	return string(b[n:])
}
