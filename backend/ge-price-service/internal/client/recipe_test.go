package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuyLimitsRequestsAndDecodes(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("ids")
		_, _ = w.Write([]byte(`{"count":1,"limits":{"453":25000}}`))
	}))
	defer srv.Close()

	got, err := NewRecipeClient(srv.URL).BuyLimits(context.Background(), []int64{453})
	if err != nil {
		t.Fatalf("BuyLimits: %v", err)
	}
	if gotPath != "/items/limits" {
		t.Errorf("path = %q, want /items/limits", gotPath)
	}
	if gotQuery != "453" {
		t.Errorf("ids = %q, want 453", gotQuery)
	}
	if got[453] != 25000 {
		t.Errorf("limit = %d, want 25000", got[453])
	}
}

// An item with no known limit must be absent rather than present as 0,
// so callers can tell "unknown" from "limited to nothing".
func TestBuyLimitsOmitsUnknownItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"count":0,"limits":{}}`))
	}))
	defer srv.Close()

	got, err := NewRecipeClient(srv.URL).BuyLimits(context.Background(), []int64{1, 2})
	if err != nil {
		t.Fatalf("BuyLimits: %v", err)
	}
	if _, present := got[1]; present {
		t.Error("an unknown item must not appear in the map")
	}
}

func TestBuyLimitsIgnoresUnparseableKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"limits":{"453":25000,"oops":5}}`))
	}))
	defer srv.Close()

	got, err := NewRecipeClient(srv.URL).BuyLimits(context.Background(), []int64{453})
	if err != nil {
		t.Fatalf("BuyLimits: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %v, want only the numeric key", got)
	}
}

func TestBuyLimitsUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	_, err := NewRecipeClient(srv.URL).BuyLimits(context.Background(), []int64{1})
	if err == nil {
		t.Fatal("expected an error for a 500 from recipe-service")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to carry the status code", err)
	}
}

func TestBuyLimitsRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewRecipeClient(srv.URL).BuyLimits(ctx, []int64{1}); err == nil {
		t.Fatal("expected a cancelled context to abort the request")
	}
}
