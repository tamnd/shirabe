// Package stooq answers "<ticker> stock" with a quote fact card and a
// closing-price chart from Stooq's free CSV endpoints, keyless.
package stooq

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/url"
	"strconv"
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
	return &Source{Client: httpx.New(), Base: "https://stooq.com"}
}

func (s *Source) Name() string  { return "stooq" }
func (s *Source) Priority() int { return 10 }

func (s *Source) Caps() source.Caps {
	return source.Caps{Intents: []string{"stock"}}
}

func (s *Source) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	return nil, source.ErrNotHandled
}

func (s *Source) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	if q.Intent != "stock" || q.Arg == "" {
		return nil, source.ErrNotHandled
	}
	// US tickers live under the .us suffix on stooq.
	sym := strings.ToLower(q.Arg)
	if !strings.Contains(sym, ".") && !strings.HasPrefix(sym, "^") {
		sym += ".us"
	}
	raw, err := s.Client.Get(ctx, fmt.Sprintf("%s/q/d/l/?s=%s&i=d", s.Base, url.QueryEscape(sym)))
	if err != nil {
		return nil, err
	}
	rows, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil || len(rows) < 2 || len(rows[0]) < 5 {
		return nil, source.ErrNotHandled
	}
	// Header is Date,Open,High,Low,Close,Volume. Keep the last ~90 sessions.
	rows = rows[1:]
	if len(rows) > 90 {
		rows = rows[len(rows)-90:]
	}
	labels := make([]string, 0, len(rows))
	closes := make([]float64, 0, len(rows))
	for _, row := range rows {
		c, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			continue
		}
		labels = append(labels, row[0])
		closes = append(closes, c)
	}
	if len(closes) < 2 {
		return nil, source.ErrNotHandled
	}
	last := closes[len(closes)-1]
	prev := closes[len(closes)-2]
	change := (last - prev) / prev * 100
	ticker := strings.ToUpper(q.Arg)
	return []schema.Card{{
		Kind: schema.KindChart, Source: s.Name(),
		Title:     fmt.Sprintf("%s %.2f (%+.2f%%)", ticker, last, change),
		URL:       "https://stooq.com/q/?s=" + url.QueryEscape(sym),
		Snippet:   fmt.Sprintf("Close %s, previous %.2f, last %d sessions", labels[len(labels)-1], prev, len(closes)),
		FetchedAt: time.Now(),
		Body: &schema.ChartBody{
			ChartKind: "line", XLabels: labels, Unit: "",
			Series: []schema.Series{{Name: ticker, Points: closes}},
		},
	}}, nil
}
