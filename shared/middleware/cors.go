package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rs3-market/backend/shared/config"
)

var defaultCORS = config.CORSConfig{
	AllowedOrigins: []string{"*"},
	AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	AllowedHeaders: []string{"Content-Type", "Authorization", "X-Request-ID", "X-Player", "X-Mode"},
	ExposeHeaders:  []string{"X-Request-ID"},
	MaxAge:         10 * time.Minute,
}

// CORS applies cross-origin headers and terminates preflight requests.
//
// The wildcard origin and credentials are mutually exclusive per the
// CORS spec — a browser rejects "Access-Control-Allow-Origin: *" on a
// credentialed request — so when credentials are enabled the request's
// own Origin is echoed back instead.
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowed := cfg.AllowedOrigins
	allowAny := len(allowed) == 0 || contains(allowed, "*")

	methods := strings.Join(orDefault(cfg.AllowedMethods, defaultCORS.AllowedMethods), ", ")
	headers := strings.Join(orDefault(cfg.AllowedHeaders, defaultCORS.AllowedHeaders), ", ")
	expose := strings.Join(orDefault(cfg.ExposeHeaders, defaultCORS.ExposeHeaders), ", ")

	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = defaultCORS.MaxAge
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			// Not a cross-origin request; CORS headers would be noise.
			c.Next()
			return
		}

		switch {
		case allowAny && cfg.AllowCreds:
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
		case allowAny:
			c.Header("Access-Control-Allow-Origin", "*")
		case contains(allowed, origin):
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			if cfg.AllowCreds {
				c.Header("Access-Control-Allow-Credentials", "true")
			}
		default:
			// Origin not allowed: send no CORS headers and let the
			// browser block it. Preflight still terminates here so the
			// request never reaches an upstream.
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
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

// CORSDefault is the permissive dev configuration: any origin, no
// credentials. Services behind the gateway use this so they stay
// directly callable during development.
func CORSDefault() gin.HandlerFunc { return CORS(defaultCORS) }

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
