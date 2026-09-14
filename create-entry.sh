#!/usr/bin/env bash
# ============================================================
#  add-gateway.sh
#  Adds a single API Gateway that fronts all services on :8080.
#  Also aggregates Swagger UI at /swagger.
#  Run from project root (where go.mod lives).
# ============================================================
set -euo pipefail

if [ ! -f "go.mod" ] || [ ! -d "hiscore-service" ]; then
  echo "Run this from the project root." >&2; exit 1
fi

mkdir -p gateway/internal/{proxy,middleware,handler}

# ============================================================
#  1. EXTEND SHARED CONFIG with Routes, CORS, RateLimit
# ============================================================

cat > shared/config/config.go <<'GO'
package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	ServiceName string          `mapstructure:"service_name"`
	Port        int             `mapstructure:"port"`
	LogLevel    string          `mapstructure:"log_level"`
	Env         string          `mapstructure:"env"`
	MySQL       MySQLConfig     `mapstructure:"mysql"`
	Redis       RedisConfig     `mapstructure:"redis"`
	WeirdGloop  WeirdGloopConf  `mapstructure:"weirdgloop"`
	Hiscore     HiscoreConf     `mapstructure:"hiscore"`
	Poller      PollerConf      `mapstructure:"poller"`
	Services    ServicesConf    `mapstructure:"services"`
	JWT         JWTConf         `mapstructure:"jwt"`
	Chat        ChatConf        `mapstructure:"chat"`

	// Gateway-only fields
	Routes    []RouteConfig   `mapstructure:"routes"`
	CORS      CORSConfig      `mapstructure:"cors"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
}

type MySQLConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DBName          string        `mapstructure:"db_name"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type WeirdGloopConf struct {
	BaseURL     string        `mapstructure:"base_url"`
	UserAgent   string        `mapstructure:"user_agent"`
	BatchSize   int           `mapstructure:"batch_size"`
	MinGap      time.Duration `mapstructure:"min_gap"`
	HTTPTimeout time.Duration `mapstructure:"http_timeout"`
}

type HiscoreConf struct {
	BaseURL  string        `mapstructure:"base_url"`
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

type PollerConf struct {
	PollInterval time.Duration `mapstructure:"poll_interval"`
}

type ServicesConf struct {
	RecipeURL  string `mapstructure:"recipe_url"`
	PriceURL   string `mapstructure:"price_url"`
	HiscoreURL string `mapstructure:"hiscore_url"`
}

type JWTConf struct {
	Secret string        `mapstructure:"secret"`
	Expiry time.Duration `mapstructure:"expiry"`
}

type ChatConf struct {
	RateLimitWindow time.Duration `mapstructure:"rate_limit_window"`
	HistoryLimit    int           `mapstructure:"history_limit"`
}

// ---------------- gateway-only ----------------

type RouteConfig struct {
	Prefix    string        `mapstructure:"prefix"`
	Target    string        `mapstructure:"target"`
	WebSocket bool          `mapstructure:"websocket"`
	Timeout   time.Duration `mapstructure:"timeout"`
	StripPrefix bool        `mapstructure:"strip_prefix"`
}

type CORSConfig struct {
	AllowedOrigins []string      `mapstructure:"allowed_origins"`
	AllowedMethods []string      `mapstructure:"allowed_methods"`
	AllowedHeaders []string      `mapstructure:"allowed_headers"`
	ExposeHeaders  []string      `mapstructure:"expose_headers"`
	MaxAge         time.Duration `mapstructure:"max_age"`
	AllowCreds     bool          `mapstructure:"allow_credentials"`
}

type RateLimitConfig struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst             int     `mapstructure:"burst"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("RS3")
	v.AutomaticEnv()

	_ = v.BindEnv("env", "RS3_ENV")
	_ = v.BindEnv("mysql.host", "RS3_DB_HOST")
	_ = v.BindEnv("mysql.password", "RS3_DB_PASSWORD")
	_ = v.BindEnv("redis.addr", "RS3_REDIS_ADDR")
	_ = v.BindEnv("jwt.secret", "RS3_JWT_SECRET")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
GO

# ============================================================
#  2. GATEWAY CONFIG
# ============================================================
cat > gateway/config.yaml <<'YAML'
service_name: gateway
port: 8080
log_level: debug
env: local

redis:
  addr: redis:6379
  password: ""
  db: 0
  pool_size: 10

# Routing table — first match wins.
routes:
  - prefix: /hiscore
    target: http://hiscore-service:8081
    timeout: 20s
  - prefix: /recipes
    target: http://recipe-service:8082
    timeout: 30s
  - prefix: /prices
    target: http://ge-price-service:8083
    timeout: 20s
  - prefix: /calc
    target: http://calc-service:8084
    timeout: 60s
  - prefix: /auth
    target: http://realtime-service:8085
    timeout: 10s
  - prefix: /chat
    target: http://realtime-service:8085
    timeout: 10s
  - prefix: /ws
    target: http://realtime-service:8085
    timeout: 0s
    websocket: true

cors:
  allowed_origins: ["*"]
  allowed_methods: ["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]
  allowed_headers: ["Content-Type", "Authorization", "X-Request-ID", "X-Player", "X-Mode"]
  expose_headers:  ["X-Request-ID"]
  max_age: 10m
  allow_credentials: false

ratelimit:
  requests_per_second: 50
  burst: 100
YAML

# ============================================================
#  3. GATEWAY DOCKERFILE
# ============================================================
cat > gateway/Dockerfile <<'DOCKER'
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download || true
COPY shared ./shared
COPY gateway ./gateway
RUN CGO_ENABLED=0 go build -o /out/app ./gateway

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/app /app/app
COPY gateway/config.yaml /app/config.yaml
COPY openapi /app/openapi
ENV RS3_CONFIG=/app/config.yaml
EXPOSE 8080
ENTRYPOINT ["/app/app"]
DOCKER

# ============================================================
#  4. PROXY PACKAGE
# ============================================================
cat > gateway/internal/proxy/proxy.go <<'GO'
package proxy

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/config"
)

// Route is a compiled proxy route.
type Route struct {
	Prefix      string
	Target      *url.URL
	WebSocket   bool
	StripPrefix bool
	Reverse     *httputil.ReverseProxy
}

// Build compiles a slice of route configs into reverse proxies.
func Build(cfgs []config.RouteConfig, log *zap.Logger) ([]*Route, error) {
	out := make([]*Route, 0, len(cfgs))
	for _, c := range cfgs {
		target, err := url.Parse(c.Target)
		if err != nil {
			return nil, fmt.Errorf("bad target %q: %w", c.Target, err)
		}

		rp := httputil.NewSingleHostReverseProxy(target)
		// FlushInterval -1 means flush every chunk immediately — required
		// for WebSocket upgrades and SSE streams.
		rp.FlushInterval = -1
		rp.Transport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			// Disable compression so we can stream.
			DisableCompression: true,
		}

		strip := c.StripPrefix
		originalDirector := rp.Director
		rp.Director = func(req *http.Request) {
			originalDirector(req)
			if strip {
				p := strings.TrimPrefix(req.URL.Path, c.Prefix)
				if p == "" {
					p = "/"
				}
				req.URL.Path = p
			}
			// Preserve original host for logging.
			req.Header.Set("X-Forwarded-Host", req.Host)
		}

		// Capture route target for logging inside the error handler.
		targetStr := c.Target
		rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Error("upstream error",
				zap.String("target", targetStr),
				zap.String("path", r.URL.Path),
				zap.Error(err),
			)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "upstream unavailable",
				"target":  targetStr,
				"details": err.Error(),
			})
		}

		out = append(out, &Route{
			Prefix:      c.Prefix,
			Target:      target,
			WebSocket:   c.WebSocket,
			StripPrefix: strip,
			Reverse:     rp,
		})
	}
	return out, nil
}

// Match returns the first route whose prefix matches the path.
func Match(routes []*Route, path string) *Route {
	for _, r := range routes {
		if strings.HasPrefix(path, r.Prefix) {
			// ensure prefix boundary — /recipes should match /recipes/1
			// but /recipes-x should NOT match /recipes
			if len(path) == len(r.Prefix) ||
				path[len(r.Prefix)] == '/' ||
				path[len(r.Prefix)] == '?' {
				return r
			}
		}
	}
	return nil
}

// Handler returns a Gin handler that dispatches to the matched route.
func Handler(routes []*Route, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		route := Match(routes, c.Request.URL.Path)
		if route == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "no route matches"})
			return
		}
		log.Debug("proxy",
			zap.String("path", c.Request.URL.Path),
			zap.String("target", route.Target.String()),
			zap.Bool("ws", route.WebSocket),
		)
		route.Reverse.ServeHTTP(c.Writer, c.Request)
	}
}
GO

# ============================================================
#  5. CORS MIDDLEWARE
# ============================================================
cat > gateway/internal/middleware/cors.go <<'GO'
package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rs3-market/backend/shared/config"
)

// CORS applies permissive cross-origin headers based on config.
// Preflight (OPTIONS) requests terminate at the gateway.
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowed := cfg.AllowedOrigins
	allowAny := len(allowed) == 0 || contains(allowed, "*")

	methods := strings.Join(orDefault(cfg.AllowedMethods,
		[]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}), ", ")
	headers := strings.Join(orDefault(cfg.AllowedHeaders,
		[]string{"Content-Type", "Authorization", "X-Request-ID"}), ", ")
	expose := strings.Join(cfg.ExposeHeaders, ", ")
	maxAge := cfg.MaxAge
	if maxAge == 0 {
		maxAge = 10 * time.Minute
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}

		if allowAny {
			if cfg.AllowCreds {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
			} else {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		} else if contains(allowed, origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			if cfg.AllowCreds {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		}

		c.Header("Access-Control-Allow-Methods", methods)
		c.Header("Access-Control-Allow-Headers", headers)
		if expose != "" {
			c.Header("Access-Control-Expose-Headers", expose)
		}
		c.Header("Access-Control-Max-Age", strconv.Itoa(int(maxAge.Seconds())))

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if strings.EqualFold(x, v) {
			return true
		}
	}
	return false
}

func orDefault(v, def []string) []string {
	if len(v) > 0 {
		return v
	}
	return def
}
GO

# ============================================================
#  6. RATE LIMIT MIDDLEWARE (per-IP token bucket, stdlib only)
# ============================================================
cat > gateway/internal/middleware/ratelimit.go <<'GO'
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rs3-market/backend/shared/config"
)

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimit implements a simple per-IP token bucket. Cleanup runs every
// 5 minutes and drops buckets idle for more than 10 minutes. This is
// sufficient for a single gateway replica; if you scale the gateway
// horizontally, swap this out for a Redis-backed sliding window.
func RateLimit(cfg config.RateLimitConfig) gin.HandlerFunc {
	rps := cfg.RequestsPerSecond
	if rps <= 0 {
		rps = 50
	}
	burst := cfg.Burst
	if burst <= 0 {
		burst = 100
	}

	var (
		mu      sync.Mutex
		buckets = map[string]*bucket{}
	)

	go func() {
		t := time.NewTicker(5 * time.Minute)
		defer t.Stop()
		for range t.C {
			cutoff := time.Now().Add(-10 * time.Minute)
			mu.Lock()
			for k, b := range buckets {
				if b.lastSeen.Before(cutoff) {
					delete(buckets, k)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		// Skip WebSocket upgrades — those are long-lived and handled by
		// the realtime service's own message-level rate limit.
		if c.GetHeader("Upgrade") != "" {
			c.Next()
			return
		}

		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		b, ok := buckets[ip]
		if !ok {
			b = &bucket{tokens: float64(burst), lastSeen: now}
			buckets[ip] = b
		}
		elapsed := now.Sub(b.lastSeen).Seconds()
		b.tokens += elapsed * rps
		if b.tokens > float64(burst) {
			b.tokens = float64(burst)
		}
		b.lastSeen = now

		allowed := b.tokens >= 1
		if allowed {
			b.tokens--
		}
		mu.Unlock()

		if !allowed {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
GO

# ============================================================
#  7. AGGREGATED SWAGGER + HEALTH HANDLERS
# ============================================================
cat > gateway/internal/handler/swagger.go <<'GO'
package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// swaggerHTML renders a Swagger UI page with a service dropdown.
const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>RS3 Market — API Gateway</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    html, body { margin:0; padding:0; background:#fafafa; font-family: -apple-system, system-ui, sans-serif; }
    #bar { padding:12px 20px; background:#1b1b1f; color:#fff; display:flex; align-items:center; gap:16px; }
    #bar h1 { font-size:16px; margin:0; font-weight:600; }
    #bar select { padding:6px 10px; border-radius:6px; border:1px solid #444; background:#2a2a30; color:#fff; font-size:14px; }
  </style>
</head>
<body>
  <div id="bar">
    <h1>RS3 Market API</h1>
    <select id="svc">%s</select>
  </div>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    let ui;
    window.addEventListener('load', function () {
      const sel = document.getElementById('svc');
      ui = SwaggerUIBundle({
        url: sel.value,
        dom_id: '#swagger-ui',
        deepLinking: true,
        displayOperationId: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        layout: 'BaseLayout'
      });
      sel.addEventListener('change', () => {
        ui.specActions.updateUrl(sel.value);
        ui.specActions.download(sel.value);
      });
    });
  </script>
</body>
</html>`

// Swagger registers the aggregated UI + per-service spec routes.
//
//	GET /swagger                     -> UI with dropdown
//	GET /swagger/index.html          -> same UI
//	GET /swagger/spec/{service}.yaml -> raw spec
func Swagger(r *gin.Engine, specDir string) {
	r.GET("/swagger", redirect("/swagger/index.html"))
	r.GET("/swagger/", redirect("/swagger/index.html"))

	r.GET("/swagger/index.html", func(c *gin.Context) {
		files, err := filepath.Glob(filepath.Join(specDir, "*.yaml"))
		if err != nil || len(files) == 0 {
			c.String(http.StatusInternalServerError,
				"no openapi specs found in %q", specDir)
			return
		}
		names := make([]string, 0, len(files))
		for _, f := range files {
			names = append(names, strings.TrimSuffix(filepath.Base(f), ".yaml"))
		}
		sort.Strings(names)

		var opts strings.Builder
		for i, n := range names {
			fmt.Fprintf(&opts,
				`<option value="/swagger/spec/%s.yaml"%s>%s</option>`,
				n, selAttr(i == 0), n)
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8",
			[]byte(fmt.Sprintf(swaggerHTML, opts.String())))
	})

	r.GET("/swagger/spec/:file", func(c *gin.Context) {
		file := c.Param("file")
		if strings.Contains(file, "..") || strings.Contains(file, "/") {
			c.String(http.StatusBadRequest, "invalid")
			return
		}
		path := filepath.Join(specDir, file)
		b, err := os.ReadFile(path)
		if err != nil {
			c.String(http.StatusNotFound, "spec not found: %s", file)
			return
		}
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", b)
	})
}

func redirect(to string) gin.HandlerFunc {
	return func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, to) }
}

func selAttr(sel bool) string {
	if sel {
		return " selected"
	}
	return ""
}
GO

cat > gateway/internal/handler/health.go <<'GO'
package handler

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/rs3-market/backend/shared/config"
)

type upstream struct {
	Name   string
	URL    string
	Client *http.Client
}

// Health aggregates /health from every upstream route target.
type Health struct {
	log       *zap.Logger
	upstreams []upstream
}

func NewHealth(cfg *config.Config, log *zap.Logger) *Health {
	seen := map[string]bool{}
	var ups []upstream
	for _, r := range cfg.Routes {
		if seen[r.Target] {
			continue
		}
		seen[r.Target] = true
		ups = append(ups, upstream{
			Name:   r.Target,
			URL:    r.Target + "/health",
			Client: &http.Client{Timeout: 3 * time.Second},
		})
	}
	return &Health{log: log, upstreams: ups}
}

func (h *Health) Register(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "gateway"})
	})
	r.GET("/health/all", h.all)
}

type result struct {
	Target string `json:"target"`
	OK     bool   `json:"ok"`
	Status int    `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
	Ms     int64  `json:"ms"`
}

func (h *Health) all(c *gin.Context) {
	results := make([]result, len(h.upstreams))
	var wg sync.WaitGroup
	for i, u := range h.upstreams {
		wg.Add(1)
		go func(i int, u upstream) {
			defer wg.Done()
			start := time.Now()
			resp, err := u.Client.Get(u.URL)
			elapsed := time.Since(start).Milliseconds()
			if err != nil {
				results[i] = result{Target: u.Name, OK: false, Error: err.Error(), Ms: elapsed}
				return
			}
			defer resp.Body.Close()
			results[i] = result{Target: u.Name, OK: resp.StatusCode == 200,
				Status: resp.StatusCode, Ms: elapsed}
		}(i, u)
	}
	wg.Wait()

	allOK := true
	for _, r := range results {
		if !r.OK {
			allOK = false
			break
		}
	}
	code := http.StatusOK
	if !allOK {
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{"gateway": "ok", "all_upstreams_ok": allOK, "upstreams": results})
}
GO

# ============================================================
#  8. GATEWAY MAIN
# ============================================================
cat > gateway/main.go <<'GO'
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

	"github.com/rs3-market/backend/shared/config"
	"github.com/rs3-market/backend/shared/logger"
	"github.com/rs3-market/backend/shared/middleware"
	"github.com/rs3-market/backend/gateway/internal/handler"
	gwmid "github.com/rs3-market/backend/gateway/internal/middleware"
	"github.com/rs3-market/backend/gateway/internal/proxy"
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
GO

# ============================================================
#  9. PATCH DOCKER-COMPOSE to add gateway
# ============================================================
if ! grep -q '^  gateway:' docker-compose.yml; then
  # insert gateway block before the "volumes:" line at the end
  python3 - <<'PY'
import re, pathlib
p = pathlib.Path("docker-compose.yml")
s = p.read_text()
block = '''  gateway:
    build: { context: ., dockerfile: gateway/Dockerfile }
    environment:
      <<: *svc-env
      RS3_CONFIG: /app/config.yaml
    depends_on:
      hiscore-service:  { condition: service_started }
      recipe-service:   { condition: service_started }
      ge-price-service: { condition: service_started }
      calc-service:     { condition: service_started }
      realtime-service: { condition: service_started }
    ports: ["8080:8080"]

'''
s = re.sub(r'^volumes:', block + 'volumes:', s, count=1, flags=re.M)
p.write_text(s)
PY
  echo "  patched docker-compose.yml"
fi

# ============================================================
#  10. PATCH MAKEFILE to include gateway in build list
# ============================================================
if ! grep -q 'gateway' Makefile; then
  perl -0777 -pi -e 's/^SERVICES = (.+)$/SERVICES = $1 gateway/m' Makefile
  echo "  patched Makefile"
fi

# ============================================================
#  11. FORMAT + VERIFY
# ============================================================
echo ""
echo ">>> gofmt"
gofmt -w shared/config/config.go gateway/main.go gateway/internal/proxy/proxy.go \
         gateway/internal/middleware/cors.go gateway/internal/middleware/ratelimit.go \
         gateway/internal/handler/swagger.go gateway/internal/handler/health.go

echo ""
echo ">>> verifying gateway files"
for f in gateway/main.go gateway/config.yaml gateway/Dockerfile \
         gateway/internal/proxy/proxy.go \
         gateway/internal/middleware/cors.go \
         gateway/internal/middleware/ratelimit.go \
         gateway/internal/handler/swagger.go \
         gateway/internal/handler/health.go; do
  [ -f "$f" ] && printf "  ✓ %s\n" "$f" || printf "  ✗ MISSING %s\n" "$f"
done

echo ""
echo "============================================================"
echo " API Gateway added."
echo ""
echo " Single entrypoint:  http://localhost:8080"
echo ""
echo " Routes (proxied):"
echo "   /hiscore/*   → hiscore-service:8081"
echo "   /recipes/*   → recipe-service:8082"
echo "   /prices/*    → ge-price-service:8083"
echo "   /calc/*      → calc-service:8084"
echo "   /auth/*      → realtime-service:8085"
echo "   /chat/*      → realtime-service:8085"
echo "   /ws          → realtime-service:8085  (WebSocket)"
echo ""
echo " Gateway-owned:"
echo "   /health          liveness"
echo "   /health/all      aggregates every upstream /health"
echo "   /swagger         aggregated Swagger UI (dropdown)"
echo ""
echo " Rebuild:  make dev"
echo "============================================================"
