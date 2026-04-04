package dash

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParser_Parse_VOD(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/dash+xml")
		fmt.Fprint(w, vodMPD) // reuse from dash_test.go
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	result, err := p.Parse(context.Background(), server.URL+"/manifest.mpd", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsLive {
		t.Error("expected VOD")
	}
	if len(result.Streams) != 3 {
		t.Fatalf("expected 3 streams, got %d", len(result.Streams))
	}
	if result.Streams[0].Playlist == nil {
		t.Fatal("expected playlist")
	}
	if result.Streams[0].Playlist.MediaInit == nil {
		t.Fatal("expected init segment")
	}
}

func TestParser_Parse_HTTP404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/missing.mpd", nil)
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestParser_Parse_HTTP403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/forbidden.mpd", nil)
	if err == nil {
		t.Fatal("expected error for 403")
	}
}

func TestParser_Parse_HTTP502(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/bad.mpd", nil)
	if err == nil {
		t.Fatal("expected error for 502")
	}
}

func TestParser_Parse_InvalidXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<not valid xml!!!`)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/bad.mpd", nil)
	if err == nil {
		t.Fatal("expected error for invalid XML")
	}
}

func TestParser_Parse_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		fmt.Fprint(w, vodMPD)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Parse(ctx, server.URL+"/slow.mpd", nil)
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

func TestParser_Parse_CustomHeaders(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, vodMPD)
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	headers := map[string]string{"Authorization": "Bearer token123"}
	_, err := p.Parse(context.Background(), server.URL+"/manifest.mpd", headers)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer token123" {
		t.Errorf("expected auth header 'Bearer token123', got %q", gotAuth)
	}
}

func TestParser_Parse_ServerDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()

	p := &Parser{}
	_, err := p.Parse(context.Background(), server.URL+"/manifest.mpd", nil)
	if err == nil {
		t.Fatal("expected error when server is down")
	}
}

func TestParser_Parse_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "")
	}))
	defer server.Close()

	p := &Parser{Client: server.Client()}
	_, err := p.Parse(context.Background(), server.URL+"/empty.mpd", nil)
	if err == nil {
		t.Fatal("expected error for empty MPD")
	}
}
