// Package weather answers "weather in <place>" via Open-Meteo, keyless.
// It emits one weather card and one chart card with the 7-day hi/lo.
package weather

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"encoding/json"

	"github.com/tamnd/shirabe/pkg/schema"
	"github.com/tamnd/shirabe/pkg/source"
	"github.com/tamnd/shirabe/pkg/source/native/httpx"
)

type Source struct {
	Client       *httpx.Client
	GeoBase      string // override in tests
	ForecastBase string
}

func New() *Source {
	return &Source{
		Client:       httpx.New(),
		GeoBase:      "https://geocoding-api.open-meteo.com",
		ForecastBase: "https://api.open-meteo.com",
	}
}

func (s *Source) Name() string  { return "weather" }
func (s *Source) Priority() int { return 10 }

func (s *Source) Caps() source.Caps {
	return source.Caps{Intents: []string{"weather"}}
}

func (s *Source) Resolve(ctx context.Context, u *url.URL) ([]schema.Card, error) {
	return nil, source.ErrNotHandled
}

// wmo maps WMO weather codes to a condition and an emoji-style icon key the
// UI turns into an SVG glyph.
func wmo(code int) (string, string) {
	switch {
	case code == 0:
		return "Clear sky", "sun"
	case code <= 2:
		return "Partly cloudy", "sun-cloud"
	case code == 3:
		return "Overcast", "cloud"
	case code == 45 || code == 48:
		return "Fog", "fog"
	case code >= 51 && code <= 57:
		return "Drizzle", "drizzle"
	case code >= 61 && code <= 67:
		return "Rain", "rain"
	case code >= 71 && code <= 77:
		return "Snow", "snow"
	case code >= 80 && code <= 82:
		return "Rain showers", "rain"
	case code == 85 || code == 86:
		return "Snow showers", "snow"
	case code >= 95:
		return "Thunderstorm", "storm"
	}
	return "Unknown", "cloud"
}

func (s *Source) Search(ctx context.Context, q source.Query) ([]schema.Card, error) {
	if q.Intent != "weather" || q.Arg == "" {
		return nil, source.ErrNotHandled
	}
	geoURL := fmt.Sprintf("%s/v1/search?count=1&language=en&format=json&name=%s", s.GeoBase, url.QueryEscape(q.Arg))
	raw, err := s.Client.Get(ctx, geoURL)
	if err != nil {
		return nil, err
	}
	var geo struct {
		Results []struct {
			Name    string  `json:"name"`
			Country string  `json:"country"`
			Lat     float64 `json:"latitude"`
			Lon     float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &geo); err != nil {
		return nil, err
	}
	if len(geo.Results) == 0 {
		return nil, source.ErrNotHandled
	}
	loc := geo.Results[0]

	fcURL := fmt.Sprintf("%s/v1/forecast?latitude=%.4f&longitude=%.4f"+
		"&current=temperature_2m,relative_humidity_2m,weather_code,wind_speed_10m"+
		"&daily=temperature_2m_max,temperature_2m_min,weather_code&timezone=auto",
		s.ForecastBase, loc.Lat, loc.Lon)
	raw, err = s.Client.Get(ctx, fcURL)
	if err != nil {
		return nil, err
	}
	var fc struct {
		Current struct {
			Temp     float64 `json:"temperature_2m"`
			Humidity int     `json:"relative_humidity_2m"`
			Code     int     `json:"weather_code"`
			Wind     float64 `json:"wind_speed_10m"`
		} `json:"current"`
		Daily struct {
			Time []string  `json:"time"`
			Max  []float64 `json:"temperature_2m_max"`
			Min  []float64 `json:"temperature_2m_min"`
			Code []int     `json:"weather_code"`
		} `json:"daily"`
	}
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, err
	}

	place := loc.Name
	if loc.Country != "" {
		place += ", " + loc.Country
	}
	condition, icon := wmo(fc.Current.Code)
	body := &schema.WeatherBody{
		Place: place, TempC: fc.Current.Temp, Condition: condition, Icon: icon,
		WindKmh: fc.Current.Wind, Humidity: fc.Current.Humidity,
	}
	days := min(len(fc.Daily.Time), 7)
	labels := make([]string, 0, days)
	hi := make([]float64, 0, days)
	lo := make([]float64, 0, days)
	for i := range days {
		cond, dayIcon := wmo(fc.Daily.Code[i])
		body.Forecast = append(body.Forecast, schema.ForecastDay{
			Date: fc.Daily.Time[i], HiC: fc.Daily.Max[i], LoC: fc.Daily.Min[i],
			Condition: cond, Icon: dayIcon,
		})
		labels = append(labels, shortDay(fc.Daily.Time[i]))
		hi = append(hi, fc.Daily.Max[i])
		lo = append(lo, fc.Daily.Min[i])
	}

	now := time.Now()
	cards := []schema.Card{{
		Kind: schema.KindWeather, Source: s.Name(),
		Title: "Weather in " + place, FetchedAt: now, Body: body,
	}}
	if days > 1 {
		cards = append(cards, schema.Card{
			Kind: schema.KindChart, Source: s.Name(),
			Title: "7-day temperature, " + place, FetchedAt: now, Score: 0.6,
			Body: &schema.ChartBody{
				ChartKind: "line", XLabels: labels, Unit: "°C",
				Series: []schema.Series{{Name: "High", Points: hi}, {Name: "Low", Points: lo}},
			},
		})
	}
	return cards, nil
}

func shortDay(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.Format("Mon")
}
