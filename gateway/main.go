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

	"github.com/rs3-market/backend/gateway/internal/handler"
	gwmid "github.com/rs3-market/backend/gateway/internal/middleware"
	"github.com/rs3-market/backend/gateway/internal/proxy"
	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/logger"
	"github.com/rs3-market/backend/shared/middleware"
)

func main() {
	cfgPath := getenv("RS3_CONFIG", "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel, cfg.Env == "local")
	defer log.Sync()

	routes, err := proxy.Build(cfg.Routes, log)
	if err != nil {
		log.Fatal("build routes", zap.Error(err))
	}
	log.Info("gateway routes compiled", zap.Int("count", len(routes)))
	for _, r := range routes {
		log.Info("route",
			zap.String("prefix", r.Prefix),
			zap.String("target", r.Target.String()),
			zap.Bool("websocket", r.WebSocket))
	}

	if cfg.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(
		gin.Recovery(),
		middleware.Recovery(log),
		gwmid.CORS(cfg.CORS),
		gwmid.RateLimit(cfg.RateLimit),
		middleware.RequestIDAndLog(log),
	)

	// Gateway-owned endpoints take priority — register them first.
	handler.Swagger(r, "openapi")
	h := handler.NewHealth(cfg, log)
	h.Register(r)

	// Everything else is proxied.
	r.NoRoute(proxy.Handler(routes, log))
	// Also catch paths that match the route prefixes exactly (no trailing slash)
	// so /hiscore, /recipes, etc. hit the proxy, not the 404 handler.
	r.Any("/*any", proxy.Handler(routes, log))

	srv := &http.Server{
		Addr:              ":" + itoa(cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout — WebSockets are long-lived.
	}

	go func() {
		log.Info("gateway starting",
			zap.String("service", cfg.ServiceName),
			zap.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("listen", zap.Error(err))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("gateway stopped")
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
