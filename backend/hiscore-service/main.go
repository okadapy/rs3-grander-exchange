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

	"github.com/rs3-market/backend/hiscore-service/internal/handler"
	"github.com/rs3-market/backend/hiscore-service/internal/repository"
	"github.com/rs3-market/backend/hiscore-service/internal/service"
	"github.com/rs3-market/backend/shared/cache"
	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/db"
	"github.com/rs3-market/backend/shared/logger"
	"github.com/rs3-market/backend/shared/middleware"
	"github.com/rs3-market/backend/shared/swagger"
)

func main() {
	cfgPath := os.Getenv("RS3_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		panic(err)
	}

	log := logger.New(cfg.LogLevel, cfg.Env == "local")
	defer log.Sync()

	gdb, err := db.Open(cfg.MySQL)
	if err != nil {
		log.Fatal("db open", zap.Error(err))
	}

	c := cache.New(cfg.Redis)
	if err := c.Ping(); err != nil {
		log.Fatal("redis ping", zap.Error(err))
	}

	repo := repository.New(gdb)
	if err := repo.Migrate(); err != nil {
		log.Fatal("migrate", zap.Error(err))
	}

	svc := service.New(cfg, log, repo)
	h := handler.New(svc, log)

	if cfg.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), middleware.Recovery(log), middleware.CORSDefault(), middleware.RequestIDAndLog(log))
	h.Register(r)
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("stopped")
}

func itoa(i int) string {
	if i == 0 {
		return "8080"
	}
	buf := [12]byte{}
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[n:])
}
