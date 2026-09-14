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
