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
