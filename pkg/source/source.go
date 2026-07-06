// Package source defines the provider abstraction. A source is anything that
// can answer a text query or dereference a URL into schema Cards. The engine
// only ever talks to this interface; no site is special-cased anywhere else.
package source

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/tamnd/shirabe/pkg/schema"
)

// ErrNotHandled means the source looked at the input and passed. The engine
// treats it as silence, not failure.
var ErrNotHandled = errors.New("source: not handled")

// Caps declares what a source can do and what it wants routed to it.
type Caps struct {
	Search  bool     // answers free-text queries
	Resolve bool     // dereferences URLs
	Hosts   []string // host suffixes this source resolves, e.g. "youtube.com"
	Intents []string // query intents it owns, e.g. "weather", "define", "stock"
}

// Query is a parsed user input handed to Search.
type Query struct {
	Raw    string // the text as typed
	Intent string // detected intent, empty for a plain search
	Arg    string // intent argument: the place, the word, the ticker
	Limit  int    // soft cap on cards to return
}

// Source is the SPI. Implementations must honor ctx cancellation and return
// ErrNotHandled when the input is not theirs.
type Source interface {
	Name() string
	Caps() Caps
	Search(ctx context.Context, q Query) ([]schema.Card, error)
	Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error)
}

// Availability lets a source report whether it can work right now, e.g. an
// exec adapter whose binary is missing. Optional.
type Availability interface {
	Available() bool
}

// Priority breaks ties when several sources match the same host or intent.
// Lower runs first. Optional; the default is 100.
type Priority interface {
	Priority() int
}

func priorityOf(s Source) int {
	if p, ok := s.(Priority); ok {
		return p.Priority()
	}
	return 100
}

func available(s Source) bool {
	if a, ok := s.(Availability); ok {
		return a.Available()
	}
	return true
}

// Registry holds the registered sources.
type Registry struct {
	mu      sync.RWMutex
	sources []Source
	byName  map[string]Source
}

func NewRegistry() *Registry {
	return &Registry{byName: map[string]Source{}}
}

// Register adds a source. Duplicate names are an error so a user manifest
// that shadows a builtin must replace it explicitly via Replace.
func (r *Registry) Register(s Source) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := s.Name()
	if name == "" {
		return errors.New("register: empty source name")
	}
	if _, dup := r.byName[name]; dup {
		return fmt.Errorf("register: duplicate source %q", name)
	}
	r.byName[name] = s
	r.sources = append(r.sources, s)
	r.sortLocked()
	return nil
}

// Replace registers s, displacing any source with the same name.
func (r *Registry) Replace(s Source) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := s.Name()
	if _, dup := r.byName[name]; dup {
		for i, old := range r.sources {
			if old.Name() == name {
				r.sources = append(r.sources[:i], r.sources[i+1:]...)
				break
			}
		}
	}
	r.byName[name] = s
	r.sources = append(r.sources, s)
	r.sortLocked()
}

func (r *Registry) sortLocked() {
	sort.SliceStable(r.sources, func(i, j int) bool {
		pi, pj := priorityOf(r.sources[i]), priorityOf(r.sources[j])
		if pi != pj {
			return pi < pj
		}
		return r.sources[i].Name() < r.sources[j].Name()
	})
}

// All returns every registered source in priority order.
func (r *Registry) All() []Source {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Source, len(r.sources))
	copy(out, r.sources)
	return out
}

// ByName looks a source up.
func (r *Registry) ByName(name string) (Source, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byName[name]
	return s, ok
}

// Searchers returns available sources that answer free-text queries.
func (r *Registry) Searchers() []Source {
	var out []Source
	for _, s := range r.All() {
		if s.Caps().Search && available(s) {
			out = append(out, s)
		}
	}
	return out
}

// ForHost returns available resolvers for a host, best match first: longest
// matching host suffix wins, then priority.
func (r *Registry) ForHost(host string) []Source {
	host = strings.ToLower(strings.TrimPrefix(host, "www."))
	type match struct {
		s      Source
		suffix int
	}
	var matches []match
	for _, s := range r.All() {
		if !s.Caps().Resolve || !available(s) {
			continue
		}
		best := -1
		for _, h := range s.Caps().Hosts {
			h = strings.ToLower(h)
			if host == h || strings.HasSuffix(host, "."+h) {
				if len(h) > best {
					best = len(h)
				}
			}
		}
		if len(s.Caps().Hosts) == 0 && s.Caps().Resolve {
			best = 0 // wildcard fallback resolver
		}
		if best >= 0 {
			matches = append(matches, match{s, best})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].suffix != matches[j].suffix {
			return matches[i].suffix > matches[j].suffix
		}
		return priorityOf(matches[i].s) < priorityOf(matches[j].s)
	})
	out := make([]Source, len(matches))
	for i, m := range matches {
		out[i] = m.s
	}
	return out
}

// ForIntent returns available sources that claimed an intent.
func (r *Registry) ForIntent(intent string) []Source {
	var out []Source
	for _, s := range r.All() {
		if !available(s) {
			continue
		}
		if slices.Contains(s.Caps().Intents, intent) {
			out = append(out, s)
		}
	}
	return out
}

// Status is one row of `shirabe sources`.
type Status struct {
	Name      string   `json:"name"`
	Search    bool     `json:"search"`
	Resolve   bool     `json:"resolve"`
	Hosts     []string `json:"hosts,omitempty"`
	Intents   []string `json:"intents,omitempty"`
	Available bool     `json:"available"`
	Priority  int      `json:"priority"`
}

// Statuses reports every source with its capabilities and availability.
func (r *Registry) Statuses() []Status {
	var out []Status
	for _, s := range r.All() {
		c := s.Caps()
		out = append(out, Status{
			Name: s.Name(), Search: c.Search, Resolve: c.Resolve,
			Hosts: c.Hosts, Intents: c.Intents,
			Available: available(s), Priority: priorityOf(s),
		})
	}
	return out
}
