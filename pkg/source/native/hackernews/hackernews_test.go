package hackernews

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

const searchBody = `{"hits":[
	{"title":"Show HN: thing","url":"https://thing.example","points":120,"num_comments":45,"objectID":"1"},
	{"title":"Ask HN: how","url":"","points":10,"num_comments":3,"objectID":"2","story_text":"body"}
]}`

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(searchBody))
	}))
	t.Cleanup(srv.Close)
	s := New()
	s.Base = srv.URL
	cards, err := s.Search(context.Background(), source.Query{Raw: "thing", Limit: 8})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || cards[0].Kind != schema.KindQA {
		t.Fatalf("bad cards: %+v", cards)
	}
	if cards[1].URL != "https://news.ycombinator.com/item?id=2" {
		t.Fatalf("self-post URL not synthesized: %s", cards[1].URL)
	}
	if cards[0].Body.(*schema.QABody).Votes != 120 {
		t.Fatalf("bad body: %+v", cards[0].Body)
	}
}

func TestResolveNonItemPasses(t *testing.T) {
	s := New()
	u, _ := url.Parse("https://news.ycombinator.com/newest")
	if _, err := s.Resolve(context.Background(), u); err != source.ErrNotHandled {
		t.Fatalf("want ErrNotHandled, got %v", err)
	}
}
