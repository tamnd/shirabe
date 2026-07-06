package engine

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

type fake struct {
	name    string
	caps    source.Caps
	delay   time.Duration
	cards   []schema.Card
	err     error
	panics  bool
	avail   bool
	touched chan string
}

func (f *fake) Name() string      { return f.name }
func (f *fake) Caps() source.Caps { return f.caps }
func (f *fake) Available() bool   { return f.avail }

func (f *fake) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	if f.touched != nil {
		f.touched <- f.name
	}
	if f.panics {
		panic("boom")
	}
	select {
	case <-time.After(f.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return f.cards, f.err
}

func (f *fake) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	return f.cards, f.err
}

func card(src, title string) schema.Card {
	return schema.Card{Kind: schema.KindWeb, Source: src, Title: title, URL: "https://" + src + ".example/" + title}
}

func newEngine(t *testing.T, sources ...source.Source) *Engine {
	t.Helper()
	reg := source.NewRegistry()
	for _, s := range sources {
		if err := reg.Register(s); err != nil {
			t.Fatal(err)
		}
	}
	e := New(reg)
	e.Deadline = 2 * time.Second
	e.SourceDeadline = 500 * time.Millisecond
	return e
}

func TestParse(t *testing.T) {
	e := newEngine(t)
	cases := []struct {
		in           string
		wantURL      bool
		intent, arg  string
		forcedSource string
	}{
		{"https://youtube.com/watch?v=x", true, "", "", ""},
		{"youtube.com/watch?v=x", true, "", "", ""},
		{"tokyo weather", false, "weather", "tokyo", ""},
		{"weather in Hanoi", false, "weather", "Hanoi", ""},
		{"define serendipity", false, "define", "serendipity", ""},
		{"AAPL stock", false, "stock", "AAPL", ""},
		{"!yt lofi beats", false, "", "", "youtube"},
		{"plain query", false, "", "", ""},
		{"1.5 as a fraction", false, "", "", ""},
	}
	for _, c := range cases {
		u, q, forced := e.Parse(c.in)
		if (u != nil) != c.wantURL {
			t.Errorf("%q: url=%v want %v", c.in, u, c.wantURL)
		}
		if q.Intent != c.intent || q.Arg != c.arg {
			t.Errorf("%q: intent %q/%q want %q/%q", c.in, q.Intent, q.Arg, c.intent, c.arg)
		}
		if forced != c.forcedSource {
			t.Errorf("%q: forced %q want %q", c.in, forced, c.forcedSource)
		}
	}
}

func TestStreamFastBeforeSlow(t *testing.T) {
	fast := &fake{name: "fast", caps: source.Caps{Search: true}, avail: true, cards: []schema.Card{card("fast", "a")}}
	slow := &fake{name: "slow", caps: source.Caps{Search: true}, avail: true, delay: 200 * time.Millisecond, cards: []schema.Card{card("slow", "b")}}
	e := newEngine(t, fast, slow)
	var order []string
	for ev := range e.Stream(context.Background(), "q") {
		if ev.Type == "cards" {
			order = append(order, ev.Source)
		}
	}
	if len(order) != 2 || order[0] != "fast" || order[1] != "slow" {
		t.Fatalf("bad order: %v", order)
	}
}

func TestSlowSourceCutOthersIntact(t *testing.T) {
	ok := &fake{name: "ok", caps: source.Caps{Search: true}, avail: true, cards: []schema.Card{card("ok", "a")}}
	stuck := &fake{name: "stuck", caps: source.Caps{Search: true}, avail: true, delay: 5 * time.Second}
	e := newEngine(t, ok, stuck)
	res := e.Run(context.Background(), "q")
	if len(res.Cards) != 1 || res.Cards[0].Source != "ok" {
		t.Fatalf("want ok card, got %+v", res.Cards)
	}
	if len(res.Errors) != 1 || res.Errors[0].Source != "stuck" {
		t.Fatalf("want stuck error, got %+v", res.Errors)
	}
}

func TestPanicRecovered(t *testing.T) {
	bad := &fake{name: "bad", caps: source.Caps{Search: true}, avail: true, panics: true}
	ok := &fake{name: "ok", caps: source.Caps{Search: true}, avail: true, cards: []schema.Card{card("ok", "a")}}
	e := newEngine(t, bad, ok)
	res := e.Run(context.Background(), "q")
	if len(res.Cards) != 1 {
		t.Fatalf("want 1 card, got %d", len(res.Cards))
	}
	if len(res.Errors) != 1 || res.Errors[0].Source != "bad" {
		t.Fatalf("want bad error, got %+v", res.Errors)
	}
}

func TestDedupeAcrossSources(t *testing.T) {
	c1 := schema.Card{Kind: schema.KindWeb, Source: "a", Title: "same", URL: "https://www.example.com/x/"}
	c2 := schema.Card{Kind: schema.KindWeb, Source: "b", Title: "same", URL: "http://example.com/x"}
	a := &fake{name: "a", caps: source.Caps{Search: true}, avail: true, cards: []schema.Card{c1}}
	b := &fake{name: "b", caps: source.Caps{Search: true}, avail: true, delay: 50 * time.Millisecond, cards: []schema.Card{c2}}
	e := newEngine(t, a, b)
	res := e.Run(context.Background(), "q")
	if len(res.Cards) != 1 {
		t.Fatalf("want dedupe to 1 card, got %d", len(res.Cards))
	}
}

func TestBangForcesSingleSource(t *testing.T) {
	touched := make(chan string, 4)
	yt := &fake{name: "youtube", caps: source.Caps{Search: true}, avail: true, cards: []schema.Card{card("youtube", "v")}, touched: touched}
	other := &fake{name: "other", caps: source.Caps{Search: true}, avail: true, cards: []schema.Card{card("other", "o")}, touched: touched}
	e := newEngine(t, yt, other)
	res := e.Run(context.Background(), "!yt lofi")
	close(touched)
	for name := range touched {
		if name == "other" {
			t.Fatal("bang query touched a non-forced source")
		}
	}
	if len(res.Cards) != 1 || res.Cards[0].Source != "youtube" {
		t.Fatalf("want youtube only, got %+v", res.Cards)
	}
}

func TestUnavailableSkipped(t *testing.T) {
	gone := &fake{name: "gone", caps: source.Caps{Search: true}, avail: false, cards: []schema.Card{card("gone", "x")}}
	ok := &fake{name: "ok", caps: source.Caps{Search: true}, avail: true, cards: []schema.Card{card("ok", "a")}}
	e := newEngine(t, gone, ok)
	res := e.Run(context.Background(), "q")
	if len(res.Cards) != 1 || res.Cards[0].Source != "ok" {
		t.Fatalf("unavailable source not skipped: %+v", res.Cards)
	}
}

func TestForHostRouting(t *testing.T) {
	yt := &fake{name: "youtube", caps: source.Caps{Resolve: true, Hosts: []string{"youtube.com", "youtu.be"}}, avail: true, cards: []schema.Card{{Kind: schema.KindVideo, Source: "youtube", Title: "v"}}}
	page := &fake{name: "page", caps: source.Caps{Resolve: true}, avail: true, cards: []schema.Card{card("page", "p")}}
	e := newEngine(t, yt, page)

	res := e.Run(context.Background(), "https://m.youtube.com/watch?v=x")
	if len(res.Cards) != 1 || res.Cards[0].Source != "youtube" {
		t.Fatalf("want youtube resolver, got %+v", res.Cards)
	}
	res = e.Run(context.Background(), "https://random.example/post")
	if len(res.Cards) != 1 || res.Cards[0].Source != "page" {
		t.Fatalf("want page fallback, got %+v", res.Cards)
	}
}
