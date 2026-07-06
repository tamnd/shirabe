// Package oembed resolves video and image URLs through public oEmbed
// endpoints. It is the fallback when the dedicated site CLI is not installed,
// so it registers at a lower priority than any exec adapter.
package oembed

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
	"github.com/tamnd/shirabe/pkg/source/native/httpx"
)

// endpoint templates get the target URL query-escaped into %s.
var endpoints = map[string]string{
	"youtube.com": "https://www.youtube.com/oembed?format=json&url=%s",
	"youtu.be":    "https://www.youtube.com/oembed?format=json&url=%s",
	"vimeo.com":   "https://vimeo.com/api/oembed.json?url=%s",
	"flickr.com":  "https://www.flickr.com/services/oembed?format=json&url=%s",
}

type Source struct {
	Client *httpx.Client
	// Endpoints overrides the builtin table in tests.
	Endpoints map[string]string
}

func New() *Source { return &Source{Client: httpx.New(), Endpoints: endpoints} }

func (s *Source) Name() string  { return "oembed" }
func (s *Source) Priority() int { return 150 }

func (s *Source) Caps() source.Caps {
	hosts := make([]string, 0, len(s.Endpoints))
	for h := range s.Endpoints {
		hosts = append(hosts, h)
	}
	return source.Caps{Resolve: true, Hosts: hosts}
}

func (s *Source) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	return nil, source.ErrNotHandled
}

func (s *Source) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	tmpl := ""
	for h, t := range s.Endpoints {
		if host == h || strings.HasSuffix(host, "."+h) {
			tmpl = t
			break
		}
	}
	if tmpl == "" {
		return nil, source.ErrNotHandled
	}
	raw, err := s.Client.Get(ctx, strings.Replace(tmpl, "%s", url.QueryEscape(u.String()), 1))
	if err != nil {
		return nil, err
	}
	var o struct {
		Type         string `json:"type"`
		Title        string `json:"title"`
		AuthorName   string `json:"author_name"`
		ThumbnailURL string `json:"thumbnail_url"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		HTML         string `json:"html"`
	}
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, err
	}
	if o.Title == "" {
		return nil, source.ErrNotHandled
	}
	card := schema.Card{
		Source: s.Name(), Title: o.Title, URL: u.String(),
		Thumbnail: o.ThumbnailURL, FetchedAt: time.Now(),
	}
	switch o.Type {
	case "photo":
		card.Kind = schema.KindImage
		card.Body = &schema.ImageBody{Width: o.Width, Height: o.Height, SourcePage: u.String()}
	default:
		card.Kind = schema.KindVideo
		card.Body = &schema.VideoBody{Channel: o.AuthorName, EmbedURL: embedURL(o.HTML)}
	}
	return []schema.Card{card}, nil
}

// embedURL pulls the iframe src out of the oEmbed html snippet.
func embedURL(html string) string {
	_, rest, ok := strings.Cut(html, `src="`)
	if !ok {
		return ""
	}
	src, _, ok := strings.Cut(rest, `"`)
	if !ok || !strings.HasPrefix(src, "https://") {
		return ""
	}
	return src
}
