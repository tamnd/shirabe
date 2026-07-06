// Package hackernews searches HN via the Algolia API, keyless.
package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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
	return &Source{Client: httpx.New(), Base: "https://hn.algolia.com"}
}

func (s *Source) Name() string  { return "hackernews" }
func (s *Source) Priority() int { return 40 }

func (s *Source) Caps() source.Caps {
	return source.Caps{Search: true, Resolve: true, Hosts: []string{"news.ycombinator.com"}}
}

type hit struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	ObjectID    string `json:"objectID"`
	StoryText   string `json:"story_text"`
}

func (h hit) card(src string) schema.Card {
	link := h.URL
	if link == "" {
		link = "https://news.ycombinator.com/item?id=" + h.ObjectID
	}
	return schema.Card{
		Kind: schema.KindQA, Source: src, Title: h.Title, URL: link,
		FetchedAt: time.Now(),
		Body: &schema.QABody{
			Question: h.Title, Answer: h.StoryText,
			Votes: h.Points, Comments: h.NumComments,
		},
	}
}

func (s *Source) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	if q.Intent != "" {
		return nil, source.ErrNotHandled
	}
	u := fmt.Sprintf("%s/api/v1/search?tags=story&hitsPerPage=%d&query=%s",
		s.Base, max(q.Limit/2, 3), url.QueryEscape(q.Raw))
	raw, err := s.Client.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	var res struct {
		Hits []hit `json:"hits"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	if len(res.Hits) == 0 {
		return nil, source.ErrNotHandled
	}
	cards := make([]schema.Card, 0, len(res.Hits))
	for _, h := range res.Hits {
		if h.Title == "" {
			continue
		}
		cards = append(cards, h.card(s.Name()))
	}
	return cards, nil
}

func (s *Source) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	id := u.Query().Get("id")
	if u.Path != "/item" || id == "" {
		return nil, source.ErrNotHandled
	}
	raw, err := s.Client.Get(ctx, fmt.Sprintf("%s/api/v1/items/%s", s.Base, url.PathEscape(id)))
	if err != nil {
		return nil, err
	}
	var item struct {
		Title    string `json:"title"`
		URL      string `json:"url"`
		Points   int    `json:"points"`
		Children []any  `json:"children"`
		Text     string `json:"text"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil, err
	}
	if item.Title == "" {
		return nil, source.ErrNotHandled
	}
	h := hit{Title: item.Title, URL: item.URL, Points: item.Points, NumComments: len(item.Children), ObjectID: id, StoryText: item.Text}
	return []schema.Card{h.card(s.Name())}, nil
}
