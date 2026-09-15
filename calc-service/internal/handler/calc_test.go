package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// parseOpts is exercised directly against synthetic requests so
// query-string validation is testable without standing up the service
// and its three upstreams.

func TestParseOptsDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/calc/1", nil)

	opts, err := parseOpts(c)
	if err != nil {
		t.Fatalf("parseOpts: %v", err)
	}
	if opts.Mode != "normal" {
		t.Errorf("mode = %q, want normal", opts.Mode)
	}
	if opts.ActionsPerHourOverride != 0 {
		t.Errorf("aph override = %d, want 0", opts.ActionsPerHourOverride)
	}
	if opts.SpreadPctOverride != nil {
		t.Errorf("spread override = %v, want nil so the server default applies", *opts.SpreadPctOverride)
	}
	// Incomplete paths cost unpriced inputs at zero, so they must be
	// opt-in rather than the default.
	if opts.IncludeIncomplete {
		t.Error("include_incomplete must default to false")
	}
}

func TestParseOptsAcceptsValidValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet,
		"/calc/1?aph=900&spread_pct=3.5&player=Zezima&mode=ironman&include_incomplete=true&boosts=smithing_cape,rapid4", nil)

	opts, err := parseOpts(c)
	if err != nil {
		t.Fatalf("parseOpts: %v", err)
	}
	if opts.ActionsPerHourOverride != 900 {
		t.Errorf("aph = %d, want 900", opts.ActionsPerHourOverride)
	}
	if opts.SpreadPctOverride == nil || *opts.SpreadPctOverride != 3.5 {
		t.Errorf("spread = %v, want 3.5", opts.SpreadPctOverride)
	}
	if opts.Player != "Zezima" {
		t.Errorf("player = %q", opts.Player)
	}
	if opts.Mode != "ironman" {
		t.Errorf("mode = %q, want ironman", opts.Mode)
	}
	if !opts.IncludeIncomplete {
		t.Error("include_incomplete should be true")
	}
	if opts.BoostsRaw != "smithing_cape,rapid4" {
		t.Errorf("boosts raw = %q, want smithing_cape,rapid4", opts.BoostsRaw)
	}
	if opts.Boosts.BaseProgressBonus != 5 {
		t.Errorf("boosts base progress = %d, want 5", opts.Boosts.BaseProgressBonus)
	}
}

// A mistyped boost checkbox must fail the request rather than silently
// falling back to no boost at all.
func TestParseOptsRejectsUnknownBoost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/calc/1?boosts=not_a_real_boost", nil)

	if _, err := parseOpts(c); err == nil {
		t.Error("parseOpts should reject an unknown boost token")
	}
}

func TestParseOptsBoostsDefaultToEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/calc/1", nil)

	opts, err := parseOpts(c)
	if err != nil {
		t.Fatalf("parseOpts: %v", err)
	}
	if opts.BoostsRaw != "" {
		t.Errorf("boosts raw = %q, want empty", opts.BoostsRaw)
	}
	if opts.Boosts.BaseProgressBonus != 0 || opts.Boosts.DoubleProgressPct != 0 {
		t.Errorf("boosts = %+v, want the zero value", opts.Boosts)
	}
}

func TestParseOptsRejectsBadInput(t *testing.T) {
	cases := map[string]string{
		"negative aph":       "aph=-1",
		"non-numeric aph":    "aph=fast",
		"negative spread":    "spread_pct=-1",
		"spread above 100":   "spread_pct=101",
		"non-numeric spread": "spread_pct=wide",
		"unknown mode":       "mode=ultimate",
	}
	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/calc/1?"+query, nil)

			if _, err := parseOpts(c); err == nil {
				t.Errorf("parseOpts(%q) should have failed", query)
			}
		})
	}
}

func TestParseOptsTrimsPlayerName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/calc/1?player=%20Zezima%20", nil)

	opts, err := parseOpts(c)
	if err != nil {
		t.Fatalf("parseOpts: %v", err)
	}
	if opts.Player != "Zezima" {
		t.Errorf("player = %q, want it trimmed", opts.Player)
	}
}

func TestParseOptsZeroAphIsAccepted(t *testing.T) {
	// 0 means "no override", which is distinct from an invalid value.
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/calc/1?aph=0", nil)

	opts, err := parseOpts(c)
	if err != nil {
		t.Fatalf("parseOpts: %v", err)
	}
	if opts.ActionsPerHourOverride != 0 {
		t.Errorf("aph = %d, want 0", opts.ActionsPerHourOverride)
	}
}
