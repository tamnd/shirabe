package stooq

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

const csvBody = `Date,Open,High,Low,Close,Volume
2026-07-01,100,101,99,100.5,1000
2026-07-02,100.5,102,100,101.0,1200
2026-07-03,101,103,100,102.0,900
`

func TestStockChart(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RawQuery
		_, _ = w.Write([]byte(csvBody))
	}))
	t.Cleanup(srv.Close)
	s := New()
	s.Base = srv.URL
	cards, err := s.Search(context.Background(), source.Query{Intent: "stock", Arg: "AAPL"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "aapl.us") {
		t.Fatalf("US suffix not applied: %s", gotPath)
	}
	if len(cards) != 1 || cards[0].Kind != schema.KindChart {
		t.Fatalf("bad cards: %+v", cards)
	}
	ch := cards[0].Body.(*schema.ChartBody)
	if len(ch.Series) != 1 || len(ch.Series[0].Points) != 3 || ch.Series[0].Points[2] != 102 {
		t.Fatalf("bad chart: %+v", ch)
	}
	if !strings.Contains(cards[0].Title, "+0.99%") {
		t.Fatalf("bad change calc: %s", cards[0].Title)
	}
}
