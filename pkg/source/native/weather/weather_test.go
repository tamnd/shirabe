package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
)

const geoBody = `{"results":[{"name":"Tokyo","country":"Japan","latitude":35.68,"longitude":139.69}]}`

const fcBody = `{
	"current":{"temperature_2m":21.5,"relative_humidity_2m":60,"weather_code":3,"wind_speed_10m":12.5},
	"daily":{
		"time":["2026-07-06","2026-07-07","2026-07-08"],
		"temperature_2m_max":[30,31,29],
		"temperature_2m_min":[22,23,21],
		"weather_code":[0,61,95]
	}
}`

func server(t *testing.T) *Source {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/search") {
			w.Write([]byte(geoBody))
			return
		}
		w.Write([]byte(fcBody))
	}))
	t.Cleanup(srv.Close)
	s := New()
	s.GeoBase, s.ForecastBase = srv.URL, srv.URL
	return s
}

func TestWeatherCards(t *testing.T) {
	s := server(t)
	cards, err := s.Search(context.Background(), source.Query{Intent: "weather", Arg: "tokyo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("want weather + chart, got %d", len(cards))
	}
	wx := cards[0].Body.(*schema.WeatherBody)
	if wx.Place != "Tokyo, Japan" || wx.TempC != 21.5 || wx.Condition != "Overcast" || len(wx.Forecast) != 3 {
		t.Fatalf("bad weather body: %+v", wx)
	}
	ch := cards[1].Body.(*schema.ChartBody)
	if len(ch.Series) != 2 || len(ch.Series[0].Points) != 3 || ch.XLabels[0] != "Mon" {
		t.Fatalf("bad chart body: %+v", ch)
	}
}

func TestWrongIntentPasses(t *testing.T) {
	s := server(t)
	if _, err := s.Search(context.Background(), source.Query{Raw: "tokyo"}); err != source.ErrNotHandled {
		t.Fatalf("want ErrNotHandled, got %v", err)
	}
}
