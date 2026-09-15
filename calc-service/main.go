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

	svc := service.New(log, rc, pc, hc, cc, service.MarketFrom(cfg.Market))
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
