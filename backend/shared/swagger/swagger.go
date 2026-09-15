// Package swagger serves an OpenAPI spec file plus a Swagger UI page
// that renders it. It has no dependencies beyond gin and the standard
// library, so any service can mount it with a single call.
package swagger

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// htmlTemplate is a self-contained Swagger UI page that loads Swagger UI
// from unpkg and points at /swagger/spec.yaml.
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>%s — API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    html, body { margin: 0; padding: 0; background: #fafafa; }
    #swagger-ui { font-family: -apple-system, system-ui, sans-serif; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    window.addEventListener('load', function () {
      window.ui = SwaggerUIBundle({
        url: '%s',
        dom_id: '#swagger-ui',
        deepLinking: true,
        displayOperationId: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIStandalonePreset
        ],
        layout: 'BaseLayout'
      });
    });
  </script>
</body>
</html>`

// Register mounts:
//
//	GET /swagger                  -> redirect to /swagger/index.html
//	GET /swagger/index.html       -> Swagger UI page
//	GET /swagger/spec.yaml        -> raw OpenAPI spec
//
// specPath is resolved at request time so missing specs produce a clear
// 404 rather than a crash at startup.
func Register(r *gin.Engine, specPath, title string) {
	if title == "" {
		title = "API"
	}
	if specPath == "" {
		specPath = "openapi/openapi.yaml"
	}

	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/index.html", func(c *gin.Context) {
		body := fmt.Sprintf(htmlTemplate, title, "/swagger/spec.yaml")
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(body))
	})
	r.GET("/swagger/spec.yaml", func(c *gin.Context) {
		b, err := os.ReadFile(specPath)
		if err != nil {
			c.String(http.StatusNotFound,
				"openapi spec not found at %q: %v", specPath, err)
			return
		}
		c.Data(http.StatusOK, "application/yaml; charset=utf-8", b)
	})
}
