// Package page is the last-resort resolver: fetch the URL, read OpenGraph
// and Twitter meta plus JSON-LD, and emit the best-typed card we can.
package page

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
	"github.com/tamnd/shirabe/pkg/source/native/httpx"
)

type Source struct {
	Client *httpx.Client
}

func New() *Source { return &Source{Client: httpx.New()} }

func (s *Source) Name() string  { return "page" }
func (s *Source) Priority() int { return 200 }

// Caps has Resolve with no hosts: the registry treats that as the wildcard
// fallback, tried after every host-matched source.
func (s *Source) Caps() source.Caps { return source.Caps{Resolve: true} }

func (s *Source) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	return nil, source.ErrNotHandled
}

type meta struct {
	title, desc, image, siteName, ogType string
	author, published                    string
	price, currency                      string
	ldTypes                              []string
}

func (s *Source) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	raw, err := s.Client.Get(ctx, u.String())
	if err != nil {
		return nil, err
	}
	m := parse(string(raw))
	if m.title == "" {
		return nil, source.ErrNotHandled
	}
	card := schema.Card{
		Source: s.Name(), Title: m.title, URL: u.String(),
		Snippet: m.desc, Thumbnail: m.image, FetchedAt: time.Now(),
	}
	switch {
	case m.ogType == "video" || strings.HasPrefix(m.ogType, "video.") || hasType(m.ldTypes, "VideoObject"):
		card.Kind = schema.KindVideo
		card.Body = &schema.VideoBody{Channel: m.siteName, Published: m.published}
	case m.price != "" || hasType(m.ldTypes, "Product"):
		card.Kind = schema.KindProduct
		card.Body = &schema.ProductBody{Price: m.price, Currency: m.currency, Merchant: m.siteName}
	case m.ogType == "article" || hasType(m.ldTypes, "Article", "NewsArticle", "BlogPosting"):
		card.Kind = schema.KindArticle
		card.Body = &schema.ArticleBody{Author: m.author, Published: m.published, Excerpt: m.desc}
	default:
		card.Kind = schema.KindWeb
		site := m.siteName
		if site == "" {
			site = u.Hostname()
		}
		card.Body = &schema.WebBody{Site: site, DisplayURL: u.Hostname() + u.Path}
	}
	return []schema.Card{card}, nil
}

func hasType(types []string, wanted ...string) bool {
	for _, t := range types {
		for _, w := range wanted {
			if strings.EqualFold(t, w) {
				return true
			}
		}
	}
	return false
}

// parse walks the document head and collects meta and JSON-LD signals.
func parse(doc string) meta {
	var m meta
	og := map[string]string{}
	tz := html.NewTokenizer(strings.NewReader(doc))
	var inTitle, inLD bool
	for {
		switch tz.Next() {
		case html.ErrorToken:
			m.apply(og)
			return m
		case html.StartTagToken, html.SelfClosingTagToken:
			tok := tz.Token()
			switch tok.Data {
			case "title":
				inTitle = true
			case "script":
				inLD = attr(tok, "type") == "application/ld+json"
			case "meta":
				key := attr(tok, "property")
				if key == "" {
					key = attr(tok, "name")
				}
				if v := attr(tok, "content"); key != "" && v != "" {
					if _, dup := og[key]; !dup {
						og[key] = v
					}
				}
			case "body":
				m.apply(og)
				return m
			}
		case html.EndTagToken:
			tok := tz.Token()
			if tok.Data == "title" {
				inTitle = false
			}
			if tok.Data == "script" {
				inLD = false
			}
		case html.TextToken:
			text := strings.TrimSpace(string(tz.Text()))
			if inTitle && text != "" && m.title == "" {
				m.title = text
			}
			if inLD && text != "" {
				m.ldTypes = append(m.ldTypes, ldTypes(text)...)
			}
		}
	}
}

func (m *meta) apply(og map[string]string) {
	pick := func(keys ...string) string {
		for _, k := range keys {
			if og[k] != "" {
				return og[k]
			}
		}
		return ""
	}
	if t := pick("og:title", "twitter:title"); t != "" {
		m.title = t
	}
	m.desc = pick("og:description", "twitter:description", "description")
	m.image = pick("og:image", "twitter:image")
	m.siteName = pick("og:site_name")
	m.ogType = pick("og:type")
	m.author = pick("article:author", "author")
	m.published = pick("article:published_time")
	m.price = pick("product:price:amount", "og:price:amount")
	m.currency = pick("product:price:currency", "og:price:currency")
}

// ldTypes pulls every @type out of a JSON-LD block, tolerating both a single
// object and an array of them.
func ldTypes(text string) []string {
	var out []string
	var walk func(v any)
	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			switch t := node["@type"].(type) {
			case string:
				out = append(out, t)
			case []any:
				for _, x := range t {
					if s, ok := x.(string); ok {
						out = append(out, s)
					}
				}
			}
			if g, ok := node["@graph"].([]any); ok {
				for _, x := range g {
					walk(x)
				}
			}
		case []any:
			for _, x := range node {
				walk(x)
			}
		}
	}
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		return nil
	}
	walk(v)
	return out
}

func attr(tok html.Token, name string) string {
	for _, a := range tok.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}
