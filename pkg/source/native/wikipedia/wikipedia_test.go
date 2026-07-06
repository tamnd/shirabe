package wikipedia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

const openSearchBody = `["turing",["Alan Turing","Turing test"],["",""],["https://en.wikipedia.org/wiki/Alan_Turing","https://en.wikipedia.org/wiki/Turing_test"]]`

const summaryBody = `{
	"title":"Alan Turing",
	"description":"English mathematician",
	"extract":"Alan Turing was an English mathematician and computer scientist.",
	"thumbnail":{"source":"https://img.example/turing.jpg"},
	"content_urls":{"desktop":{"page":"https://en.wikipedia.org/wiki/Alan_Turing"}}
}`

func server(t *testing.T) *Source {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/w/api.php") {
			_, _ = w.Write([]byte(openSearchBody))
			return
		}
		_, _ = w.Write([]byte(summaryBody))
	}))
	t.Cleanup(srv.Close)
	s := New()
	s.Base = srv.URL
	return s
}

func TestSearchEntityFirst(t *testing.T) {
	s := server(t)
	cards, err := s.Search(context.Background(), source.Query{Raw: "turing", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("want entity + web, got %d", len(cards))
	}
	if cards[0].Kind != schema.KindEntity {
		t.Fatalf("first card should be entity, got %s", cards[0].Kind)
	}
	ent := cards[0].Body.(*schema.EntityBody)
	if !strings.Contains(ent.Description, "mathematician") || ent.Image == "" {
		t.Fatalf("bad entity body: %+v", ent)
	}
	if cards[1].Kind != schema.KindWeb {
		t.Fatalf("second card should be web, got %s", cards[1].Kind)
	}
}

func TestResolveWikiURL(t *testing.T) {
	s := server(t)
	u, _ := url.Parse("https://en.wikipedia.org/wiki/Alan_Turing")
	cards, err := s.Resolve(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].Kind != schema.KindEntity {
		t.Fatalf("bad resolve: %+v", cards)
	}
	u2, _ := url.Parse("https://en.wikipedia.org/about")
	if _, err := s.Resolve(context.Background(), u2); err != source.ErrNotHandled {
		t.Fatalf("want ErrNotHandled, got %v", err)
	}
}
