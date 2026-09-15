// Package apicontract compares the routes a service registers against
// the routes its OpenAPI spec documents.
//
// It exists because a spec nobody verifies is worse than no spec: a
// frontend generated from a stale document compiles cleanly and then
// 404s at runtime. This project has already been there — the previous
// combined spec was unparseable, documented paths that no longer
// existed, and omitted several that did.
//
// Each service calls Check from a test in its handler package. Go's
// internal-package rule prevents one shared test from importing every
// service's handlers, so the comparison logic lives here instead and the
// assertions live next to the routes they guard.
package apicontract

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// httpMethods are the OpenAPI path-item keys that describe operations.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true,
	"patch": true, "delete": true, "head": true, "options": true,
}

type document struct {
	OpenAPI string                            `yaml:"openapi"`
	Paths   map[string]map[string]interface{} `yaml:"paths"`
}

// Result reports the two ways a spec and a router can disagree.
type Result struct {
	// Undocumented routes are served but absent from the spec, so a
	// generated client has no way to call them.
	Undocumented []string
	// Phantom routes are documented but not served, so a generated
	// client calls them and gets a 404.
	Phantom []string
}

// OK reports whether the spec and the router agree.
func (r Result) OK() bool { return len(r.Undocumented) == 0 && len(r.Phantom) == 0 }

func (r Result) Error() string {
	var b strings.Builder
	if len(r.Undocumented) > 0 {
		fmt.Fprintf(&b, "\n%d route(s) served but not documented:\n  %s",
			len(r.Undocumented), strings.Join(r.Undocumented, "\n  "))
	}
	if len(r.Phantom) > 0 {
		fmt.Fprintf(&b, "\n%d route(s) documented but not served:\n  %s",
			len(r.Phantom), strings.Join(r.Phantom, "\n  "))
	}
	b.WriteString("\n\nRegenerate with `make openapi` after editing openapi/combined.yaml.")
	return b.String()
}

// Check compares the engine's routes with the spec at specPath.
//
// `ignore` lists routes deliberately left out of the spec, as
// "METHOD /path" — operational endpoints and legacy redirects that
// should not become generated client methods.
func Check(engine *gin.Engine, specPath string, ignore []string) (Result, error) {
	documented, err := DocumentedRoutes(specPath)
	if err != nil {
		return Result{}, err
	}

	skip := make(map[string]bool, len(ignore))
	for _, route := range ignore {
		skip[route] = true
	}

	served := RegisteredRoutes(engine)

	var res Result
	for route := range served {
		if !documented[route] && !skip[route] {
			res.Undocumented = append(res.Undocumented, route)
		}
	}
	for route := range documented {
		if !served[route] && !skip[route] {
			res.Phantom = append(res.Phantom, route)
		}
	}
	sort.Strings(res.Undocumented)
	sort.Strings(res.Phantom)
	return res, nil
}

// RegisteredRoutes renders a gin router's routes in OpenAPI syntax,
// converting gin's ":itemID" parameters to "{itemID}".
func RegisteredRoutes(engine *gin.Engine) map[string]bool {
	out := map[string]bool{}
	for _, route := range engine.Routes() {
		out[route.Method+" "+ToOpenAPIPath(route.Path)] = true
	}
	return out
}

// DocumentedRoutes reads "METHOD /path" entries from an OpenAPI file.
func DocumentedRoutes(specPath string) (map[string]bool, error) {
	body, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	var doc document
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	if len(doc.Paths) == 0 {
		return nil, fmt.Errorf("%s declares no paths", specPath)
	}

	out := map[string]bool{}
	for path, item := range doc.Paths {
		for method := range item {
			if httpMethods[strings.ToLower(method)] {
				out[strings.ToUpper(method)+" "+path] = true
			}
		}
	}
	return out, nil
}

// ToOpenAPIPath converts gin's ":param" and "*param" segments to the
// OpenAPI "{param}" form.
func ToOpenAPIPath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, ":") || strings.HasPrefix(seg, "*") {
			parts[i] = "{" + seg[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}
