// Package server is the HTTP surface: the embedded UI, one streaming query
// endpoint, resolve, sources, suggest, and a hardened image proxy.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/shirabe/pkg/engine"
	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
	"github.com/tamnd/shirabe/pkg/source/native/httpx"
	"github.com/tamnd/shirabe/web"
)

const maxQueryLen = 512

type Server struct {
	Engine   *engine.Engine
	Registry *source.Registry
	Version  string
	Dev      bool // serve web assets from disk and enable /dev/cards

	client *httpx.Client
	cache  *resultCache
}

func New(e *engine.Engine, reg *source.Registry, version string) *Server {
	return &Server{
		Engine: e, Registry: reg, Version: version,
		client: httpx.New(),
		cache:  newResultCache(60 * time.Second),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/query", s.handleQuery)
	mux.HandleFunc("GET /api/resolve", s.handleResolve)
	mux.HandleFunc("GET /api/sources", s.handleSources)
	mux.HandleFunc("GET /api/suggest", s.handleSuggest)
	mux.HandleFunc("GET /img", s.handleImg)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok", "version": s.Version})
	})
	if s.Dev {
		mux.HandleFunc("GET /api/dev/cards", s.handleDevCards)
	}

	var assets fs.FS = web.FS
	if s.Dev {
		assets = os.DirFS("web")
	}
	fileServer := http.FileServer(http.FS(assets))
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// The SPA shell answers / and /search; real files are served as is.
		if r.URL.Path == "/" || r.URL.Path == "/search" || (s.Dev && r.URL.Path == "/dev/cards") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			data, err := fs.ReadFile(assets, "index.html")
			if err != nil {
				http.Error(w, "missing UI", http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(data)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
	return logRequests(mux)
}

// ListenAndServe runs until ctx is canceled, then drains for 5 seconds.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

func (s *Server) query(r *http.Request) (string, error) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		return "", fmt.Errorf("missing q")
	}
	if len(q) > maxQueryLen {
		return "", fmt.Errorf("query too long")
	}
	return q, nil
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	q, err := s.query(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if r.URL.Query().Get("stream") == "0" {
		if res, ok := s.cache.get(q); ok {
			writeJSON(w, res)
			return
		}
		res := s.Engine.Run(r.Context(), q)
		s.cache.put(q, res)
		writeJSON(w, res)
		return
	}
	s.streamSSE(w, r, q)
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("url"))
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeError(w, http.StatusBadRequest, "url must be absolute http(s)")
		return
	}
	res := s.Engine.Run(r.Context(), u.String())
	writeJSON(w, res)
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.Registry.Statuses())
}

// handleSuggest proxies wikipedia opensearch for typeahead. Recent local
// queries live client-side; this endpoint stays stateless.
func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) > maxQueryLen {
		writeJSON(w, []string{})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	u := "https://en.wikipedia.org/w/api.php?action=opensearch&format=json&limit=6&search=" + url.QueryEscape(q)
	raw, err := s.client.Get(ctx, u)
	if err != nil {
		writeJSON(w, []string{})
		return
	}
	var parts []json.RawMessage
	var titles []string
	if json.Unmarshal(raw, &parts) == nil && len(parts) > 1 {
		_ = json.Unmarshal(parts[1], &titles)
	}
	if titles == nil {
		titles = []string{}
	}
	writeJSON(w, titles)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.code, time.Since(start).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// resultCache keeps buffered query results just long enough that the back
// button feels instant. Queries are never written to disk.
type resultCache struct {
	mu  sync.Mutex
	ttl time.Duration
	m   map[string]cacheEntry
}

type cacheEntry struct {
	res schema.Result
	at  time.Time
}

func newResultCache(ttl time.Duration) *resultCache {
	return &resultCache{ttl: ttl, m: map[string]cacheEntry{}}
}

func (c *resultCache) get(q string) (schema.Result, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[q]
	if !ok || time.Since(e.at) > c.ttl {
		return schema.Result{}, false
	}
	return e.res, true
}

func (c *resultCache) put(q string, res schema.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) > 256 {
		c.m = map[string]cacheEntry{}
	}
	c.m[q] = cacheEntry{res, time.Now()}
}
