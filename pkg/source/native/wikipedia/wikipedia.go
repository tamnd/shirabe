// Package wikipedia turns the top opensearch hit into an entity card for the
// knowledge panel and the rest into web cards. Keyless REST API.
package wikipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
	"github.com/tamnd/shirabe/pkg/source/native/httpx"
)

type Source struct {
	Client *httpx.Client
	Base   string
}

func New() *Source {
	return &Source{Client: httpx.New(), Base: "https://en.wikipedia.org"}
}

func (s *Source) Name() string  { return "wikipedia" }
func (s *Source) Priority() int { return 20 }

func (s *Source) Caps() source.Caps {
	return source.Caps{Search: true, Resolve: true, Hosts: []string{"wikipedia.org"}}
}

type summary struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Extract     string `json:"extract"`
	Thumbnail   struct {
		Source string `json:"source"`
	} `json:"thumbnail"`
	Content struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

func (s *Source) summaryCard(ctx context.Context, title string) (schema.Card, error) {
	u := fmt.Sprintf("%s/api/rest_v1/page/summary/%s", s.Base, url.PathEscape(title))
	raw, err := s.Client.Get(ctx, u)
	if err != nil {
		return schema.Card{}, err
	}
	var sum summary
	if err := json.Unmarshal(raw, &sum); err != nil {
		return schema.Card{}, err
	}
	if sum.Extract == "" {
		return schema.Card{}, source.ErrNotHandled
	}
	body := &schema.EntityBody{
		Description: sum.Extract,
		Image:       sum.Thumbnail.Source,
		Attribution: "From Wikipedia, the free encyclopedia",
	}
	if sum.Description != "" {
		body.Facts = append(body.Facts, schema.Fact{Label: "Known for", Value: sum.Description})
	}
	page := sum.Content.Desktop.Page
	if page == "" {
		page = fmt.Sprintf("%s/wiki/%s", s.Base, url.PathEscape(strings.ReplaceAll(sum.Title, " ", "_")))
	}
	return schema.Card{
		Kind: schema.KindEntity, Source: s.Name(), Title: sum.Title,
		URL: page, Snippet: sum.Description, Thumbnail: sum.Thumbnail.Source,
		FetchedAt: time.Now(), Body: body,
	}, nil
}

func (s *Source) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	if q.Intent != "" && q.Intent != "define" {
		return nil, source.ErrNotHandled
	}
	term := q.Raw
	openURL := fmt.Sprintf("%s/w/api.php?action=opensearch&format=json&limit=%d&search=%s",
		s.Base, max(q.Limit, 3), url.QueryEscape(term))
	raw, err := s.Client.Get(ctx, openURL)
	if err != nil {
		return nil, err
	}
	// opensearch replies [term, [titles], [descriptions], [urls]]
	var parts []json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil || len(parts) < 4 {
		return nil, fmt.Errorf("opensearch: unexpected shape")
	}
	var titles, urls []string
	if err := json.Unmarshal(parts[1], &titles); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(parts[3], &urls); err != nil {
		return nil, err
	}
	if len(titles) == 0 {
		return nil, source.ErrNotHandled
	}

	var cards []schema.Card
	if ent, err := s.summaryCard(ctx, titles[0]); err == nil {
		cards = append(cards, ent)
	}
	for i := 1; i < len(titles) && i < len(urls); i++ {
		cards = append(cards, schema.Card{
			Kind: schema.KindWeb, Source: s.Name(), Title: titles[i] + " - Wikipedia",
			URL: urls[i], FetchedAt: time.Now(),
			Body: &schema.WebBody{Site: "en.wikipedia.org", DisplayURL: displayURL(urls[i])},
		})
	}
	return cards, nil
}

func (s *Source) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	title, ok := strings.CutPrefix(u.Path, "/wiki/")
	if !ok || title == "" {
		return nil, source.ErrNotHandled
	}
	card, err := s.summaryCard(ctx, title)
	if err != nil {
		return nil, err
	}
	return []schema.Card{card}, nil
}

func displayURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Host + " > " + strings.Trim(strings.ReplaceAll(u.Path, "/", " > "), " >")
}
