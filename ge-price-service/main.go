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

	"github.com/rs3-market/backend/ge-price-service/internal/client"
	"github.com/rs3-market/backend/ge-price-service/internal/handler"
	"github.com/rs3-market/backend/ge-price-service/internal/poller"
	"github.com/rs3-market/backend/ge-price-service/internal/repository"
	"github.com/rs3-market/backend/ge-price-service/internal/service"
	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/db"
	"github.com/rs3-market/backend/shared/logger"
	"github.com/rs3-market/backend/shared/middleware"
	"github.com/rs3-market/backend/shared/swagger"
	"github.com/rs3-market/backend/shared/weirdgloop"
)

func main() {
	cfgPath := getenv("RS3_CONFIG", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel, cfg.Env == "local")
	defer log.Sync()

	gdb, err := db.Open(cfg.MySQL)
	if err != nil {
		log.Fatal("db", zap.Error(err))
	}
	c := cache.New(cfg.Redis)
	if err := c.Ping(); err != nil {
		log.Fatal("redis", zap.Error(err))
	}
	repo := repository.New(gdb)
	if err := repo.Migrate(); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}

	wgc := weirdgloop.New(cfg.WeirdGloop)
	recipeClient := client.NewRecipeClient(cfg.Services.RecipeURL)
	svc := service.New(repo, log, c, recipeClient)
	p := poller.New(cfg, repo, c, wgc, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Run(ctx)

	h := handler.New(svc, log)
	if cfg.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), middleware.Recovery(log), middleware.RequestIDAndLog(log))
	h.Register(r)

	admin := handler.NewAdmin(ctx, p, log)
	admin.Register(r)
	swagger.Register(r, "openapi/"+cfg.ServiceName+".yaml", cfg.ServiceName)

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
