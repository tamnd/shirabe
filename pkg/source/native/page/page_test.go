package page

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/tamnd/shirabe/pkg/schema"
)

func resolveDoc(t *testing.T, doc string) schema.Card {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(doc))
	}))
	t.Cleanup(srv.Close)
	s := New()
	u, _ := url.Parse(srv.URL + "/thing")
	cards, err := s.Resolve(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("want 1 card, got %d", len(cards))
	}
	return cards[0]
}

func TestArticleFromOpenGraph(t *testing.T) {
	c := resolveDoc(t, `<html><head>
		<title>fallback</title>
		<meta property="og:title" content="A Post">
		<meta property="og:type" content="article">
		<meta property="og:description" content="About things.">
		<meta property="article:published_time" content="2026-01-01">
	</head><body></body></html>`)
	if c.Kind != schema.KindArticle || c.Title != "A Post" {
		t.Fatalf("bad card: %+v", c)
	}
	if c.Body.(*schema.ArticleBody).Published != "2026-01-01" {
		t.Fatalf("bad body: %+v", c.Body)
	}
}

func TestProductFromJSONLD(t *testing.T) {
	c := resolveDoc(t, `<html><head>
		<title>Widget | Shop</title>
		<meta property="product:price:amount" content="9.99">
		<meta property="product:price:currency" content="USD">
		<script type="application/ld+json">{"@type":"Product","name":"Widget"}</script>
	</head><body></body></html>`)
	if c.Kind != schema.KindProduct {
		t.Fatalf("want product, got %s", c.Kind)
	}
	p := c.Body.(*schema.ProductBody)
	if p.Price != "9.99" || p.Currency != "USD" {
		t.Fatalf("bad body: %+v", p)
	}
}

func TestPlainPageFallsToWeb(t *testing.T) {
	c := resolveDoc(t, `<html><head><title>Just a page</title></head><body>hi</body></html>`)
	if c.Kind != schema.KindWeb || c.Title != "Just a page" {
		t.Fatalf("bad card: %+v", c)
	}
}
