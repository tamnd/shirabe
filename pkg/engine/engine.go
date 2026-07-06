// Package engine turns one input box into fanned-out source calls. It
// classifies the input (URL or text), detects intents, routes to the
// registry, and streams card batches back as sources answer.
package engine

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

// Engine holds the registry and the timing policy.
type Engine struct {
	Registry       *source.Registry
	Deadline       time.Duration // whole query budget
	SourceDeadline time.Duration // per-source budget
	Limit          int           // default per-source card budget
}

func New(reg *source.Registry) *Engine {
	return &Engine{Registry: reg, Deadline: 8 * time.Second, SourceDeadline: 5 * time.Second, Limit: 8}
}

// Event is one message on the stream: a batch of cards from one source, or a
// source failure, or the final done marker with timings.
type Event struct {
	Type    string               `json:"type"` // cards, error, done
	Source  string               `json:"source,omitempty"`
	Cards   []schema.Card        `json:"cards,omitempty"`
	Message string               `json:"message,omitempty"`
	Timings []schema.Timing      `json:"timings,omitempty"`
	Errors  []schema.SourceError `json:"errors,omitempty"`
}

// bangs force a single source: "!yt lofi" searches only the youtube source.
var bangs = map[string]string{
	"!yt":   "youtube",
	"!wiki": "wikipedia",
	"!hn":   "hackernews",
	"!amz":  "amazon",
	"!gr":   "goodreads",
}

var (
	weatherLead  = regexp.MustCompile(`(?i)^weather\s+(?:in\s+|at\s+|for\s+)?(.+)$`)
	weatherTrail = regexp.MustCompile(`(?i)^(.+?)\s+weather$`)
	defineLead   = regexp.MustCompile(`(?i)^(?:define|definition of|meaning of)\s+([a-z][a-z '-]*)$`)
	stockLead    = regexp.MustCompile(`(?i)^(?:stock|quote|ticker)\s+([a-z0-9.^-]{1,12})$`)
	stockTrail   = regexp.MustCompile(`(?i)^([a-z0-9.^-]{1,12})\s+(?:stock|share price|stock price|quote)$`)
)

// Parse classifies raw input. It returns either a URL to resolve or a Query
// to search, plus the forced source name when a bang prefix was used.
func (e *Engine) Parse(raw string) (u *url.URL, q source.Query, forced string) {
	raw = strings.TrimSpace(raw)
	if parsed, ok := asURL(raw); ok {
		return parsed, source.Query{}, ""
	}
	if bang, rest, ok := strings.Cut(raw, " "); ok {
		if name, isBang := bangs[strings.ToLower(bang)]; isBang && strings.TrimSpace(rest) != "" {
			return nil, source.Query{Raw: strings.TrimSpace(rest), Limit: e.Limit}, name
		}
	}
	q = source.Query{Raw: raw, Limit: e.Limit}
	for intent, res := range map[string][]*regexp.Regexp{
		"weather": {weatherLead, weatherTrail},
		"define":  {defineLead},
		"stock":   {stockLead, stockTrail},
	} {
		for _, re := range res {
			if m := re.FindStringSubmatch(raw); m != nil {
				q.Intent, q.Arg = intent, strings.TrimSpace(m[1])
				return nil, q, ""
			}
		}
	}
	return nil, q, ""
}

func asURL(raw string) (*url.URL, bool) {
	if strings.ContainsAny(raw, " \t") {
		return nil, false
	}
	candidate := raw
	if !strings.Contains(candidate, "://") {
		if !strings.Contains(candidate, ".") {
			return nil, false
		}
		candidate = "https://" + candidate
	}
	u, err := url.Parse(candidate)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, false
	}
	if !strings.Contains(u.Host, ".") {
		return nil, false
	}
	return u, true
}

// Stream runs the query and emits events as sources answer. The channel is
// closed after the done event. Cancel ctx to abort early.
func (e *Engine) Stream(ctx context.Context, raw string) <-chan Event {
	out := make(chan Event, 16)
	go func() {
		defer close(out)
		ctx, cancel := context.WithTimeout(ctx, e.Deadline)
		defer cancel()

		u, q, forced := e.Parse(raw)
		if u != nil {
			e.resolveStream(ctx, u, out)
			return
		}
		e.searchStream(ctx, q, forced, out)
	}()
	return out
}

func (e *Engine) resolveStream(ctx context.Context, u *url.URL, out chan<- Event) {
	var errs []schema.SourceError
	var timings []schema.Timing
	for _, s := range e.Registry.ForHost(u.Hostname()) {
		start := time.Now()
		cards, err := e.callResolve(ctx, s, u)
		if err == source.ErrNotHandled {
			continue
		}
		if err != nil {
			errs = append(errs, schema.SourceError{Source: s.Name(), Message: safeMsg(err)})
			continue
		}
		cards = sanitize(s.Name(), cards)
		timings = append(timings, schema.Timing{Source: s.Name(), Millis: time.Since(start).Milliseconds(), Cards: len(cards)})
		if len(cards) > 0 {
			out <- Event{Type: "cards", Source: s.Name(), Cards: cards}
			out <- Event{Type: "done", Timings: timings, Errors: errs}
			return
		}
	}
	out <- Event{Type: "done", Timings: timings, Errors: errs}
}

func (e *Engine) searchStream(ctx context.Context, q source.Query, forced string, out chan<- Event) {
	var sources []source.Source
	if forced != "" {
		if s, ok := e.Registry.ByName(forced); ok {
			sources = []source.Source{s}
		}
	} else {
		sources = e.Registry.Searchers()
		// Intent sources answer even when their Caps().Search is false.
		if q.Intent != "" {
			seen := map[string]bool{}
			for _, s := range sources {
				seen[s.Name()] = true
			}
			for _, s := range e.Registry.ForIntent(q.Intent) {
				if !seen[s.Name()] {
					sources = append(sources, s)
				}
			}
		}
	}
	if len(sources) == 0 {
		out <- Event{Type: "done", Errors: []schema.SourceError{{Source: "engine", Message: "no sources available"}}}
		return
	}

	type answer struct {
		name   string
		cards  []schema.Card
		err    error
		millis int64
	}
	answers := make(chan answer, len(sources))
	for _, s := range sources {
		go func(s source.Source) {
			start := time.Now()
			cards, err := e.callSearch(ctx, s, q)
			answers <- answer{s.Name(), cards, err, time.Since(start).Milliseconds()}
		}(s)
	}

	var errs []schema.SourceError
	var timings []schema.Timing
	seenURL := map[string]bool{}
	for range sources {
		var a answer
		select {
		case a = <-answers:
		case <-ctx.Done():
			errs = append(errs, schema.SourceError{Source: "engine", Message: "query deadline exceeded"})
			out <- Event{Type: "done", Timings: timings, Errors: errs}
			return
		}
		if a.err == source.ErrNotHandled {
			continue
		}
		if a.err != nil {
			errs = append(errs, schema.SourceError{Source: a.name, Message: safeMsg(a.err)})
			out <- Event{Type: "error", Source: a.name, Message: safeMsg(a.err)}
			continue
		}
		cards := sanitize(a.name, a.cards)
		cards = dedupe(cards, seenURL)
		score(cards, q)
		timings = append(timings, schema.Timing{Source: a.name, Millis: a.millis, Cards: len(cards)})
		if len(cards) > 0 {
			out <- Event{Type: "cards", Source: a.name, Cards: cards}
		}
	}
	out <- Event{Type: "done", Timings: timings, Errors: errs}
}

// Run buffers Stream into one Result for the CLI and the non-streaming API.
func (e *Engine) Run(ctx context.Context, raw string) schema.Result {
	res := schema.Result{Query: raw}
	for ev := range e.Stream(ctx, raw) {
		switch ev.Type {
		case "cards":
			res.Cards = append(res.Cards, ev.Cards...)
		case "done":
			res.Timings, res.Errors = ev.Timings, ev.Errors
		}
	}
	schema.Sort(res.Cards)
	return res
}

// callSearch runs one source under its own deadline and recovers panics so a
// bad provider can never kill the query.
func (e *Engine) callSearch(ctx context.Context, s source.Source, q source.Query) (cards []schema.Card, err error) {
	ctx, cancel := context.WithTimeout(ctx, e.SourceDeadline)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return s.Search(ctx, q)
}

func (e *Engine) callResolve(ctx context.Context, s source.Source, u *url.URL) (cards []schema.Card, err error) {
	ctx, cancel := context.WithTimeout(ctx, e.SourceDeadline)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return s.Resolve(ctx, u)
}

func sanitize(name string, cards []schema.Card) []schema.Card {
	out := cards[:0]
	for i := range cards {
		c := cards[i]
		if c.Source == "" {
			c.Source = name
		}
		if c.Title == "" {
			c.Title = c.URL
		}
		if err := c.Validate(); err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}

func dedupe(cards []schema.Card, seen map[string]bool) []schema.Card {
	out := cards[:0]
	for _, c := range cards {
		key := canonicalURL(c.URL)
		if key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, c)
	}
	return out
}

func canonicalURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	u.Host = strings.ToLower(strings.TrimPrefix(u.Host, "www."))
	u.Scheme = "https"
	return strings.TrimSuffix(u.String(), "/")
}

// score assigns a source-relative rank normalized to 0..1 with a boost for
// cards that answer the detected intent directly.
func score(cards []schema.Card, q source.Query) {
	for i := range cards {
		if cards[i].Score == 0 {
			cards[i].Score = 1 - float64(i)*0.05
			if cards[i].Score < 0.1 {
				cards[i].Score = 0.1
			}
		}
		if q.Intent != "" {
			switch {
			case q.Intent == "weather" && cards[i].Kind == schema.KindWeather,
				q.Intent == "define" && cards[i].Kind == schema.KindDefinition,
				q.Intent == "stock" && cards[i].Kind == schema.KindChart:
				cards[i].Score += 1
			}
		}
		if cards[i].Kind == schema.KindEntity {
			cards[i].Score += 0.5
		}
	}
}

func safeMsg(err error) string {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}
