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
