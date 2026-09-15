package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecipeClientTree(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"root":{"recipe":{"name":"Widget","output_item_id":100},"depth":0,"children":[]}}`))
	}))
	defer srv.Close()

	got, err := NewRecipeClient(srv.URL).Tree(context.Background(), 100)
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	if gotPath != "/recipes/100/tree" {
		t.Errorf("path = %q, want /recipes/100/tree", gotPath)
	}
	if got.Root == nil || got.Root.Recipe.Name != "Widget" {
		t.Errorf("root = %+v, want the Widget recipe", got.Root)
	}
}

func TestRecipeClientTreeSurfacesUpstreamStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"no recipe"}`))
	}))
	defer srv.Close()

	if _, err := NewRecipeClient(srv.URL).Tree(context.Background(), 100); err == nil {
		t.Fatal("expected an error for a 404 from recipe-service")
	}
}

func TestRecipeClientBuyLimits(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("ids")
		_, _ = w.Write([]byte(`{"count":2,"limits":{"1513":25000,"2363":10000}}`))
	}))
	defer srv.Close()

	got, err := NewRecipeClient(srv.URL).BuyLimits(context.Background(), []int64{1513, 2363})
	if err != nil {
		t.Fatalf("BuyLimits: %v", err)
	}
	if gotQuery != "1513,2363" {
		t.Errorf("ids = %q, want 1513,2363", gotQuery)
	}
	if got[1513] != 25000 || got[2363] != 10000 {
		t.Errorf("limits = %v", got)
	}
}

func TestRecipeClientBuyLimitsEmptyMakesNoRequest(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	got, err := NewRecipeClient(srv.URL).BuyLimits(context.Background(), nil)
	if err != nil {
		t.Fatalf("BuyLimits: %v", err)
	}
	if called {
		t.Error("an empty ID list should not reach the network")
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestPriceClientLatestKeysByItemID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"count":2,"prices":[
			{"item_id":100,"price":1000,"volume":5},
			{"item_id":200,"price":300,"volume":9}
		]}`))
	}))
	defer srv.Close()

	got, err := NewPriceClient(srv.URL).Latest(context.Background(), []int64{100, 200})
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got[100].Price != 1000 || got[200].Price != 300 {
		t.Errorf("prices = %v", got)
	}
}

// Player names contain spaces, so an unescaped name produces a malformed
// request URL and every lookup for those players fails.
func TestHiscoreClientEscapesName(t *testing.T) {
	var gotPath, gotMode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMode = r.URL.Query().Get("mode")
		_, _ = w.Write([]byte(`{"name":"Two Words","mode":"normal","skills":[]}`))
	}))
	defer srv.Close()

	got, err := NewHiscoreClient(srv.URL).Get(context.Background(), "Two Words", "normal")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if gotPath != "/hiscore/Two Words" {
		t.Errorf("path = %q, want the name escaped and decoded back", gotPath)
	}
	if gotMode != "normal" {
		t.Errorf("mode = %q, want normal", gotMode)
	}
	if got.Name != "Two Words" {
		t.Errorf("name = %q", got.Name)
	}
}

func TestRecipeClientAllItemIDs(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"skill":"","min_level":0,"max_level":0,"count":3,"item_ids":[100,200,300]}`))
	}))
	defer srv.Close()

	got, err := NewRecipeClient(srv.URL).AllItemIDs(context.Background())
	if err != nil {
		t.Fatalf("AllItemIDs: %v", err)
	}
	if gotPath != "/recipes/ids" {
		t.Errorf("path = %q, want /recipes/ids", gotPath)
	}
	want := []int64{100, 200, 300}
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ids = %v, want %v", got, want)
		}
	}
}

func TestRecipeClientAllItemIDsSurfacesUpstreamStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"db down"}`))
	}))
	defer srv.Close()

	if _, err := NewRecipeClient(srv.URL).AllItemIDs(context.Background()); err == nil {
		t.Fatal("expected an error for a 500 from recipe-service")
	}
}

func TestClientsTrimTrailingSlashFromBase(t *testing.T) {
	if got := NewRecipeClient("http://x:1/").Base; got != "http://x:1" {
		t.Errorf("base = %q, want no trailing slash", got)
	}
	if got := NewPriceClient("http://x:1///").Base; got != "http://x:1" {
		t.Errorf("base = %q, want no trailing slash", got)
	}
}
